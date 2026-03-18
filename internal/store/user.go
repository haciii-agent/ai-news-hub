package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// User 用户数据模型
type User struct {
	ID         int64  `json:"id"`
	Token      string `json:"token"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
}

// UserStore 用户相关存储接口
type UserStore interface {
	GetOrCreateUserByToken(token string) (*User, bool, error)
	UpdateUserLastSeen(userID int64) error
	BookmarkArticle(userID, articleID int64) error
	UnbookmarkArticle(userID, articleID int64) error
	IsBookmarked(userID, articleID int64) (bool, error)
	ListBookmarks(userID int64, filter ArticleFilter) ([]Article, int, error)
	GetBookmarkedIDs(userID int64, articleIDs []int64) (map[int64]bool, error)
	RecordReadHistory(userID, articleID int64) error
	ListReadHistory(userID int64, filter ArticleFilter) ([]Article, int, error)
}

// userStore implements UserStore backed by a *sql.DB (SQLite).
type userStore struct {
	db     *sql.DB
	pStore ProfileStore
}

// NewUserStore wraps an open *sql.DB into a UserStore.
// Profile update is triggered on read history recording.
func NewUserStore(db *sql.DB) UserStore {
	return &userStore{db: db, pStore: NewProfileStore(db)}
}

// NewUserStoreWithProfile creates a UserStore with an external ProfileStore (for testing).
func NewUserStoreWithProfile(db *sql.DB, ps ProfileStore) UserStore {
	return &userStore{db: db, pStore: ps}
}

// GetOrCreateUserByToken returns the user for the given token.
// If the token doesn't exist, a new user is created.
// The boolean `created` indicates whether a new user was created.
func (s *userStore) GetOrCreateUserByToken(token string) (*User, bool, error) {
	// Try to find existing user
	var u User
	err := s.db.QueryRow(
		`SELECT id, token, created_at, last_seen_at FROM users WHERE token = ?`, token,
	).Scan(&u.ID, &u.Token, &u.CreatedAt, &u.LastSeenAt)

	if err == nil {
		// User exists, update last_seen_at
		_ = s.UpdateUserLastSeen(u.ID)
		return &u, false, nil
	}

	if err != sql.ErrNoRows {
		return nil, false, fmt.Errorf("query user by token: %w", err)
	}

	// Create new user
	res, err := s.db.Exec(
		`INSERT INTO users (token, created_at, last_seen_at) VALUES (?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		token,
	)
	if err != nil {
		return nil, false, fmt.Errorf("create user: %w", err)
	}

	id, _ := res.LastInsertId()
	return &User{
		ID:         id,
		Token:      token,
		CreatedAt:  "", // will be populated on next query
		LastSeenAt: "",
	}, true, nil
}

// UpdateUserLastSeen updates the last_seen_at timestamp for a user.
func (s *userStore) UpdateUserLastSeen(userID int64) error {
	_, err := s.db.Exec(`UPDATE users SET last_seen_at = CURRENT_TIMESTAMP WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("update user last_seen: %w", err)
	}
	return nil
}

// BookmarkArticle adds a bookmark for a user/article pair.
func (s *userStore) BookmarkArticle(userID, articleID int64) error {
	_, err := s.db.Exec(
		`INSERT INTO bookmarks (user_id, article_id, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id, article_id) DO NOTHING`,
		userID, articleID,
	)
	if err != nil {
		return fmt.Errorf("bookmark article %d for user %d: %w", articleID, userID, err)
	}
	return nil
}

// UnbookmarkArticle removes a bookmark for a user/article pair.
func (s *userStore) UnbookmarkArticle(userID, articleID int64) error {
	_, err := s.db.Exec(
		`DELETE FROM bookmarks WHERE user_id = ? AND article_id = ?`,
		userID, articleID,
	)
	if err != nil {
		return fmt.Errorf("unbookmark article %d for user %d: %w", articleID, userID, err)
	}
	return nil
}

// IsBookmarked checks if a user has bookmarked a specific article.
func (s *userStore) IsBookmarked(userID, articleID int64) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM bookmarks WHERE user_id = ? AND article_id = ?`,
		userID, articleID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check bookmark: %w", err)
	}
	return count > 0, nil
}

