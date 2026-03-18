package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// Article 文章数据模型
type Article struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Source      string  `json:"source"`
	SourceURL   string  `json:"source_url,omitempty"`
	Category    string  `json:"category"`
	Summary     string  `json:"summary,omitempty"`
	ContentHTML string  `json:"content_html,omitempty"`
	ImageURL    string  `json:"image_url,omitempty"`
	PublishedAt *string `json:"published_at,omitempty"`
	CollectedAt string  `json:"collected_at"`
	Language    string  `json:"language"`
}

// CollectRun 采集运行记录
type CollectRun struct {
	ID             int64   `json:"id"`
	StartedAt      string  `json:"started_at"`
	FinishedAt     *string `json:"finished_at,omitempty"`
	Status         string  `json:"status"`
	TotalCollected int     `json:"total_collected"`
	TotalNew       int     `json:"total_new"`
	ErrorsCount    int     `json:"errors_count"`
	Errors         *string `json:"errors,omitempty"`
}

// ArticleStore 文章存储接口（T005/T007 扩展）
type ArticleStore interface {
	InsertArticle(article *Article) error
	BatchInsertArticles(articles []Article) (inserted int, skipped int, err error)
	QueryArticles(filter ArticleFilter) ([]Article, int, error)
	SearchArticles(query string, filter ArticleFilter) ([]Article, int, map[int64]string, error)
	GetArticleByID(id int64) (*Article, error)
	GetCategoryStats() ([]CategoryStat, error)
	GetLanguageCounts() (map[string]int, error)
	DeleteArticlesBefore(before string) (int64, error)
	InsertCollectRun(run *CollectRun) (int64, error)
	GetLatestCollectRun() (*CollectRun, error)
}

// ArticleFilter 文章查询过滤器
type ArticleFilter struct {
	Category string
	Page     int
	PerPage  int
	Sort     string
	Search   string
	Language string
}

// CategoryStat 分类统计
type CategoryStat struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// --- SQLite 实现 ---

// articleStore implements ArticleStore backed by a *sql.DB (SQLite).
type articleStore struct {
	db *sql.DB
}

// NewArticleStore wraps an open *sql.DB into an ArticleStore.
func NewArticleStore(db *sql.DB) ArticleStore {
	return &articleStore{db: db}
}

// InsertArticle inserts a single article. Duplicate URLs are silently skipped
// (returns nil on conflict so the caller treats it as success).
func (s *articleStore) InsertArticle(article *Article) error {
	_, err := s.db.Exec(`
		INSERT INTO articles (title, url, source, source_url, category, summary, content_html, image_url, published_at, language)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url) DO NOTHING
	`,
		article.Title,
		article.URL,
		article.Source,
		article.SourceURL,
		article.Category,
		article.Summary,
		article.ContentHTML,
		article.ImageURL,
		article.PublishedAt,
		article.Language,
	)
	return err
}

