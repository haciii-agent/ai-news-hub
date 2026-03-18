package ai

import (
	"database/sql"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"ai-news-hub/internal/store"
)

// TrendAnalyzer performs local trend analysis on articles.
type TrendAnalyzer struct {
	db *sql.DB
}

// NewTrendAnalyzer creates a new TrendAnalyzer.
func NewTrendAnalyzer(db *sql.DB) *TrendAnalyzer {
	return &TrendAnalyzer{db: db}
}

// TrendTopic represents a trending topic.
type TrendTopic struct {
	Keyword      string        `json:"keyword"`
	Score        float64       `json:"score"`         // 综合热度 0-100
	ArticleCount int           `json:"article_count"`
	RecentCount  int           `json:"recent_count"`  // 近24h文章数
	Trend        string        `json:"trend"`         // "rising" / "stable" / "declining"
	RelatedTags  []string      `json:"related_tags"`
	TopArticles  []TopicArticle `json:"top_articles"`
}

// TopicArticle is a simplified article for trend topic display.
type TopicArticle struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Source      string  `json:"source"`
	PublishedAt *string `json:"published_at,omitempty"`
}

// TimelineDataPoint represents a single day's keyword count.
type TimelineDataPoint struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// TimelineResponse holds timeline data for a keyword.
type TimelineResponse struct {
	Keyword string              `json:"keyword"`
	Days    int                 `json:"days"`
	Data    []TimelineDataPoint `json:"data"`
	Peak    *TimelineDataPoint  `json:"peak"`
}

// StoryPitch represents a recommended writing topic.
type StoryPitch struct {
	Topic             string   `json:"topic"`
	Score             float64  `json:"score"`
	Reason            string   `json:"reason"`
	Angle             string   `json:"angle"`
	RelatedArticlesCount int  `json:"related_articles_count"`
	TopSources        []string `json:"top_sources"`
	Writability       float64  `json:"writability"`
}

// HotTopicsResponse is the response for hot topics API.
type HotTopicsResponse struct {
	Period      string       `json:"period"`
	Topics      []TrendTopic `json:"topics"`
	GeneratedAt string       `json:"generated_at"`
}

// StoryPitchesResponse is the response for story pitches API.
type StoryPitchesResponse struct {
	Pitches []StoryPitch `json:"pitches"`
}

// RelatedTopicsResponse is the response for related topics API.
type RelatedTopicsResponse struct {
	Keyword       string  `json:"keyword"`
	RelatedTopics []RelatedTopic `json:"related_topics"`
}

// RelatedTopic represents a topic related to a given keyword.
type RelatedTopic struct {
	Keyword  string  `json:"keyword"`
	CoOccur  int     `json:"co_occur"`
	Score    float64 `json:"score"`
}

// --- Keyword extraction ---

// articleText represents an article's text for keyword extraction.
type articleText struct {
	Title     string
	Summary   string
	Source    string
	CollectedAt string
	HasAISummary bool
	ImportanceScore float64
	ID int64
}

// chineseStopWords is a built-in list of common Chinese stop words.
var chineseStopWords = map[string]bool{
	"的": true, "了": true, "在": true, "是": true, "我": true, "有": true, "和": true,
	"就": true, "不": true, "人": true, "都": true, "一": true, "一个": true, "上": true,
	"也": true, "很": true, "到": true, "说": true, "要": true, "去": true, "你": true,
	"会": true, "着": true, "没有": true, "看": true, "好": true, "自己": true, "这": true,
	"他": true, "她": true, "它": true, "们": true, "那": true, "些": true, "什么": true,
	"吗": true, "吧": true, "啊": true, "呢": true, "把": true, "被": true, "让": true,
	"给": true, "从": true, "对": true, "与": true, "而": true, "但": true, "如果": true,
	"因为": true, "所以": true, "虽然": true, "可以": true, "这个": true, "那个": true,
	"为": true, "以": true, "及": true, "等": true, "其": true, "之": true, "中": true,
	"将": true, "能": true, "还": true, "或": true, "并": true, "于": true,
	"所": true, "最": true, "新": true, "更": true, "多": true, "大": true,
	"年": true, "月": true, "日": true, "个": true, "来": true, "用": true,
	"做": true, "出": true, "下": true, "里": true, "后": true, "前": true, "又": true,
	"只": true, "过": true, "想": true, "样": true, "两": true, "然": true,
	"此": true, "当": true, "无": true, "地": true, "得": true,
	"可": true, "进": true, "别": true, "这些": true, "那些": true, "这种": true,
}

