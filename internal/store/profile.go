package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// UserProfile 用户画像
type UserProfile struct {
	UserID             int64             `json:"user_id"`
	Interests          map[string]float64 `json:"interests"`           // 兴趣标签权重 {"GPT": 0.85, "NLP": 0.6}
	PreferredCategories []string          `json:"preferred_categories"` // 偏好分类 ["AI/ML", "科技前沿"]
	UpdatedAt          string            `json:"updated_at"`
}

// ReadingStreak 阅读连续天数
type ReadingStreak struct {
	CurrentStreak    int `json:"current_streak"`
	LongestStreak    int `json:"longest_streak"`
	TotalReadingDays int `json:"total_reading_days"`
}

// InterestTags 内置兴趣标签池
var InterestTags = []string{
	// 大模型
	"GPT", "Claude", "Gemini", "Llama", "GLM", "Qwen", "Mistral", "DeepSeek",
	// 技术方向
	"NLP", "计算机视觉", "强化学习", "多模态", "代码生成", "RAG", "Agent", "AI安全",
	// 行业
	"自动驾驶", "机器人", "医疗AI", "金融AI", "教育AI", "芯片", "云计算",
	// 话题
	"开源", "融资", "产品发布", "学术论文", "政策法规", "创业",
}

// CategoryTagMap 分类到标签的映射
var CategoryTagMap = map[string][]string{
	"AI/ML":     {"GPT", "Claude", "Gemini", "Llama", "GLM", "Qwen", "NLP", "多模态", "代码生成", "RAG", "Agent"},
	"科技前沿":   {"芯片", "云计算", "量子计算", "区块链", "机器人"},
	"商业动态":   {"融资", "创业", "IPO", "收购"},
	"开源生态":   {"开源", "Llama", "Mistral", "DeepSeek"},
	"学术研究":   {"学术论文", "论文", "研究"},
	"政策监管":   {"政策法规", "监管", "AI安全", "隐私"},
	"产品发布":   {"产品发布", "GPT", "Claude", "Gemini"},
	"综合资讯":   {},
}

// ProfileStore 用户画像存储接口
type ProfileStore interface {
	GetProfile(userID int64) (*UserProfile, error)
	UpsertProfile(profile *UserProfile) error
	UpdateProfileInterests(userID int64, interests map[string]float64) error
	UpdatePreferredCategories(userID int64, categories []string) error
	GetReadingStreak(userID int64) (*ReadingStreak, error)
	GetTotalReadsAndBookmarks(userID int64) (int, int, error)
}

type profileStore struct {
	db *sql.DB
}

// NewProfileStore creates a ProfileStore backed by SQLite.
func NewProfileStore(db *sql.DB) ProfileStore {
	return &profileStore{db: db}
}

// GetProfile retrieves user profile, returns empty profile if not exists.
func (s *profileStore) GetProfile(userID int64) (*UserProfile, error) {
	profile := &UserProfile{
		UserID:             userID,
		Interests:          make(map[string]float64),
		PreferredCategories: []string{},
	}

	var interestsJSON, categoriesJSON sql.NullString
	var updatedAt sql.NullString

	err := s.db.QueryRow(
		`SELECT interests, preferred_categories, updated_at FROM user_profiles WHERE user_id = ?`,
		userID,
	).Scan(&interestsJSON, &categoriesJSON, &updatedAt)

	if err == sql.ErrNoRows {
		return profile, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get profile for user %d: %w", userID, err)
	}

	if interestsJSON.Valid && interestsJSON.String != "" {
		if err := json.Unmarshal([]byte(interestsJSON.String), &profile.Interests); err != nil {
			log.Printf("[profile] failed to parse interests JSON for user %d: %v", userID, err)
			profile.Interests = make(map[string]float64)
		}
	}

	if categoriesJSON.Valid && categoriesJSON.String != "" {
		if err := json.Unmarshal([]byte(categoriesJSON.String), &profile.PreferredCategories); err != nil {
			log.Printf("[profile] failed to parse categories JSON for user %d: %v", userID, err)
			profile.PreferredCategories = []string{}
		}
	}

	if updatedAt.Valid {
		profile.UpdatedAt = updatedAt.String
	}

	return profile, nil
}

