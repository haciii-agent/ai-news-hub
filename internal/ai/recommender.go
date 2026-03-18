package ai

import (
	"math"
	"sort"
	"strings"
	"time"

	"ai-news-hub/internal/store"
)

// Recommender handles content-based recommendation scoring.
type Recommender struct{}

// NewRecommender creates a new Recommender.
func NewRecommender() *Recommender {
	return &Recommender{}
}

// Recommendation represents a scored article recommendation.
type Recommendation struct {
	Article store.Article
	Score   float64
	Reason  string // e.g., "分类匹配", "标签匹配: GPT, NLP"
}

// ScoreForUser calculates a recommendation score (0-100) for a single article
// based on the user's profile.
//
// Score composition:
//   - Category match: 40%
//   - Tag match: 30%
//   - Importance score: 15%
//   - Timeliness: 10%
//   - Source diversity: 5%
func (r *Recommender) ScoreForUser(article store.Article, profile *store.UserProfile, sourceCounts map[string]int) (float64, string) {
	var categoryScore, tagScore, importanceScore, timelinessScore, diversityScore float64
	var reasons []string

	// 1. Category match (40%)
	if len(profile.PreferredCategories) > 0 {
		for _, cat := range profile.PreferredCategories {
			if article.Category == cat {
				categoryScore = 1.0
				reasons = append(reasons, "分类: "+cat)
				break
			}
		}
	}

	// 2. Tag match (30%)
	if len(profile.Interests) > 0 {
		articleTags := r.extractArticleTags(article)
		if len(articleTags) > 0 {
			var totalWeight float64
			var matchedWeight float64
			var matchedTags []string

			// Get user's top interests for scoring
			for tag, userWeight := range profile.Interests {
				totalWeight += userWeight
				if articleTags[tag] {
					matchedWeight += userWeight
					matchedTags = append(matchedTags, tag)
				}
			}

			if totalWeight > 0 {
				tagScore = matchedWeight / totalWeight
				// Cap at 1.0
				if tagScore > 1.0 {
					tagScore = 1.0
				}
			}

			if len(matchedTags) > 0 {
				// Show top 3 matched tags
				sort.Slice(matchedTags, func(i, j int) bool {
					return profile.Interests[matchedTags[i]] > profile.Interests[matchedTags[j]]
				})
				if len(matchedTags) > 3 {
					matchedTags = matchedTags[:3]
				}
				reasons = append(reasons, "标签: "+strings.Join(matchedTags, ", "))
			}
		}
	}

	// 3. Importance score (15%)
	importanceScore = article.ImportanceScore / 100.0

	// 4. Timeliness (10%) — articles from last 7 days score higher
	timelinessScore = r.calcTimeliness(article.PublishedAt)

	// 5. Source diversity (5%) — penalize over-read sources
	diversityScore = r.calcDiversity(article.Source, sourceCounts)

	// Weighted sum
	totalScore := categoryScore*40 + tagScore*30 + importanceScore*15 + timelinessScore*10 + diversityScore*5

	// Build reason string
	reason := ""
	if len(reasons) > 0 {
		reason = strings.Join(reasons, " · ")
	}

	return math.Round(totalScore*10) / 10, reason
}

// extractArticleTags returns which interest tags match an article.
func (r *Recommender) extractArticleTags(article store.Article) map[string]bool {
	tags := make(map[string]bool)

	// From category mapping
	if catTags, ok := store.CategoryTagMap[article.Category]; ok {
		for _, tag := range catTags {
			tags[tag] = true
		}
	}

	// From title
	titleLower := strings.ToLower(article.Title)
	for _, tag := range store.InterestTags {
		if strings.Contains(titleLower, strings.ToLower(tag)) {
			tags[tag] = true
		}
	}

	// From summary
	if article.Summary != "" {
		summaryLower := strings.ToLower(article.Summary)
		for _, tag := range store.InterestTags {
			if strings.Contains(summaryLower, strings.ToLower(tag)) {
				tags[tag] = true
			}
		}
	}

	return tags
}

// calcTimeliness calculates a score [0, 1] based on article recency.
// Articles from the last 24 hours score 1.0, decaying to ~0 over 7 days.
func (r *Recommender) calcTimeliness(publishedAt *string) float64 {
	if publishedAt == nil || *publishedAt == "" {
		return 0.5 // Neutral for articles without publish time
	}

	t, err := time.Parse(time.RFC3339, *publishedAt)
	if err != nil {
		// Try other formats
		t, err = time.Parse("2006-01-02T15:04:05Z07:00", *publishedAt)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", *publishedAt)
			if err != nil {
				return 0.5
			}
		}
	}

	hoursSince := time.Since(t).Hours()
	if hoursSince < 0 {
		hoursSince = 0
	}

	// Exponential decay over 7 days (168 hours)
	decay := math.Exp(-hoursSince / 168.0 * 3)
	return decay
}

// calcDiversity calculates a diversity score [0, 1] that favors less-read sources.
func (r *Recommender) calcDiversity(source string, sourceCounts map[string]int) float64 {
	if sourceCounts == nil {
		return 1.0
	}

	totalReads := 0
	for _, count := range sourceCounts {
		totalReads += count
	}

	if totalReads == 0 {
		return 1.0
	}

	sourceReads := sourceCounts[source]
	ratio := float64(sourceReads) / float64(totalReads)

	// Inverse ratio: less read sources get higher score
	// If source has 50% of reads → score 0.5, if 0% → 1.0
	return 1.0 - ratio
}

// ScoreForHot calculates a simple score for "hot" articles when there's no user profile.
// Uses importance_score + timeliness.
func (r *Recommender) ScoreForHot(article store.Article) float64 {
	importance := article.ImportanceScore / 100.0
	timeliness := r.calcTimeliness(article.PublishedAt)
	total := importance*70 + timeliness*30
	return math.Round(total*10) / 10
}