// englishStopWords is a built-in list of common English stop words.
var englishStopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true, "have": true, "has": true, "had": true,
	"do": true, "does": true, "did": true, "will": true, "would": true, "could": true,
	"should": true, "may": true, "might": true, "must": true, "shall": true,
	"can": true, "need": true, "dare": true, "ought": true, "used": true,
	"to": true, "of": true, "in": true, "for": true, "on": true, "with": true,
	"at": true, "by": true, "from": true, "as": true, "into": true, "through": true,
	"during": true, "before": true, "after": true, "above": true, "below": true,
	"between": true, "out": true, "off": true, "over": true, "under": true,
	"again": true, "further": true, "then": true, "once": true, "here": true,
	"there": true, "when": true, "where": true, "why": true, "how": true,
	"all": true, "both": true, "each": true, "few": true, "more": true,
	"most": true, "other": true, "some": true, "such": true, "no": true,
	"nor": true, "not": true, "only": true, "own": true, "same": true,
	"so": true, "than": true, "too": true, "very": true, "just": true,
	"because": true, "but": true, "and": true, "or": true, "if": true,
	"while": true, "about": true, "against": true, "up": true, "down": true,
	"that": true, "this": true, "these": true, "those": true, "i": true,
	"me": true, "my": true, "myself": true, "we": true, "our": true,
	"ours": true, "ourselves": true, "you": true, "your": true, "yours": true,
	"he": true, "him": true, "his": true, "she": true, "her": true, "hers": true,
	"it": true, "its": true, "they": true, "them": true, "their": true,
	"what": true, "which": true, "who": true, "whom": true,
	// tech-specific stop words that are too generic
	"new": true, "latest": true, "announced": true, "says": true, "report": true,
	"according": true, "also": true, "now": true, "first": true, "one": true,
	"two": true, "three": true, "even": true, "still": true, "back": true,
	"well": true, "way": true, "many": true, "much": true, "us": true,
	"make": true, "made": true, "get": true, "got": true, "go": true,
	"going": true, "come": true, "came": true, "take": true, "took": true,
	"know": true, "knows": true, "see": true, "saw": true, "think": true,
	"really": true, "already": true, "since": true, "years": true, "year": true,
	"every": true, "next": true, "last": true, "long": true, "great": true,
	"old": true, "big": true, "high": true, "world": true, "work": true,
	"working": true, "people": true, "don": true, "didn": true, "doesn": true,
	"won": true, "isn": true, "wasn": true, "aren": true, "weren": true,
	"couldn": true, "shouldn": true, "wouldn": true, "hasn": true, "haven": true,
}

