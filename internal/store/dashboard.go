package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// DashboardStats holds core system statistics for the dashboard.
type DashboardStats struct {
	TotalArticles   int             `json:"total_articles"`
	TodayNew        int             `json:"today_new"`
	TotalSources    int             `json:"total_sources"`
	ActiveSources   int             `json:"active_sources"`
	FailedSources   int             `json:"failed_sources"`
	TotalCategories int             `json:"total_categories"`
	TotalCollectRuns int            `json:"total_collect_runs"`
	LastCollectTime string          `json:"last_collect_time,omitempty"`
	LastCollectStatus string        `json:"last_collect_status,omitempty"`
	LatestCollect   *CollectRunSummary `json:"latest_collect,omitempty"`
}

// CollectRunSummary is a simplified collect run for dashboard display.
type CollectRunSummary struct {
	TotalCollected  int    `json:"total_collected"`
	TotalNew        int    `json:"total_new"`
	ErrorsCount     int    `json:"errors_count"`
	DurationSeconds int64  `json:"duration_seconds"`
}

// TrendDataPoint represents a single day's article count.
type TrendDataPoint struct {
	Date        string `json:"date"`
	NewArticles int    `json:"new_articles"`
	TotalArticles int  `json:"total_articles"`
}

// TrendResponse holds article trend data.
type TrendResponse struct {
	Days int             `json:"days"`
	Data []TrendDataPoint `json:"data"`
}