// BatchInsertArticles inserts multiple articles in a single transaction.
// Returns (inserted, skipped) counts.
func (s *articleStore) BatchInsertArticles(articles []Article) (inserted int, skipped int, err error) {
	if len(articles) == 0 {
		return 0, 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("begin batch insert: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO articles (title, url, source, source_url, category, summary, content_html, image_url, published_at, language)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url) DO NOTHING
	`)
	if err != nil {
		return 0, 0, fmt.Errorf("prepare batch insert: %w", err)
	}
	defer stmt.Close()

	for i := range articles {
		a := &articles[i]
		res, err := stmt.Exec(
			a.Title,
			a.URL,
			a.Source,
			a.SourceURL,
			a.Category,
			a.Summary,
			a.ContentHTML,
			a.ImageURL,
			a.PublishedAt,
			a.Language,
		)
		if err != nil {
			return inserted, skipped, fmt.Errorf("batch insert row %d: %w", i, err)
		}
		ra, _ := res.RowsAffected()
		if ra > 0 {
			inserted++
		} else {
			skipped++
		}
	}

	if err := tx.Commit(); err != nil {
		return inserted, skipped, fmt.Errorf("commit batch insert: %w", err)
	}

	return inserted, skipped, nil
}

// QueryArticles returns a paginated, filtered list of articles along with total count.
func (s *articleStore) QueryArticles(filter ArticleFilter) ([]Article, int, error) {
	// Sanitize defaults.
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}
	if filter.PerPage > 100 {
		filter.PerPage = 100
	}

	var conditions []string
	var args []interface{}

	if filter.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, filter.Category)
	}
	if filter.Language != "" {
		conditions = append(conditions, "language = ?")
		args = append(args, filter.Language)
	}
	if filter.Search != "" {
		conditions = append(conditions, "(title LIKE ? OR summary LIKE ?)")
		pat := "%" + filter.Search + "%"
		args = append(args, pat, pat)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Determine sort order (whitelist to prevent injection).
	orderBy := "collected_at DESC"
	switch strings.ToLower(filter.Sort) {
	case "time":
		orderBy = "collected_at DESC"
	case "source":
		orderBy = "source ASC"
	case "published":
		orderBy = "published_at DESC"
	case "title":
		orderBy = "title ASC"
	}

	// Total count.
	var total int
	countSQL := "SELECT COUNT(*) FROM articles " + where
	if err := s.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count articles: %w", err)
	}

	// Paginated results.
	offset := (filter.Page - 1) * filter.PerPage
	querySQL := fmt.Sprintf("SELECT id, title, url, source, COALESCE(source_url,''), category, COALESCE(summary,''), COALESCE(content_html,''), COALESCE(image_url,''), published_at, collected_at, language FROM articles %s ORDER BY %s LIMIT ? OFFSET ?", where, orderBy)

	qArgs := append(args, filter.PerPage, offset)
	rows, err := s.db.Query(querySQL, qArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query articles: %w", err)
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(
			&a.ID,
			&a.Title,
			&a.URL,
			&a.Source,
			&a.SourceURL,
			&a.Category,
			&a.Summary,
			&a.ContentHTML,
			&a.ImageURL,
			&a.PublishedAt,
			&a.CollectedAt,
			&a.Language,
		); err != nil {
			return nil, 0, fmt.Errorf("scan article: %w", err)
		}
		articles = append(articles, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate articles: %w", err)
	}

	return articles, total, nil
}

// GetArticleByID returns a single article by its primary key, or nil if not found.
func (s *articleStore) GetArticleByID(id int64) (*Article, error) {
	var a Article
	err := s.db.QueryRow(`
		SELECT id, title, url, source, COALESCE(source_url,''), category, COALESCE(summary,''), COALESCE(content_html,''), COALESCE(image_url,''), published_at, collected_at, language
		FROM articles WHERE id = ?
	`, id).Scan(
		&a.ID,
		&a.Title,
		&a.URL,
		&a.Source,
		&a.SourceURL,
		&a.Category,
		&a.Summary,
		&a.ContentHTML,
		&a.ImageURL,
		&a.PublishedAt,
		&a.CollectedAt,
		&a.Language,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get article by id %d: %w", id, err)
	}
	return &a, nil
}

// GetCategoryStats returns article counts grouped by category.
func (s *articleStore) GetCategoryStats() ([]CategoryStat, error) {
	rows, err := s.db.Query(`
		SELECT category, COUNT(*) as count FROM articles GROUP BY category ORDER BY count DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("category stats: %w", err)
	}
	defer rows.Close()

	var stats []CategoryStat
	for rows.Next() {
		var cs CategoryStat
		if err := rows.Scan(&cs.Category, &cs.Count); err != nil {
			return nil, fmt.Errorf("scan category stat: %w", err)
		}
		stats = append(stats, cs)
	}
	return stats, rows.Err()
}

// InsertCollectRun creates a new collect_runs record and returns its ID.
func (s *articleStore) InsertCollectRun(run *CollectRun) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO collect_runs (started_at, status, total_collected, total_new, errors_count, errors)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		run.StartedAt,
		run.Status,
		run.TotalCollected,
		run.TotalNew,
		run.ErrorsCount,
		run.Errors,
	)
	if err != nil {
		return 0, fmt.Errorf("insert collect run: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// GetLanguageCounts returns article counts grouped by language.
func (s *articleStore) GetLanguageCounts() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT language, COUNT(*) FROM articles GROUP BY language`)
	if err != nil {
		return nil, fmt.Errorf("language counts: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var lang string
		var count int
		if err := rows.Scan(&lang, &count); err != nil {
			return nil, fmt.Errorf("scan language count: %w", err)
		}
		counts[lang] = count
	}
	return counts, rows.Err()
}

// SearchArticles performs full-text search using FTS5 on title + summary.
// Returns matched articles, total count, a map of article ID → highlighted snippet, and error.
// Results are ordered by BM25 relevance. Supports category/language filters and pagination.
// Empty query falls back to returning all articles (same as QueryArticles).
func (s *articleStore) SearchArticles(query string, filter ArticleFilter) ([]Article, int, map[int64]string, error) {
	// Sanitize defaults
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}
	if filter.PerPage > 100 {
		filter.PerPage = 100
	}

	// Empty query → fall back to normal list
	if strings.TrimSpace(query) == "" {
		articles, total, err := s.QueryArticles(filter)
		return articles, total, nil, err
	}

	// Sanitize FTS query: escape special characters
	ftsQuery := sanitizeFTSQuery(query)

	// Build WHERE conditions (JOIN on articles table for category/language filter)
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "articles_fts MATCH ?")
	args = append(args, ftsQuery)

	if filter.Category != "" {
		conditions = append(conditions, "articles.category = ?")
		args = append(args, filter.Category)
	}
	if filter.Language != "" {
		conditions = append(conditions, "articles.language = ?")
		args = append(args, filter.Language)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	// Total count
	var total int
	countSQL := "SELECT COUNT(*) FROM articles_fts JOIN articles ON articles_fts.rowid = articles.id " + where
	if err := s.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, nil, fmt.Errorf("count search results: %w", err)
	}

	// Paginated results with BM25 ranking and snippet extraction
	offset := (filter.Page - 1) * filter.PerPage
	querySQL := fmt.Sprintf(
		`SELECT articles.id, articles.title, articles.url, articles.source,
		        COALESCE(articles.source_url,''), articles.category,
		        COALESCE(articles.summary,''), COALESCE(articles.content_html,''),
		        COALESCE(articles.image_url,''), articles.published_at,
		        articles.collected_at, articles.language,
		        snippet(articles_fts, 0, '<mark>', '</mark>', '...', 32)
		 FROM articles_fts JOIN articles ON articles_fts.rowid = articles.id
		 %s
		 ORDER BY bm25(articles_fts)
		 LIMIT ? OFFSET ?`, where)

	qArgs := append(args, filter.PerPage, offset)
	rows, err := s.db.Query(querySQL, qArgs...)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("search articles: %w", err)
	}
	defer rows.Close()

	snippets := make(map[int64]string)
	var articles []Article
	for rows.Next() {
		var a Article
		var snippet string
		if err := rows.Scan(
			&a.ID, &a.Title, &a.URL, &a.Source,
			&a.SourceURL, &a.Category, &a.Summary,
			&a.ContentHTML, &a.ImageURL, &a.PublishedAt,
			&a.CollectedAt, &a.Language,
			&snippet,
		); err != nil {
			return nil, 0, nil, fmt.Errorf("scan search result: %w", err)
		}
		// Strip HTML tags from snippet (from summary which may contain tags)
		snippets[a.ID] = stripHTML(snippet)
		articles = append(articles, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, nil, fmt.Errorf("iterate search results: %w", err)
	}

	return articles, total, snippets, nil
}

// sanitizeFTSQuery escapes FTS5 special characters to prevent query syntax errors.
// Wraps each token in double quotes for safe phrase matching.
func sanitizeFTSQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return query
	}
	// Escape double quotes in the query
	query = strings.ReplaceAll(query, `"`, `""`)
	// Split into tokens and join with OR for flexible matching
	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		return query
	}
	var parts []string
	for _, t := range tokens {
		if t != "" {
			parts = append(parts, `"`+t+`"`)
		}
	}
	return strings.Join(parts, " OR ")
}

// stripHTML removes all HTML tags from a string.
func stripHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// DeleteArticlesBefore deletes articles with collected_at before the given date string (YYYY-MM-DD).
func (s *articleStore) DeleteArticlesBefore(before string) (int64, error) {
	result, err := s.db.Exec(`DELETE FROM articles WHERE date(collected_at) < date(?)`, before)
	if err != nil {
		return 0, fmt.Errorf("delete articles before %s: %w", before, err)
	}
	return result.RowsAffected()
}

// GetLatestCollectRun returns the most recent collect_runs record, or nil if none exist.
func (s *articleStore) GetLatestCollectRun() (*CollectRun, error) {
	var r CollectRun
	err := s.db.QueryRow(`
		SELECT id, started_at, finished_at, status, total_collected, total_new, errors_count, errors
		FROM collect_runs ORDER BY id DESC LIMIT 1
	`).Scan(
		&r.ID,
		&r.StartedAt,
		&r.FinishedAt,
		&r.Status,
		&r.TotalCollected,
		&r.TotalNew,
		&r.ErrorsCount,
		&r.Errors,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest collect run: %w", err)
	}
	return &r, nil
}