// ListBookmarks returns a paginated list of bookmarked articles for a user.
func (s *userStore) ListBookmarks(userID int64, filter ArticleFilter) ([]Article, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}
	if filter.PerPage > 100 {
		filter.PerPage = 100
	}

	// Total count
	var total int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM bookmarks WHERE user_id = ?`, userID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count bookmarks: %w", err)
	}

	// Paginated results — join bookmarks with articles, order by bookmark created_at desc
	offset := (filter.Page - 1) * filter.PerPage
	query := `SELECT a.id, a.title, a.url, a.source, COALESCE(a.source_url,''), a.category,
	          COALESCE(a.summary,''), COALESCE(a.content_html,''), COALESCE(a.image_url,''),
	          a.published_at, a.collected_at, a.language
	          FROM bookmarks b
	          JOIN articles a ON b.article_id = a.id
	          WHERE b.user_id = ?
	          ORDER BY b.created_at DESC
	          LIMIT ? OFFSET ?`

	rows, err := s.db.Query(query, userID, filter.PerPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list bookmarks: %w", err)
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(
			&a.ID, &a.Title, &a.URL, &a.Source, &a.SourceURL,
			&a.Category, &a.Summary, &a.ContentHTML, &a.ImageURL,
			&a.PublishedAt, &a.CollectedAt, &a.Language,
		); err != nil {
			return nil, 0, fmt.Errorf("scan bookmark article: %w", err)
		}
		articles = append(articles, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate bookmarks: %w", err)
	}

	return articles, total, nil
}

// GetBookmarkedIDs checks bookmark status for multiple article IDs at once.
// Returns a map of articleID → bool.
func (s *userStore) GetBookmarkedIDs(userID int64, articleIDs []int64) (map[int64]bool, error) {
	result := make(map[int64]bool, len(articleIDs))
	if len(articleIDs) == 0 {
		return result, nil
	}

	// Build placeholders
	placeholders := make([]string, len(articleIDs))
	args := make([]interface{}, len(articleIDs))
	for i, id := range articleIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT article_id FROM bookmarks WHERE user_id = ? AND article_id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	args = append([]interface{}{userID}, args...)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get bookmarked ids: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan bookmarked id: %w", err)
		}
		result[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookmarked ids: %w", err)
	}

	return result, nil
}

// RecordReadHistory records a read history entry (idempotent — updates read_at on conflict).
// After recording, it asynchronously triggers a user profile update.
func (s *userStore) RecordReadHistory(userID, articleID int64) error {
	_, err := s.db.Exec(
		`INSERT INTO read_history (user_id, article_id, read_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id, article_id) DO UPDATE SET read_at = CURRENT_TIMESTAMP`,
		userID, articleID,
	)
	if err != nil {
		return fmt.Errorf("record read history %d for user %d: %w", articleID, userID, err)
	}

	// Trigger profile update asynchronously (fire and forget)
	go s.triggerProfileUpdate(userID, articleID, false)

	return nil
}

// BookmarkArticleWithProfileUpdate bookmarks an article and triggers profile update with higher weight.
func (s *userStore) BookmarkArticleWithProfileUpdate(userID, articleID int64) error {
	err := s.BookmarkArticle(userID, articleID)
	if err != nil {
		return err
	}

	// Trigger profile update asynchronously with bookmark weight
	go s.triggerProfileUpdate(userID, articleID, true)

	return nil
}

// triggerProfileUpdate updates user profile based on article content.
func (s *userStore) triggerProfileUpdate(userID, articleID int64, isBookmark bool) {
	// Fetch article to get category and title
	var category, title string
	err := s.db.QueryRow(
		`SELECT COALESCE(category, ''), COALESCE(title, '') FROM articles WHERE id = ?`,
		articleID,
	).Scan(&category, &title)
	if err != nil {
		return
	}

	var tags map[string]float64
	if isBookmark {
		tags = ExtractTagsFromArticleForBookmark(category, title)
	} else {
		tags = ExtractTagsFromArticle(category, title)
	}

	if len(tags) == 0 {
		return
	}

	if err := s.pStore.UpdateProfileInterests(userID, tags); err != nil {
		// Log but don't fail the original operation
		fmt.Printf("[profile] async update failed for user %d: %v\n", userID, err)
	}
}

// ListReadHistory returns a paginated list of recently read articles for a user.
func (s *userStore) ListReadHistory(userID int64, filter ArticleFilter) ([]Article, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}
	if filter.PerPage > 100 {
		filter.PerPage = 100
	}

	// Total count
	var total int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM read_history WHERE user_id = ?`, userID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count read history: %w", err)
	}

	// Paginated results — join read_history with articles, order by read_at desc
	offset := (filter.Page - 1) * filter.PerPage
	query := `SELECT a.id, a.title, a.url, a.source, COALESCE(a.source_url,''), a.category,
	          COALESCE(a.summary,''), COALESCE(a.content_html,''), COALESCE(a.image_url,''),
	          a.published_at, a.collected_at, a.language
	          FROM read_history rh
	          JOIN articles a ON rh.article_id = a.id
	          WHERE rh.user_id = ?
	          ORDER BY rh.read_at DESC
	          LIMIT ? OFFSET ?`

	rows, err := s.db.Query(query, userID, filter.PerPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list read history: %w", err)
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(
			&a.ID, &a.Title, &a.URL, &a.Source, &a.SourceURL,
			&a.Category, &a.Summary, &a.ContentHTML, &a.ImageURL,
			&a.PublishedAt, &a.CollectedAt, &a.Language,
		); err != nil {
			return nil, 0, fmt.Errorf("scan history article: %w", err)
		}
		articles = append(articles, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate read history: %w", err)
	}

	return articles, total, nil
}

// ParseIDs parses a comma-separated string of IDs into a slice of int64.
func ParseIDs(idsStr string) ([]int64, error) {
	parts := strings.Split(idsStr, ",")
	var ids []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid id %q: %w", p, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