// UpsertProfile creates or updates the full user profile.
func (s *profileStore) UpsertProfile(profile *UserProfile) error {
	interestsJSON, err := json.Marshal(profile.Interests)
	if err != nil {
		return fmt.Errorf("marshal interests: %w", err)
	}
	categoriesJSON, err := json.Marshal(profile.PreferredCategories)
	if err != nil {
		return fmt.Errorf("marshal categories: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO user_profiles (user_id, interests, preferred_categories, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET
			interests = excluded.interests,
			preferred_categories = excluded.preferred_categories,
			updated_at = CURRENT_TIMESTAMP
	`, profile.UserID, string(interestsJSON), string(categoriesJSON))

	if err != nil {
		return fmt.Errorf("upsert profile for user %d: %w", profile.UserID, err)
	}
	return nil
}

// UpdateProfileInterests incrementally updates the interest weights for a user.
// It reads the current profile, applies the update (boost matching tags, decay others),
// normalizes, and writes back.
func (s *profileStore) UpdateProfileInterests(userID int64, interests map[string]float64) error {
	profile, err := s.GetProfile(userID)
	if err != nil {
		return err
	}

	// Incremental update: boost matched tags, decay unmatched
	for tag, weight := range interests {
		profile.Interests[tag] = profile.Interests[tag] + weight
	}

	// Decay unmatched tags
	for tag := range profile.Interests {
		if _, matched := interests[tag]; !matched {
			profile.Interests[tag] *= 0.95
		}
	}

	// Normalize weights to [0, 1]
	profile.Interests = normalizeInterests(profile.Interests)

	// Remove very low weights (< 0.01)
	for tag, weight := range profile.Interests {
		if weight < 0.01 {
			delete(profile.Interests, tag)
		}
	}

	// Update preferred categories based on top interests
	profile.updatePreferredCategoriesFromInterests()

	return s.UpsertProfile(profile)
}

// UpdatePreferredCategories allows user to manually set preferred categories.
func (s *profileStore) UpdatePreferredCategories(userID int64, categories []string) error {
	profile, err := s.GetProfile(userID)
	if err != nil {
		return err
	}

	profile.PreferredCategories = categories
	return s.UpsertProfile(profile)
}

// GetReadingStreak calculates the reading streak for a user.
func (s *profileStore) GetReadingStreak(userID int64) (*ReadingStreak, error) {
	streak := &ReadingStreak{}

	rows, err := s.db.Query(`
		SELECT DISTINCT date(read_at) as read_date
		FROM read_history
		WHERE user_id = ?
		ORDER BY read_date DESC
		LIMIT 365
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get reading streak: %w", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("scan reading date: %w", err)
		}
		dates = append(dates, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reading dates: %w", err)
	}

	if len(dates) == 0 {
		return streak, nil
	}

	streak.TotalReadingDays = len(dates)

	// Calculate current streak
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	if dates[0] == today || dates[0] == yesterday {
		streak.CurrentStreak = 1
		for i := 1; i < len(dates); i++ {
			prev, err := time.Parse("2006-01-02", dates[i-1])
			if err != nil {
				break
			}
			curr, err := time.Parse("2006-01-02", dates[i])
			if err != nil {
				break
			}
			if prev.Sub(curr) == 24*time.Hour {
				streak.CurrentStreak++
			} else {
				break
			}
		}
	}

	// Calculate longest streak (already sorted DESC, so iterate all)
	maxStreak := 1
	runStreak := 1
	for i := 1; i < len(dates); i++ {
		prev, err := time.Parse("2006-01-02", dates[i-1])
		if err != nil {
			break
		}
		curr, err := time.Parse("2006-01-02", dates[i])
		if err != nil {
			break
		}
		if prev.Sub(curr) == 24*time.Hour {
			runStreak++
			if runStreak > maxStreak {
				maxStreak = runStreak
			}
		} else {
			runStreak = 1
		}
	}
	streak.LongestStreak = maxStreak

	return streak, nil
}

// GetTotalReadsAndBookmarks returns total read and bookmark counts for a user.
func (s *profileStore) GetTotalReadsAndBookmarks(userID int64) (reads int, bookmarks int, err error) {
	err = s.db.QueryRow(`SELECT COUNT(*) FROM read_history WHERE user_id = ?`, userID).Scan(&reads)
	if err != nil {
		return 0, 0, fmt.Errorf("count reads: %w", err)
	}
	err = s.db.QueryRow(`SELECT COUNT(*) FROM bookmarks WHERE user_id = ?`, userID).Scan(&bookmarks)
	if err != nil {
		return reads, 0, fmt.Errorf("count bookmarks: %w", err)
	}
	return reads, bookmarks, nil
}

// normalizeInterests normalizes weights to [0, 1] range.
func normalizeInterests(interests map[string]float64) map[string]float64 {
	if len(interests) == 0 {
		return interests
	}

	maxVal := 0.0
	for _, v := range interests {
		if v > maxVal {
			maxVal = v
		}
	}

	if maxVal <= 0 {
		return interests
	}

	normalized := make(map[string]float64, len(interests))
	for k, v := range interests {
		normalized[k] = v / maxVal
	}
	return normalized
}

// updatePreferredCategoriesFromInterests infers preferred categories from top interest tags.
func (p *UserProfile) updatePreferredCategoriesFromInterests() {
	// Build a reverse map: tag → categories
	tagToCategories := make(map[string][]string)
	for cat, tags := range CategoryTagMap {
		for _, tag := range tags {
			tagToCategories[tag] = append(tagToCategories[tag], cat)
		}
	}

	// Count category hits from user's interest tags (weighted by interest score)
	catScores := make(map[string]float64)
	for tag, weight := range p.Interests {
		for _, cat := range tagToCategories[tag] {
			catScores[cat] += weight
		}
	}

	// Sort categories by score, take top 3
	type catScore struct {
		cat   string
		score float64
	}
	var sorted []catScore
	for cat, score := range catScores {
		sorted = append(sorted, catScore{cat, score})
	}

	// Simple sort (top 3)
	for i := 0; i < len(sorted) && i < 3; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].score > sorted[i].score {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	p.PreferredCategories = nil
	for i := 0; i < len(sorted) && i < 3; i++ {
		if sorted[i].score > 0.1 {
			p.PreferredCategories = append(p.PreferredCategories, sorted[i].cat)
		}
	}
}

// ExtractTagsFromArticle extracts interest tags from an article's category and title.
// Returns a map of tag → weight boost.
func ExtractTagsFromArticle(category string, title string) map[string]float64 {
	tags := make(map[string]float64)

	// 1. Category → tag mapping
	if catTags, ok := CategoryTagMap[category]; ok {
		for _, tag := range catTags {
			tags[tag] = 0.1
		}
	}

	// 2. Title keyword matching
	titleLower := strings.ToLower(title)
	for _, tag := range InterestTags {
		if strings.Contains(titleLower, strings.ToLower(tag)) {
			tags[tag] = 0.15 // Higher weight for title matches
		}
	}

	return tags
}

// ExtractTagsFromArticleForBookmark extracts tags with higher weight for bookmark actions.
func ExtractTagsFromArticleForBookmark(category string, title string) map[string]float64 {
	tags := ExtractTagsFromArticle(category, title)
	// Bookmark: double the weight
	for tag := range tags {
		tags[tag] *= 2
	}
	return tags
}
