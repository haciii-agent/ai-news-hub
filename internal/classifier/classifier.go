// Package classifier provides keyword-based article classification.
package classifier

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ArticleInput represents the input data for classification.
type ArticleInput struct {
	Title     string   // Article title
	Summary   string   // Article summary/content
	Category  []string // Pre-assigned categories from the data source (e.g. RSS source categories)
	Source    string   // Source name
	Language  string   // Article language (en/zh)
}

// ClassifyResult represents the output of classification.
type ClassifyResult struct {
	Category string  // Final classified category name
	Scores   Scores  // All category scores for debugging/transparency
}

// Scores maps category name to its calculated score.
type Scores map[string]float64

// Classifier is the interface for article classifiers.
type Classifier interface {
	// Classify determines the best category for an article.
	Classify(input *ArticleInput) *ClassifyResult

	// Reload reloads classification rules from disk (hot update).
	Reload() error

	// Categories returns the list of all available category names.
	Categories() []string
}

// CategoryRules defines the keyword rules for a single category.
type CategoryRules struct {
	Name          string   `yaml:"name"`
	Keywords      []string `yaml:"keywords"`
	BoostKeywords []string `yaml:"boost_keywords"`
}

// RulesConfig represents the full rules.yaml structure.
type RulesConfig struct {
	SourceCategoryBoost float64                   `yaml:"source_category_boost"`
	Categories          map[string]*CategoryRules `yaml:"categories"`
}

// FallbackCategory is the default category for unclassified articles.
const FallbackCategory = "综合资讯"

// KnownCategories is the ordered list of all 8 categories.
var KnownCategories = []string{
	"ai_ml",
	"tech_frontier",
	"business",
	"open_source",
	"research",
	"policy",
	"product",
	"general",
}

// CategoryDisplayName maps internal keys to display names.
var CategoryDisplayName = map[string]string{
	"ai_ml":          "AI/ML",
	"tech_frontier":  "科技前沿",
	"business":       "商业动态",
	"open_source":    "开源生态",
	"research":       "学术研究",
	"policy":         "政策监管",
	"product":        "产品发布",
	"general":        "综合资讯",
}

// SourceCategoryMapping maps common source category strings to internal category keys.
// Used to convert RSS source categories into boost signals.
var SourceCategoryMapping = map[string]string{
	"ai/ml":      "ai_ml",
	"人工智能":    "ai_ml",
	"科技前沿":    "tech_frontier",
	"商业动态":    "business",
	"开源生态":    "open_source",
	"学术研究":    "research",
	"政策监管":    "policy",
	"产品发布":    "product",
}

// Manager manages classifier instances and provides thread-safe access.
type Manager struct {
	classifier Classifier
	mu         sync.RWMutex
	rulesPath  string
	watchInterval time.Duration
	stopCh     chan struct{}
}

// NewManager creates a new classifier manager.
func NewManager(rulesPath string) (*Manager, error) {
	m := &Manager{
		rulesPath:     rulesPath,
		watchInterval: 30 * time.Second,
		stopCh:        make(chan struct{}),
	}

	c, err := NewKeywordClassifier(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("create keyword classifier: %w", err)
	}
	m.classifier = c

	// Start background file watcher for hot reload
	go m.watchRules()

	return m, nil
}

// Classify classifies an article using the current classifier.
func (m *Manager) Classify(input *ArticleInput) *ClassifyResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.classifier.Classify(input)
}

// Reload forces a reload of classification rules.
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.classifier.Reload(); err != nil {
		return fmt.Errorf("reload classifier: %w", err)
	}
	log.Printf("[classifier] rules reloaded from %s", m.rulesPath)
	return nil
}

// Categories returns the list of all available category names.
func (m *Manager) Categories() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.classifier.Categories()
}

// Stop stops the background file watcher.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// watchRules periodically checks if the rules file has been modified and reloads if so.
func (m *Manager) watchRules() {
	var lastMod time.Time

	// Initialize lastMod
	if info, err := os.Stat(m.rulesPath); err == nil {
		lastMod = info.ModTime()
	}

	ticker := time.NewTicker(m.watchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			info, err := os.Stat(m.rulesPath)
			if err != nil {
				continue
			}
			if info.ModTime().After(lastMod) {
				lastMod = info.ModTime()
				if err := m.Reload(); err != nil {
					log.Printf("[classifier] hot reload failed: %v", err)
				} else {
					log.Printf("[classifier] hot reload successful (file modified)")
				}
			}
		}
	}
}

// ResolveSourceCategory converts a display name to an internal category key.
func ResolveSourceCategory(name string) string {
	// Try direct mapping first
	if key, ok := SourceCategoryMapping[strings.ToLower(name)]; ok {
		return key
	}
	// Try reverse lookup from display names
	for key, display := range CategoryDisplayName {
		if strings.EqualFold(display, name) {
			return key
		}
	}
	return ""
}

// DisplayCategory converts an internal category key to its display name.
func DisplayCategory(key string) string {
	if name, ok := CategoryDisplayName[key]; ok {
		return name
	}
	return key
}

// LoadRulesConfig loads and parses a rules.yaml file.
func LoadRulesConfig(path string) (*RulesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rules %s: %w", path, err)
	}

	cfg := &RulesConfig{
		SourceCategoryBoost: 2.0, // default boost
		Categories:          make(map[string]*CategoryRules),
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse rules %s: %w", path, err)
	}

	// Ensure all known categories exist in the config
	for _, cat := range KnownCategories {
		if _, ok := cfg.Categories[cat]; !ok {
			cfg.Categories[cat] = &CategoryRules{
				Name:          CategoryDisplayName[cat],
				Keywords:      []string{},
				BoostKeywords: []string{},
			}
		}
	}

	return cfg, nil
}
