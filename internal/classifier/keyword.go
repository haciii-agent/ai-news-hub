package classifier

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// KeywordClassifier implements the Classifier interface using keyword matching.
//
// Classification algorithm:
//  1. For each category, match keywords against title (weight * TitleWeight) and summary (weight * SummaryWeight)
//  2. Normal keywords score KeywordWeight each; boost keywords score BoostWeight each
//  3. If the article has source categories that map to a classifier category, add SourceBoost
//  4. The category with the highest score wins; ties go to the first matching category
//  5. If no category scores > 0, fall back to "综合资讯" (general)
type KeywordClassifier struct {
	mu        sync.RWMutex
	rulesPath string
	rules     *RulesConfig

	// Compiled lowercase keyword sets for fast matching
	compiled map[string]*compiledCategory
}

// compiledCategory holds preprocessed, lowercased keyword sets for a category.
type compiledCategory struct {
	Name          string
	Keywords      []string // lowercased
	BoostKeywords []string // lowercased
}

// KeywordMatchConfig holds tuning parameters for the keyword classifier.
type KeywordMatchConfig struct {
	KeywordWeight       float64 // weight per normal keyword match (default 1.0)
	BoostWeight         float64 // weight per boost keyword match (default 3.0)
	TitleWeight         float64 // additional multiplier for title matches (default 1.5)
	SummaryWeight       float64 // multiplier for summary matches (default 1.0)
	SourceBoost         float64 // boost when source category matches (default from rules.yaml)
}

// DefaultMatchConfig returns the default matching configuration.
func DefaultMatchConfig() KeywordMatchConfig {
	return KeywordMatchConfig{
		KeywordWeight: 1.0,
		BoostWeight:   3.0,
		TitleWeight:   1.5,
		SummaryWeight: 1.0,
		SourceBoost:   2.0,
	}
}

// NewKeywordClassifier creates a new keyword-based classifier by loading rules from the given path.
func NewKeywordClassifier(rulesPath string) (*KeywordClassifier, error) {
	kc := &KeywordClassifier{
		rulesPath: rulesPath,
	}

	if err := kc.load(); err != nil {
		return nil, err
	}

	return kc, nil
}

// load reads rules.yaml and compiles keyword sets.
func (kc *KeywordClassifier) load() error {
	rules, err := LoadRulesConfig(kc.rulesPath)
	if err != nil {
		return err
	}

	compiled := make(map[string]*compiledCategory, len(rules.Categories))
	for key, cat := range rules.Categories {
		compiled[key] = &compiledCategory{
			Name:          cat.Name,
			Keywords:      toLowerSlice(cat.Keywords),
			BoostKeywords: toLowerSlice(cat.BoostKeywords),
		}
	}

	kc.mu.Lock()
	kc.rules = rules
	kc.compiled = compiled
	kc.mu.Unlock()

	log.Printf("[classifier] loaded %d categories from %s", len(compiled), kc.rulesPath)
	return nil
}

// Classify determines the best category for an article.
func (kc *KeywordClassifier) Classify(input *ArticleInput) *ClassifyResult {
	kc.mu.RLock()
	defer kc.mu.RUnlock()

	cfg := DefaultMatchConfig()
	if kc.rules != nil && kc.rules.SourceCategoryBoost > 0 {
		cfg.SourceBoost = kc.rules.SourceCategoryBoost
	}

	titleLower := strings.ToLower(input.Title)
	summaryLower := strings.ToLower(input.Summary)

	scores := make(Scores, len(KnownCategories))
	bestCategory := "general"
	bestScore := 0.0

	// Resolve source categories to internal keys
	sourceCatKeys := make(map[string]bool)
	for _, sc := range input.Category {
		if key := ResolveSourceCategory(sc); key != "" {
			sourceCatKeys[key] = true
		}
	}

	for _, catKey := range KnownCategories {
		cc, ok := kc.compiled[catKey]
		if !ok {
			continue
		}

		score := 0.0

		// Match keywords against title (higher weight) and summary
		for _, kw := range cc.Keywords {
			if kw == "" {
				continue
			}
			if strings.Contains(titleLower, kw) {
				score += cfg.KeywordWeight * cfg.TitleWeight
			} else if strings.Contains(summaryLower, kw) {
				score += cfg.KeywordWeight * cfg.SummaryWeight
			}
		}

		// Match boost keywords (higher weight)
		for _, kw := range cc.BoostKeywords {
			if kw == "" {
				continue
			}
			if strings.Contains(titleLower, kw) {
				score += cfg.BoostWeight * cfg.TitleWeight
			} else if strings.Contains(summaryLower, kw) {
				score += cfg.BoostWeight * cfg.SummaryWeight
			}
		}

		// Source category boost
		if sourceCatKeys[catKey] {
			score += cfg.SourceBoost
		}

		scores[catKey] = score
		if score > bestScore {
			bestScore = score
			bestCategory = catKey
		}
	}

	// If nothing matched, use general as fallback
	if bestScore == 0 {
		bestCategory = "general"
		scores["general"] = 0
	}

	return &ClassifyResult{
		Category: DisplayCategory(bestCategory),
		Scores:   scores,
	}
}

// Reload reloads classification rules from disk.
func (kc *KeywordClassifier) Reload() error {
	return kc.load()
}

// Categories returns the list of all available category names (display names).
func (kc *KeywordClassifier) Categories() []string {
	names := make([]string, len(KnownCategories))
	for i, key := range KnownCategories {
		names[i] = DisplayCategory(key)
	}
	return names
}

// ClassifySimple is a convenience function that classifies based on title and summary only.
// Returns the display name of the best category.
func (kc *KeywordClassifier) ClassifySimple(title, summary string) string {
	return kc.Classify(&ArticleInput{
		Title:   title,
		Summary: summary,
	}).Category
}

// ClassifyWithSource is a convenience function that classifies with source category hints.
// sourceCategories are the pre-assigned categories from the data source (e.g. RSS feed categories).
// Returns the display name of the best category.
func (kc *KeywordClassifier) ClassifyWithSource(title, summary string, sourceCategories []string) string {
	return kc.Classify(&ArticleInput{
		Title:    title,
		Summary:  summary,
		Category: sourceCategories,
	}).Category
}

// GetCategoryKey returns the internal category key for a display name.
// Useful for storing in the database.
func (kc *KeywordClassifier) GetCategoryKey(displayName string) string {
	for key, name := range CategoryDisplayName {
		if name == displayName {
			return key
		}
	}
	return "general"
}

// toLowerSlice converts a slice of strings to lowercase, deduplicating empty strings.
func toLowerSlice(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	result := make([]string, 0, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(strings.ToLower(s))
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// Ensure KeywordClassifier implements Classifier interface at compile time.
var _ Classifier = (*KeywordClassifier)(nil)

// DebugClassify returns detailed classification info for debugging.
func (kc *KeywordClassifier) DebugClassify(input *ArticleInput) string {
	result := kc.Classify(input)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Title: %s\n", input.Title))
	sb.WriteString(fmt.Sprintf("Summary: %s\n", input.Summary))
	if len(input.Category) > 0 {
		sb.WriteString(fmt.Sprintf("Source Categories: %v\n", input.Category))
	}
	sb.WriteString(fmt.Sprintf("Result: %s\n", result.Category))
	sb.WriteString("Scores:\n")
	for _, cat := range KnownCategories {
		if score, ok := result.Scores[cat]; ok {
			sb.WriteString(fmt.Sprintf("  %s (%s): %.2f\n", CategoryDisplayName[cat], cat, score))
		}
	}
	return sb.String()
}