// CategoryDistribution holds category distribution data.
type CategoryDistribution struct {
	Name       string  `json:"name"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// CategoryResponse holds category distribution response.
type CategoryResponse struct {
	Categories []CategoryDistribution `json:"categories"`
}

// SourceHealth holds data source health information.
type SourceHealth struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	ArticleCount int     `json:"article_count"`
	Status       string  `json:"status"` // healthy, degraded, failing, never
	LastSuccess  string  `json:"last_success,omitempty"`
	SuccessRate  float64 `json:"success_rate"`
}

// SourceResponse holds source health response.
type SourceResponse struct {
	Sources []SourceHealth `json:"sources"`
}

// RecentArticleSummary is a simplified article for dashboard display.
type RecentArticleSummary struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Source      string  `json:"source"`
	Category    string  `json:"category"`
	PublishedAt *string `json:"published_at,omitempty"`
}

// CollectHistoryResponse holds collect run history.
type CollectHistoryResponse struct {
	Runs []CollectRun `json:"runs"`
}

// SourceErrorJSON represents a parsed source error from collect_runs.errors.
type SourceErrorJSON struct {
	Source string `json:"source"`
	Error  string `json:"error"`
	Type   string `json:"type,omitempty"`
}

// --- Dashboard Store ---

// DashboardStore provides dashboard-related query methods.
type DashboardStore struct {
	db *sql.DB
}

// NewDashboardStore creates a new DashboardStore.
func NewDashboardStore(db *sql.DB) *DashboardStore {
	return &DashboardStore{db: db}
}

// GetStats returns core system statistics.
func (ds *DashboardStore) GetStats() (*DashboardStats, error) {
	stats := &DashboardStats{}

	// Total articles
	if err := ds.db.QueryRow("SELECT COUNT(*) FROM articles").Scan(&stats.TotalArticles); err != nil {
		return nil, fmt.Errorf("count total articles: %w", err)
	}

	// Today new articles
	if err := ds.db.QueryRow(
		"SELECT COUNT(*) FROM articles WHERE date(collected_at) = date('now', 'localtime')",
	).Scan(&stats.TodayNew); err != nil {
		return nil, fmt.Errorf("count today articles: %w", err)
	}

	// Total distinct sources
	if err := ds.db.QueryRow(
		"SELECT COUNT(DISTINCT source) FROM articles",
	).Scan(&stats.TotalSources); err != nil {
		return nil, fmt.Errorf("count sources: %w", err)
	}

	// Total distinct categories
	if err := ds.db.QueryRow(
		"SELECT COUNT(DISTINCT category) FROM articles",
	).Scan(&stats.TotalCategories); err != nil {
		return nil, fmt.Errorf("count categories: %w", err)
	}

	// Total collect runs
	if err := ds.db.QueryRow(
		"SELECT COUNT(*) FROM collect_runs",
	).Scan(&stats.TotalCollectRuns); err != nil {
		return nil, fmt.Errorf("count collect runs: %w", err)
	}

	// Latest collect run
	latestRun, err := ds.getLatestCollectRun()
	if err != nil {
		return nil, err
	}
	if latestRun != nil {
		stats.LastCollectTime = latestRun.StartedAt
		stats.LastCollectStatus = latestRun.Status

		duration := int64(0)
		if latestRun.FinishedAt != nil {
			started, _ := time.Parse(time.RFC3339, latestRun.StartedAt)
			finished, _ := time.Parse(time.RFC3339, *latestRun.FinishedAt)
			if !started.IsZero() && !finished.IsZero() {
				duration = int64(finished.Sub(started).Seconds())
			}
		}
		stats.LatestCollect = &CollectRunSummary{
			TotalCollected:  latestRun.TotalCollected,
			TotalNew:        latestRun.TotalNew,
			ErrorsCount:     latestRun.ErrorsCount,
			DurationSeconds: duration,
		}
	}

	// Source health: active vs failed
	sourceHealthResp, err := ds.GetSources()
	if err == nil && len(sourceHealthResp.Sources) > 0 {
		for _, s := range sourceHealthResp.Sources {
			if s.Status == "healthy" || s.Status == "degraded" {
				stats.ActiveSources++
			} else {
				stats.FailedSources++
			}
		}
	} else if stats.TotalSources > 0 {
		// No collect runs yet, but we have sources in articles — assume all active
		stats.ActiveSources = stats.TotalSources
	}

	return stats, nil
}

// getLatestCollectRun returns the most recent collect run.
func (ds *DashboardStore) getLatestCollectRun() (*CollectRun, error) {
	var r CollectRun
	err := ds.db.QueryRow(`
		SELECT id, started_at, finished_at, status, total_collected, total_new, errors_count, errors
		FROM collect_runs ORDER BY id DESC LIMIT 1
	`).Scan(
		&r.ID, &r.StartedAt, &r.FinishedAt, &r.Status,
		&r.TotalCollected, &r.TotalNew, &r.ErrorsCount, &r.Errors,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest collect run: %w", err)
	}
	return &r, nil
}

// GetTrend returns article growth trend for the past N days.
func (ds *DashboardStore) GetTrend(days int) (*TrendResponse, error) {
	if days < 1 {
		days = 7
	}
	if days > 30 {
		days = 30
	}

	// Get total articles before the date range (for cumulative count)
	var baseTotal int
	if err := ds.db.QueryRow(
		"SELECT COUNT(*) FROM articles WHERE date(collected_at) < date('now', 'localtime', ?)",
		fmt.Sprintf("-%d days", days),
	).Scan(&baseTotal); err != nil {
		return nil, fmt.Errorf("count base total: %w", err)
	}

	// Get daily counts for the past N days
	query := `
		SELECT date(collected_at, 'localtime') as d, COUNT(*) as cnt
		FROM articles
		WHERE date(collected_at, 'localtime') >= date('now', 'localtime', ?)
		GROUP BY d
		ORDER BY d ASC
	`
	rows, err := ds.db.Query(query, fmt.Sprintf("-%d days", days))
	if err != nil {
		return nil, fmt.Errorf("query trend: %w", err)
	}
	defer rows.Close()

	// Build a map of date -> count
	dateCounts := make(map[string]int)
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			return nil, fmt.Errorf("scan trend: %w", err)
		}
		dateCounts[date] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trend: %w", err)
	}

	// Fill in all days (including days with 0 articles)
	now := time.Now()
	data := make([]TrendDataPoint, 0, days)
	cumulative := baseTotal

	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		cnt := dateCounts[date]
		cumulative += cnt
		data = append(data, TrendDataPoint{
			Date:         date,
			NewArticles:  cnt,
			TotalArticles: cumulative,
		})
	}

	return &TrendResponse{
		Days: days,
		Data: data,
	}, nil
}

// GetCategoryDistribution returns article count distribution by category.
func (ds *DashboardStore) GetCategoryDistribution() (*CategoryResponse, error) {
	rows, err := ds.db.Query(`
		SELECT category, COUNT(*) as count FROM articles GROUP BY category ORDER BY count DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("category distribution: %w", err)
	}
	defer rows.Close()

	var categories []CategoryDistribution
	var totalCount int

	for rows.Next() {
		var cd CategoryDistribution
		if err := rows.Scan(&cd.Name, &cd.Count); err != nil {
			return nil, fmt.Errorf("scan category distribution: %w", err)
		}
		totalCount += cd.Count
		categories = append(categories, cd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate category distribution: %w", err)
	}

	// Calculate percentages
	if totalCount > 0 {
		for i := range categories {
			categories[i].Percentage = float64(categories[i].Count) / float64(totalCount) * 100
			// Round to 1 decimal
			categories[i].Percentage = float64(int(categories[i].Percentage*10)) / 10
		}
	}

	return &CategoryResponse{Categories: categories}, nil
}

