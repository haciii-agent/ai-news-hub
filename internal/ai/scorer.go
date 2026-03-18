package ai

import (
	"strings"
	"time"

	"ai-news-hub/internal/store"
)

// CalculateImportance computes a multi-dimensional importance score (0-100) for an article.
// This is a pure local computation — no external API calls.
func CalculateImportance(article store.Article) float64 {
	var total float64

	// 1. Source weight (30%)
	sourceScore := scoreSource(article.Source)
	total += sourceScore * 0.30

	// 2. Timeliness (25%)
	timeScore := scoreTimeliness(article.PublishedAt)
	total += timeScore * 0.25

	// 3. Category hotness (15%) — placeholder, will be overridden if categoryCounts provided
	// Default: assume mid-range since we don't have the count here
	total += 60.0 * 0.15

	// 4. Title keywords (15%)
	keywordScore := scoreKeywords(article.Title)
	total += keywordScore * 0.15

	// 5. Summary quality (10%)
	summaryScore := scoreSummary(article)
	total += summaryScore * 0.10

	// 6. Image presence (5%)
	imageScore := scoreImage(article.ImageURL)
	total += imageScore * 0.05

	// Clamp to [0, 100]
	if total < 0 {
		total = 0
	}
	if total > 100 {
		total = 100
	}

	return total
}

// CalculateImportanceWithCategoryStats computes importance with actual category hotness data.
func CalculateImportanceWithCategoryStats(article store.Article, categoryCounts map[string]int) float64 {
	var total float64

	sourceScore := scoreSource(article.Source)
	total += sourceScore * 0.30

	timeScore := scoreTimeliness(article.PublishedAt)
	total += timeScore * 0.25

	catScore := scoreCategoryHotness(article.Category, categoryCounts)
	total += catScore * 0.15

	keywordScore := scoreKeywords(article.Title)
	total += keywordScore * 0.15

	summaryScore := scoreSummary(article)
	total += summaryScore * 0.10

	imageScore := scoreImage(article.ImageURL)
	total += imageScore * 0.05

	if total < 0 {
		total = 0
	}
	if total > 100 {
		total = 100
	}

	return total
}

// scoreSource returns a score based on the news source reputation.
// Core sources (HN, TechCrunch, MIT Tech Review, etc.) = 90
// Normal sources = 70
// Edge/unknown sources = 50
func scoreSource(source string) float64 {
	if source == "" {
		return 50
	}

	s := strings.ToLower(source)

	// Core sources
	coreSources := []string{
		"hacker news", "techcrunch", "mit technology review",
		"the verge", "wired", "arstechnica", "reuters",
		"bbc", "nytimes", "washington post", "nature",
		"science", "36kr", "钛媒体", "虎嗅", "量子位",
		"机器之心", "arXiv",
	}
	for _, cs := range coreSources {
		if strings.Contains(s, cs) {
			return 90
		}
	}

	// Normal sources — most RSS feeds
	return 70
}

// scoreTimeliness returns a score based on how recently the article was published.
// 24h = 100, 48h = 80, 7d = 50, 30d = 20, >30d = 10
func scoreTimeliness(publishedAt *string) float64 {
	if publishedAt == nil || *publishedAt == "" {
		return 50 // unknown time, mid-range
	}

	t, err := time.Parse(time.RFC3339, *publishedAt)
	if err != nil {
		// Try other common formats
		t, err = time.Parse("2006-01-02T15:04:05Z", *publishedAt)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", *publishedAt)
			if err != nil {
				return 50
			}
		}
	}

	hoursDiff := time.Since(t).Hours()

	if hoursDiff < 0 {
		hoursDiff = 0
	}

	switch {
	case hoursDiff <= 24:
		return 100
	case hoursDiff <= 48:
		return 80
	case hoursDiff <= 168: // 7 days
		return 50
	case hoursDiff <= 720: // 30 days
		return 20
	default:
		return 10
	}
}

// scoreCategoryHotness returns a score based on how popular the category is.
// More articles in a category = hotter topic.
func scoreCategoryHotness(category string, categoryCounts map[string]int) float64 {
	count := categoryCounts[category]

	if count <= 0 {
		return 30 // rare category
	}

	// Normalize: 100 articles+ = 100, scale linearly
	score := float64(count) / 100.0 * 100
	if score > 100 {
		score = 100
	}
	return score
}

// scoreKeywords checks for hot keywords in the article title.
func scoreKeywords(title string) float64 {
	if title == "" {
		return 20
	}

	t := strings.ToLower(title)

	// High-impact keywords (each adds points)
	hotKeywords := []string{
		"gpt", "chatgpt", "openai", "claude", "gemini", "llama",
		"突破", "首发", "发布", "融资", "收购", "上市",
		"开源", "安全", "监管", "ban", "突破性",
		"transformer", "diffusion", "多模态", "agent", "agi",
		"transformer", "sora", "midjourney", "stable diffusion",
		"量子计算", "芯片", "半导体", "gpu", "nvidia",
		"人工智能", "大模型", "deepseek",
	}

	matchCount := 0
	for _, kw := range hotKeywords {
		if strings.Contains(t, kw) {
			matchCount++
		}
	}

	// Score: 0 matches = 30, 1 match = 60, 2+ matches = 90+
	switch {
	case matchCount == 0:
		return 30
	case matchCount == 1:
		return 60
	case matchCount == 2:
		return 80
	default:
		return 95
	}
}

// scoreSummary checks summary availability and quality.
// AI summary = 100, original summary = 60, none = 20
func scoreSummary(article store.Article) float64 {
	if article.AISummary != "" {
		return 100
	}
	if article.Summary != "" {
		return 60
	}
	return 20
}

// scoreImage checks if the article has a cover image.
func scoreImage(imageURL string) float64 {
	if imageURL != "" {
		return 100
	}
	return 30
}

// RecalculateScores recalculates importance scores for all articles given the store.
// Returns the number of articles updated.
func RecalculateScores(store store.ArticleStore) (int, error) {
	// Get category counts for hotness scoring
	catStats, err := store.GetCategoryStats()
	if err != nil {
		return 0, err
	}
	categoryCounts := make(map[string]int)
	for _, cs := range catStats {
		categoryCounts[cs.Category] = cs.Count
	}

	// Get all article IDs
	ids, err := store.GetAllArticleIDs()
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, id := range ids {
		article, err := store.GetArticleByID(id)
		if err != nil || article == nil {
			continue
		}
		score := CalculateImportanceWithCategoryStats(*article, categoryCounts)
		if err := store.UpdateImportanceScore(id, score); err != nil {
			continue
		}
		updated++
	}

	return updated, nil
}