// reNonWord matches non-word characters (for tokenization).
var reNonWord = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// reHyphenated matches hyphenated tech terms (e.g., GPT-5, Llama-3).
var reHyphenated = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*-[A-Za-z0-9]+`)

// extractKeywords extracts keyword frequencies from article texts.
func extractKeywords(articles []articleText) map[string]*keywordData {
	freq := make(map[string]*keywordData)

	for _, art := range articles {
		text := art.Title + " " + art.Summary
		// Apply time weight: newer articles get higher weight
		timeWeight := calcTimeWeight(art.CollectedAt)

		// Extract hyphenated terms first (e.g., GPT-5, Llama-3)
		hyphenated := reHyphenated.FindAllString(text, -1)
		for _, term := range hyphenated {
			term = strings.ToLower(strings.TrimSpace(term))
			if len(term) < 3 {
				continue
			}
			if kd, ok := freq[term]; ok {
				kd.Frequency += timeWeight
				kd.Sources[art.Source] = true
				kd.ArticleIDs[art.ID] = true
			} else {
				freq[term] = &keywordData{
					Frequency:  timeWeight,
					Sources:    map[string]bool{art.Source: true},
					ArticleIDs: map[int64]bool{art.ID: true},
				}
			}
		}

		// Tokenize by non-word characters
		tokens := reNonWord.Split(text, -1)
		for _, token := range tokens {
			token = strings.ToLower(strings.TrimSpace(token))
			if len(token) < 2 {
				continue
			}
			if isStopWord(token) {
				continue
			}
			// Filter pure numbers
			if isPureNumber(token) {
				continue
			}
			// For Chinese characters, extract bigrams
			if isChinese(token) {
				bigrams := extractChineseBigrams(token)
				for _, bg := range bigrams {
					if isStopWord(bg) {
						continue
					}
					addKeyword(freq, bg, art.Source, art.ID, timeWeight)
				}
				// Also add the full token if >= 2 Chinese chars
				if len([]rune(token)) >= 2 {
					addKeyword(freq, token, art.Source, art.ID, timeWeight)
				}
			} else if isMixedScript(token) {
				// Mixed script (e.g., "AI芯片"): split and process parts
				parts := splitMixed(token)
				for _, part := range parts {
					if len(part) < 2 || isStopWord(part) || isPureNumber(part) {
						continue
					}
					addKeyword(freq, part, art.Source, art.ID, timeWeight)
				}
			} else {
				// English or other single-script token
				if len(token) < 3 {
					continue
				}
				addKeyword(freq, token, art.Source, art.ID, timeWeight)
			}
		}
	}

	return freq
}

// addKeyword adds a keyword occurrence to the frequency map.
func addKeyword(freq map[string]*keywordData, kw, source string, articleID int64, weight float64) {
	if kd, ok := freq[kw]; ok {
		kd.Frequency += weight
		kd.Sources[source] = true
		kd.ArticleIDs[articleID] = true
	} else {
		freq[kw] = &keywordData{
			Frequency:  weight,
			Sources:    map[string]bool{source: true},
			ArticleIDs: map[int64]bool{articleID: true},
		}
	}
}

// keywordData holds accumulated data for a keyword.
type keywordData struct {
	Frequency  float64
	Sources    map[string]bool
	ArticleIDs map[int64]bool
}

// isStopWord checks if a token is a stop word (Chinese or English).
func isStopWord(token string) bool {
	return chineseStopWords[token] || englishStopWords[token]
}

// isPureNumber checks if a string is purely numeric.
func isPureNumber(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// isChinese checks if a rune is a CJK character.
func isChinese(token string) bool {
	cnCount := 0
	for _, r := range token {
		if unicode.Is(unicode.Han, r) {
			cnCount++
		}
	}
	return cnCount > 0 && cnCount == len([]rune(token))
}

// isMixedScript checks if a token contains both Chinese and non-Chinese characters.
func isMixedScript(token string) bool {
	hasChinese := false
	hasNonChinese := false
	for _, r := range token {
		if unicode.Is(unicode.Han, r) {
			hasChinese = true
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			hasNonChinese = true
		}
	}
	return hasChinese && hasNonChinese
}

// splitMixed splits a mixed-script token into Chinese and non-Chinese parts.
func splitMixed(token string) []string {
	var parts []string
	var current strings.Builder
	currentType := 0 // 0=none, 1=chinese, 2=non-chinese

	for _, r := range token {
		isCn := unicode.Is(unicode.Han, r)
		isLetterOrDigit := unicode.IsLetter(r) || unicode.IsDigit(r)

		rType := 0
		if isCn {
			rType = 1
		} else if isLetterOrDigit {
			rType = 2
		}

		if rType != 0 && rType == currentType {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				parts = append(parts, current.String())
			}
			current.Reset()
			if rType != 0 {
				current.WriteRune(r)
				currentType = rType
			} else {
				currentType = 0
			}
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// extractChineseBigrams extracts bigrams from a Chinese string.
func extractChineseBigrams(s string) []string {
	runes := []rune(s)
	if len(runes) < 2 {
		return nil
	}
	var bigrams []string
	for i := 0; i < len(runes)-1; i++ {
		bigrams = append(bigrams, string(runes[i:i+2]))
	}
	return bigrams
}

// calcTimeWeight calculates a time weight for an article (newer = higher weight).
func calcTimeWeight(collectedAt string) float64 {
	if collectedAt == "" {
		return 0.5
	}

	t, err := time.Parse(time.RFC3339, collectedAt)
	if err != nil {
		// Try other formats
		t, err = time.Parse("2006-01-02T15:04:05Z07:00", collectedAt)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", collectedAt)
			if err != nil {
				return 0.5
			}
		}
	}

	hoursSince := time.Since(t).Hours()
	if hoursSince < 0 {
		hoursSince = 0
	}

	// Weight decays from 2.0 (fresh) to 0.2 (30 days old)
	// Using: weight = 2.0 * exp(-hoursSince / 360) where 360h = 15 days
	weight := 2.0 * math.Exp(-hoursSince / 360.0)
	if weight < 0.2 {
		weight = 0.2
	}
	return weight
}

// --- Main analysis methods ---

// GetHotTopics returns trending topics for a given period.
func (ta *TrendAnalyzer) GetHotTopics(period string, limit int) (*HotTopicsResponse, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	days := parsePeriod(period)
	now := time.Now()

	// Fetch articles in the period
	articles, err := ta.fetchArticles(days)
	if err != nil {
		return nil, fmt.Errorf("fetch articles: %w", err)
	}

	if len(articles) == 0 {
		return &HotTopicsResponse{
			Period:      period,
			Topics:      []TrendTopic{},
			GeneratedAt: now.UTC().Format(time.RFC3339),
		}, nil
	}

	// Extract keywords
	kwFreq := extractKeywords(articles)

	// Get recent 24h vs previous 24h counts for trend direction
	recentArticles, err := ta.fetchArticles(1)
	if err != nil {
		recentArticles = []articleText{}
	}
	prevArticles, err := ta.fetchArticlesRange(2, 1) // 2 days ago to 1 day ago
	if err != nil {
		prevArticles = []articleText{}
	}

	recentFreq := extractKeywords(recentArticles)
	prevFreq := extractKeywords(prevArticles)

	// Filter out low-frequency keywords (appeared in less than 2 articles)
	filtered := make(map[string]*keywordData)
	for kw, data := range kwFreq {
		if len(data.ArticleIDs) < 2 {
			continue
		}
		filtered[kw] = data
	}

	// Normalize and merge similar keywords
	merged := mergeSimilarKeywords(filtered)

	// Build topics
	topics := make([]TrendTopic, 0, len(merged))
	maxFreq := 0.0
	for _, data := range merged {
		if data.Frequency > maxFreq {
			maxFreq = data.Frequency
		}
	}

	for kw, data := range merged {
		topic := TrendTopic{
			Keyword:      kw,
			ArticleCount: len(data.ArticleIDs),
		}

		// Recent count (24h)
		if rd, ok := recentFreq[kw]; ok {
			topic.RecentCount = len(rd.ArticleIDs)
		}

		// Trend direction
		recentCount := 0
		prevCount := 0
		if rd, ok := recentFreq[kw]; ok {
			recentCount = len(rd.ArticleIDs)
		}
		if pd, ok := prevFreq[kw]; ok {
			prevCount = len(pd.ArticleIDs)
		}
		topic.Trend = calcTrendDirection(recentCount, prevCount)

		// Source diversity
		sourceCount := len(data.Sources)

		// Composite score: freq(40%) + growth(25%) + diversity(15%) + engagement(20%)
		freqScore := normalizeScore(data.Frequency, maxFreq)
		growthScore := calcGrowthScore(recentCount, prevCount)
		diversityScore := normalizeScore(float64(sourceCount), 10) // 10 sources = 100%
		engagementScore := calcEngagementScore(articles, kw)

		topic.Score = math.Round((freqScore*40 + growthScore*25 + diversityScore*15 + engagementScore*20)*10) / 10
		if topic.Score > 100 {
			topic.Score = 100
		}

		// Find top articles for this keyword
		topic.TopArticles = ta.findTopArticles(kw, articles, 3)

		// Find related tags (other keywords that co-occur frequently)
		topic.RelatedTags = ta.findRelatedTags(kw, articles, 5)

		topics = append(topics, topic)
	}

	// Sort by score descending
	sort.Slice(topics, func(i, j int) bool {
		return topics[i].Score > topics[j].Score
	})

	// Limit results
	if len(topics) > limit {
		topics = topics[:limit]
	}

	return &HotTopicsResponse{
		Period:      period,
		Topics:      topics,
		GeneratedAt: now.UTC().Format(time.RFC3339),
	}, nil
}

// GetTimeline returns keyword frequency over time.
func (ta *TrendAnalyzer) GetTimeline(keyword string, days int) (*TimelineResponse, error) {
	if days < 1 {
		days = 14
	}
	if days > 30 {
		days = 30
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))

	// Fetch articles for the period
	articles, err := ta.fetchArticles(days)
	if err != nil {
		return nil, fmt.Errorf("fetch articles: %w", err)
	}

	// Group articles by date
	dateArticles := make(map[string][]articleText)
	for _, art := range articles {
		date := parseDate(art.CollectedAt)
		dateArticles[date] = append(dateArticles[date], art)
	}

	// Build timeline
	data := make([]TimelineDataPoint, 0, days)
	now := time.Now()
	var peak *TimelineDataPoint

	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		dayArticles := dateArticles[date]

		count := 0
		for _, art := range dayArticles {
			text := strings.ToLower(art.Title + " " + art.Summary)
			if strings.Contains(text, keyword) {
				count++
			}
		}

		dp := TimelineDataPoint{Date: date, Count: count}
		data = append(data, dp)

		if peak == nil || count > peak.Count {
			dpCopy := dp
			peak = &dpCopy
		}
	}

	return &TimelineResponse{
		Keyword: keyword,
		Days:    days,
		Data:    data,
		Peak:    peak,
	}, nil
}

// GetStoryPitches returns recommended writing topics.
func (ta *TrendAnalyzer) GetStoryPitches(limit int) ([]StoryPitch, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 30 {
		limit = 30
	}

	// Get hot topics from the last 7 days
	hotResp, err := ta.GetHotTopics("7d", limit * 2)
	if err != nil {
		return nil, err
	}

	// Get articles for scoring
	articles, err := ta.fetchArticles(7)
	if err != nil {
		return nil, err
	}

	articleMap := make(map[int64]store.Article)
	for _, art := range articles {
		// We need full article data for sources; use what we have
		articleMap[art.ID] = store.Article{
			ID:      art.ID,
			Title:   art.Title,
			Source:  art.Source,
			CollectedAt: art.CollectedAt,
		}
	}

	pitches := make([]StoryPitch, 0)
	for _, topic := range hotResp.Topics {
		pitch := StoryPitch{
			Topic:             generatePitchTitle(topic.Keyword),
			RelatedArticlesCount: topic.ArticleCount,
			Writability:       calcWritability(topic),
		}

		// Top sources
		sources := make([]string, 0)
		if sourceList := ta.getKeywordSources(topic.Keyword, articles); len(sourceList) > 0 {
			for _, s := range sourceList {
				sources = append(sources, s)
			}
		}
		pitch.TopSources = sources

		// Reason
		trendText := ""
		switch topic.Trend {
		case "rising":
			trendText = "热度上升"
		case "stable":
			trendText = "热度稳定"
		case "declining":
			trendText = "热度下降"
		}
		pitch.Reason = fmt.Sprintf("近7天相关文章%d篇，%s，覆盖%d个数据源",
			topic.ArticleCount, trendText, len(sources))

		// Writing angle
		pitch.Angle = generateWritingAngle(topic.Keyword)

		// Score: writability weighted with hotness
		pitch.Score = math.Round((pitch.Writability*0.6+topic.Score*0.4)*10) / 10
		if pitch.Score > 100 {
			pitch.Score = 100
		}

		pitches = append(pitches, pitch)
	}

	// Sort by score
	sort.Slice(pitches, func(i, j int) bool {
		return pitches[i].Score > pitches[j].Score
	})

	if len(pitches) > limit {
		pitches = pitches[:limit]
	}

	return pitches, nil
}

// GetRelatedTopics returns topics related to a given keyword.
func (ta *TrendAnalyzer) GetRelatedTopics(keyword string, limit int) (*RelatedTopicsResponse, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 30 {
		limit = 30
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))

	// Fetch articles from last 7 days
	articles, err := ta.fetchArticles(7)
	if err != nil {
		return nil, err
	}

	// Find articles containing the keyword
	var matchedArticles []articleText
	for _, art := range articles {
		text := strings.ToLower(art.Title + " " + art.Summary)
		if strings.Contains(text, keyword) {
			matchedArticles = append(matchedArticles, art)
		}
	}

	if len(matchedArticles) == 0 {
		return &RelatedTopicsResponse{
			Keyword:       keyword,
			RelatedTopics: []RelatedTopic{},
		}, nil
	}

	// Extract keywords from matched articles and find co-occurrence
	coOccur := make(map[string]int)
	for _, art := range matchedArticles {
		text := art.Title + " " + art.Summary
		tokens := reNonWord.Split(text, -1)
		seen := make(map[string]bool)
		for _, token := range tokens {
			token = strings.ToLower(strings.TrimSpace(token))
			if token == keyword || len(token) < 2 || isStopWord(token) || isPureNumber(token) {
				continue
			}
			if !seen[token] {
				coOccur[token]++
				seen[token] = true
			}
		}
		// Also add hyphenated terms
		hyphenated := reHyphenated.FindAllString(text, -1)
		for _, term := range hyphenated {
			term = strings.ToLower(strings.TrimSpace(term))
			if term == keyword || len(term) < 3 {
				continue
			}
			if !seen[term] {
				coOccur[term]++
				seen[term] = true
			}
		}
	}

	// Convert to slice and sort by co-occurrence
	related := make([]RelatedTopic, 0)
	for kw, count := range coOccur {
		if count < 2 {
			continue
		}
		score := math.Round(float64(count)/float64(len(matchedArticles))*1000) / 10.0
		related = append(related, RelatedTopic{
			Keyword: kw,
			CoOccur: count,
			Score:   score,
		})
	}

	sort.Slice(related, func(i, j int) bool {
		return related[i].Score > related[j].Score
	})

	if len(related) > limit {
		related = related[:limit]
	}

	return &RelatedTopicsResponse{
		Keyword:       keyword,
		RelatedTopics: related,
	}, nil
}

// --- Helper functions ---

// fetchArticles fetches articles from the last N days.
func (ta *TrendAnalyzer) fetchArticles(days int) ([]articleText, error) {
	query := `
		SELECT id, title, COALESCE(ai_summary, summary) as summary, source, collected_at,
		       CASE WHEN ai_summary IS NOT NULL AND ai_summary != '' THEN 1 ELSE 0 END as has_ai,
		       COALESCE(importance_score, 0)
		FROM articles
		WHERE collected_at >= datetime('now', ?, 'localtime')
		ORDER BY collected_at DESC
	`

	var rows *sql.Rows
	var err error

	if days <= 1 {
		rows, err = ta.db.Query(query, "-1 day")
	} else {
		rows, err = ta.db.Query(query, fmt.Sprintf("-%d days", days))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []articleText
	for rows.Next() {
		var art articleText
		if err := rows.Scan(&art.ID, &art.Title, &art.Summary, &art.Source, &art.CollectedAt, &art.HasAISummary, &art.ImportanceScore); err != nil {
			return nil, err
		}
		articles = append(articles, art)
	}
	return articles, rows.Err()
}

// fetchArticlesRange fetches articles between (startDaysAgo) and (endDaysAgo) days ago.
func (ta *TrendAnalyzer) fetchArticlesRange(startDaysAgo, endDaysAgo int) ([]articleText, error) {
	query := `
		SELECT id, title, COALESCE(ai_summary, summary) as summary, source, collected_at,
		       CASE WHEN ai_summary IS NOT NULL AND ai_summary != '' THEN 1 ELSE 0 END as has_ai,
		       COALESCE(importance_score, 0)
		FROM articles
		WHERE collected_at >= datetime('now', ?, 'localtime')
		  AND collected_at < datetime('now', ?, 'localtime')
		ORDER BY collected_at DESC
	`

	rows, err := ta.db.Query(query,
		fmt.Sprintf("-%d days", startDaysAgo),
		fmt.Sprintf("-%d days", endDaysAgo),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []articleText
	for rows.Next() {
		var art articleText
		if err := rows.Scan(&art.ID, &art.Title, &art.Summary, &art.Source, &art.CollectedAt, &art.HasAISummary, &art.ImportanceScore); err != nil {
			return nil, err
		}
		articles = append(articles, art)
	}
	return articles, rows.Err()
}

// parsePeriod converts a period string to number of days.
func parsePeriod(period string) int {
	switch strings.ToLower(period) {
	case "24h":
		return 1
	case "30d":
		return 30
	default:
		return 7
	}
}

// calcTrendDirection determines trend direction from recent vs previous counts.
func calcTrendDirection(recent, prev int) string {
	if prev == 0 {
		if recent > 0 {
			return "rising"
		}
		return "stable"
	}
	change := float64(recent-prev) / float64(prev) * 100
	if change > 30 {
		return "rising"
	}
	if change < -30 {
		return "declining"
	}
	return "stable"
}

// normalizeScore normalizes a value to [0, 100] range.
func normalizeScore(value, maxValue float64) float64 {
	if maxValue <= 0 {
		return 0
	}
	score := (value / maxValue) * 100
	if score > 100 {
		score = 100
	}
	return score
}

// calcGrowthScore calculates growth score based on recent vs previous counts.
func calcGrowthScore(recent, prev int) float64 {
	if recent == 0 && prev == 0 {
		return 0
	}
	if prev == 0 {
		return 100 // From 0 to something = max growth
	}
	change := float64(recent-prev) / float64(prev)
	// Map growth to 0-100: 0% change = 50, +100% = 100, -100% = 0
	score := 50 + change*50
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

// calcEngagementScore calculates engagement score based on related articles' importance scores.
func calcEngagementScore(articles []articleText, keyword string) float64 {
	keyword = strings.ToLower(keyword)
	totalScore := 0.0
	count := 0

	for _, art := range articles {
		text := strings.ToLower(art.Title + " " + art.Summary)
		if strings.Contains(text, keyword) {
			totalScore += art.ImportanceScore
			count++
		}
	}

	if count == 0 {
		return 0
	}

	avgScore := totalScore / float64(count)
	return normalizeScore(avgScore, 80) // 80+ importance = 100
}

// findTopArticles finds top articles containing a keyword.
func (ta *TrendAnalyzer) findTopArticles(keyword string, articles []articleText, limit int) []TopicArticle {
	keyword = strings.ToLower(keyword)
	var matched []TopicArticle

	for _, art := range articles {
		text := strings.ToLower(art.Title + " " + art.Summary)
		if strings.Contains(text, keyword) {
			// Parse published_at for display
			matched = append(matched, TopicArticle{
				ID:          art.ID,
				Title:       art.Title,
				Source:      art.Source,
				PublishedAt: parseNullableTime(art.CollectedAt),
			})
		}
	}

	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched
}

// findRelatedTags finds related tags for a keyword.
func (ta *TrendAnalyzer) findRelatedTags(keyword string, articles []articleText, limit int) []string {
	keyword = strings.ToLower(keyword)
	coOccur := make(map[string]int)

	for _, art := range articles {
		text := strings.ToLower(art.Title + " " + art.Summary)
		if !strings.Contains(text, keyword) {
			continue
		}
		tokens := reNonWord.Split(text, -1)
		seen := make(map[string]bool)
		for _, token := range tokens {
			token = strings.TrimSpace(token)
			if token == keyword || len(token) < 2 || isStopWord(token) || isPureNumber(token) {
				continue
			}
			if !seen[token] {
				coOccur[token]++
				seen[token] = true
			}
		}
	}

	// Sort by co-occurrence
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range coOccur {
		if v >= 2 {
			sorted = append(sorted, kv{k, v})
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].v > sorted[j].v
	})

	tags := make([]string, 0)
	for _, kv := range sorted {
		tags = append(tags, kv.k)
		if len(tags) >= limit {
			break
		}
	}
	return tags
}

// mergeSimilarKeywords merges keywords that are likely the same topic.
func mergeSimilarKeywords(freq map[string]*keywordData) map[string]*keywordData {
	merged := make(map[string]*keywordData)

	for kw, data := range freq {
		// Check if there's already a similar key
		found := false
		for existing := range merged {
			if isSimilar(kw, existing) {
				// Merge into the longer/more common one
				if len(kw) > len(existing) {
					merged[kw] = data
					// Also merge the old data
					oldData := merged[existing]
					data.Frequency += oldData.Frequency * 0.3 // Partial credit
					for s := range oldData.Sources {
						data.Sources[s] = true
					}
					for id := range oldData.ArticleIDs {
						data.ArticleIDs[id] = true
					}
					delete(merged, existing)
				} else {
					merged[existing].Frequency += data.Frequency * 0.3
					for s := range data.Sources {
						merged[existing].Sources[s] = true
					}
					for id := range data.ArticleIDs {
						merged[existing].ArticleIDs[id] = true
					}
				}
				found = true
				break
			}
		}
		if !found {
			merged[kw] = data
		}
	}

	return merged
}

// isSimilar checks if two keywords are likely the same topic.
func isSimilar(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if a == b {
		return true
	}

	// Check if one contains the other (e.g., "gpt" and "gpt5" — but not "gpt" and "pga")
	if strings.Contains(a, b) || strings.Contains(b, a) {
		// Only merge if the shorter one is >= 2 chars
		shorter := a
		if len(b) < len(a) {
			shorter = b
		}
		if len(shorter) >= 3 {
			return true
		}
	}

	// Check if they differ only by hyphen/dash (e.g., "gpt-5" and "gpt5")
	aNorm := strings.ReplaceAll(a, "-", "")
	bNorm := strings.ReplaceAll(b, "-", "")
	if aNorm == bNorm && len(aNorm) >= 3 {
		return true
	}

	return false
}

// parseDate extracts date from a datetime string.
func parseDate(dt string) string {
	if dt == "" {
		return time.Now().Format("2006-01-02")
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, dt); err == nil {
		return t.Format("2006-01-02")
	}
	// Try other format
	if t, err := time.Parse("2006-01-02T15:04:05Z07:00", dt); err == nil {
		return t.Format("2006-01-02")
	}
	if t, err := time.Parse("2006-01-02 15:04:05", dt); err == nil {
		return t.Format("2006-01-02")
	}
	// Fallback
	if len(dt) >= 10 {
		return dt[:10]
	}
	return time.Now().Format("2006-01-02")
}

// parseNullableTime creates a string pointer from a datetime string.
func parseNullableTime(dt string) *string {
	if dt == "" {
		return nil
	}
	s := dt
	return &s
}

// calcWritability calculates how suitable a topic is for writing a deep blog post.
func calcWritability(topic TrendTopic) float64 {
	var score float64

	// Article count (0-30): need >= 5 articles for good material
	if topic.ArticleCount >= 10 {
		score += 30
	} else if topic.ArticleCount >= 5 {
		score += 20 + float64(topic.ArticleCount-5)*2
	} else if topic.ArticleCount >= 2 {
		score += float64(topic.ArticleCount) * 4
	}

	// Source diversity (0-25): 3+ sources is good
	sourceCount := 0
	if topic.RelatedTags != nil {
		sourceCount = len(topic.RelatedTags)
	}
	if sourceCount >= 5 {
		score += 25
	} else if sourceCount >= 3 {
		score += 15 + float64(sourceCount-3)*5
	} else if sourceCount >= 1 {
		score += float64(sourceCount) * 5
	}

	// Trend direction (0-25): rising > stable > declining
	switch topic.Trend {
	case "rising":
		score += 25
	case "stable":
		score += 15
	case "declining":
		score += 8
	}

	// Summary availability (0-20): top articles with AI summaries
	if len(topic.TopArticles) > 0 {
		// Check if articles have summaries (we approximate from article count)
		if topic.ArticleCount >= 5 {
			score += 20
		} else if topic.ArticleCount >= 3 {
			score += 12
		} else {
			score += float64(topic.ArticleCount) * 4
		}
	}

	if score > 100 {
		score = 100
	}
	return math.Round(score*10) / 10
}

// generatePitchTitle creates a catchy pitch title from a keyword.
func generatePitchTitle(keyword string) string {
	// Simple title generation based on keyword patterns
	keyword = strings.TrimSpace(keyword)
	lower := strings.ToLower(keyword)

	// Common patterns
	patterns := map[string]string{
		"gpt":    "GPT 系列最新动态与深度分析",
		"claude": "Claude 生态发展观察",
		"gemini": "Gemini 技术演进追踪",
		"llama":  "Llama 开源模型生态报告",
		"openai": "OpenAI 战略布局分析",
		"google": "Google AI 布局全景解读",
		"meta":   "Meta AI 研究前沿速递",
		"deepseek": "DeepSeek 技术突破解析",
		"mistral": "Mistral 模型家族进展",
		"agent":  "AI Agent 趋势与应用前景",
		"rag":    "RAG 技术实践与优化策略",
	}

	for k, v := range patterns {
		if strings.Contains(lower, k) {
			return v
		}
	}

	return keyword + " 趋势分析与深度解读"
}

// generateWritingAngle suggests a writing angle for a keyword.
func generateWritingAngle(keyword string) string {
	keyword = strings.TrimSpace(keyword)
	lower := strings.ToLower(keyword)

	angles := map[string]string{
		"gpt":    "技术架构对比 + 行业影响分析",
		"claude": "产品能力评测 + 竞争格局分析",
		"gemini": "多模态能力评估 + 生态整合路径",
		"llama":  "开源模型对比 + 部署实践指南",
		"openai": "战略分析 + 产品矩阵解读",
		"agent":  "框架对比 + 落地案例 + 未来展望",
		"rag":    "技术选型 + 优化实践 + 效果评估",
	}

	for k, v := range angles {
		if strings.Contains(lower, k) {
			return v
		}
	}

	return "技术分析 + 应用场景 + 行业趋势"
}

// getKeywordSources returns the list of sources that contain a keyword.
func (ta *TrendAnalyzer) getKeywordSources(keyword string, articles []articleText) []string {
	keyword = strings.ToLower(keyword)
	sources := make(map[string]bool)
	for _, art := range articles {
		text := strings.ToLower(art.Title + " " + art.Summary)
		if strings.Contains(text, keyword) {
			sources[art.Source] = true
		}
	}

	result := make([]string, 0, len(sources))
	for s := range sources {
		result = append(result, s)
	}
	// Sort by source name
	sort.Strings(result)
	if len(result) > 5 {
		result = result[:5]
	}
	return result
}