// GetSources returns data source health information.
func (ds *DashboardStore) GetSources() (*SourceResponse, error) {
	// Get article counts per source
	rows, err := ds.db.Query(`
		SELECT source, COUNT(*) as count FROM articles GROUP BY source ORDER BY count DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("source article counts: %w", err)
	}
	defer rows.Close()

	type sourceInfo struct {
		Name         string
		ArticleCount int
	}

	var sources []sourceInfo
	for rows.Next() {
		var si sourceInfo
		if err := rows.Scan(&si.Name, &si.ArticleCount); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, si)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sources: %w", err)
	}

	if len(sources) == 0 {
		return &SourceResponse{Sources: []SourceHealth{}}, nil
	}

	// Parse recent collect_runs to determine source health
	// Look at last 10 runs
	type sourceHealthData struct {
		totalRuns   int
		successRuns int
		lastSuccess *string
	}

	sourceHealthMap := make(map[string]*sourceHealthData)
	for _, s := range sources {
		sourceHealthMap[s.Name] = &sourceHealthData{}
	}

	collectRows, err := ds.db.Query(`
		SELECT started_at, status, errors FROM collect_runs ORDER BY id DESC LIMIT 10
	`)
	if err != nil {
		// If no collect runs, all sources are "never"
		healths := make([]SourceHealth, 0, len(sources))
		for _, s := range sources {
			healths = append(healths, SourceHealth{
				Name:         s.Name,
				Type:         "rss",
				ArticleCount: s.ArticleCount,
				Status:       "never",
				SuccessRate:  0,
			})
		}
		return &SourceResponse{Sources: healths}, nil
	}
	defer collectRows.Close()

	now := time.Now()
	for collectRows.Next() {
		var startedAt, status string
		var errorsJSON *string
		if err := collectRows.Scan(&startedAt, &status, &errorsJSON); err != nil {
			continue
		}

		// Parse error sources
		errorSources := make(map[string]bool)
		if errorsJSON != nil && *errorsJSON != "" && *errorsJSON != "null" {
			var errs []SourceErrorJSON
			if json.Unmarshal([]byte(*errorsJSON), &errs) == nil {
				for _, e := range errs {
					errorSources[e.Source] = true
				}
			}
		}

		// For each source, determine if this run was a success for it
		for name, hd := range sourceHealthMap {
			hd.totalRuns++
			if !errorSources[name] {
				hd.successRuns++
				if hd.lastSuccess == nil || startedAt > *hd.lastSuccess {
					hd.lastSuccess = &startedAt
				}
			}
		}
	}

	// Build response
	healths := make([]SourceHealth, 0, len(sources))
	for _, s := range sources {
		hd := sourceHealthMap[s.Name]
		sh := SourceHealth{
			Name:         s.Name,
			Type:         "rss",
			ArticleCount: s.ArticleCount,
		}

		if hd.totalRuns > 0 {
			sh.SuccessRate = float64(int(float64(hd.successRuns)/float64(hd.totalRuns)*1000)) / 1000
		}

		if hd.lastSuccess != nil {
			sh.LastSuccess = *hd.lastSuccess
			lastT, _ := time.Parse(time.RFC3339, *hd.lastSuccess)
			hoursSince := now.Sub(lastT).Hours()

			if hoursSince <= 24 {
				sh.Status = "healthy"
			} else if hoursSince <= 48 {
				sh.Status = "degraded"
			} else {
				sh.Status = "failing"
			}
		} else {
			sh.Status = "never"
		}

		healths = append(healths, sh)
	}

	return &SourceResponse{Sources: healths}, nil
}

// GetRecentArticles returns the most recent articles in summary form.
func (ds *DashboardStore) GetRecentArticles(limit int) ([]RecentArticleSummary, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	rows, err := ds.db.Query(`
		SELECT id, title, source, category, published_at
		FROM articles ORDER BY collected_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent articles: %w", err)
	}
	defer rows.Close()

	var articles []RecentArticleSummary
	for rows.Next() {
		var a RecentArticleSummary
		if err := rows.Scan(&a.ID, &a.Title, &a.Source, &a.Category, &a.PublishedAt); err != nil {
			return nil, fmt.Errorf("scan recent article: %w", err)
		}
		articles = append(articles, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent articles: %w", err)
	}

	return articles, nil
}

// GetCollectHistory returns recent collect run records.
func (ds *DashboardStore) GetCollectHistory(limit int) (*CollectHistoryResponse, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	rows, err := ds.db.Query(`
		SELECT id, started_at, finished_at, status, total_collected, total_new, errors_count, errors
		FROM collect_runs ORDER BY id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("collect history: %w", err)
	}
	defer rows.Close()

	var runs []CollectRun
	for rows.Next() {
		var r CollectRun
		if err := rows.Scan(
			&r.ID, &r.StartedAt, &r.FinishedAt, &r.Status,
			&r.TotalCollected, &r.TotalNew, &r.ErrorsCount, &r.Errors,
		); err != nil {
			return nil, fmt.Errorf("scan collect run: %w", err)
		}
		runs = append(runs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collect runs: %w", err)
	}

	if runs == nil {
		runs = []CollectRun{}
	}

	return &CollectHistoryResponse{Runs: runs}, nil
}
