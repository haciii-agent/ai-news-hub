package store

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestBatchInsertArticles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewArticleStore(db)

	now := "2026-03-17T12:00:00Z"

	// Insert 3 articles
	articles := []Article{
		{Title: "Alpha", URL: "http://a.com/1", Source: "test", Category: "AI", Language: "en", PublishedAt: &now},
		{Title: "Beta", URL: "http://a.com/2", Source: "test", Category: "AI", Language: "en", PublishedAt: &now},
		{Title: "Gamma", URL: "http://a.com/3", Source: "test", Category: "LLM", Language: "zh", PublishedAt: &now},
	}

	inserted, skipped, err := s.BatchInsertArticles(articles)
	if err != nil {
		t.Fatalf("BatchInsertArticles error: %v", err)
	}
	if inserted != 3 {
		t.Errorf("expected 3 inserted, got %d", inserted)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", skipped)
	}

	// Insert duplicate — should be skipped
	articles2 := []Article{
		{Title: "Alpha", URL: "http://a.com/1", Source: "test", Category: "AI", Language: "en"},
		{Title: "Delta", URL: "http://a.com/4", Source: "test", Category: "AI", Language: "en"},
	}

	inserted2, skipped2, err := s.BatchInsertArticles(articles2)
	if err != nil {
		t.Fatalf("BatchInsertArticles (dedup) error: %v", err)
	}
	if inserted2 != 1 {
		t.Errorf("expected 1 new inserted, got %d", inserted2)
	}
	if skipped2 != 1 {
		t.Errorf("expected 1 skipped, got %d", skipped2)
	}

	// Verify total count via QueryArticles
	_, total, err := s.QueryArticles(ArticleFilter{PerPage: 100})
	if err != nil {
		t.Fatalf("QueryArticles error: %v", err)
	}
	if total != 4 {
		t.Errorf("expected 4 total articles, got %d", total)
	}
}

func TestQueryArticlesWithFilter(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewArticleStore(db)

	now := "2026-03-17T12:00:00Z"

	// Seed data
	articles := []Article{
		{Title: "AI News 1", URL: "http://a.com/1", Source: "src1", Category: "AI", Language: "en", PublishedAt: &now},
		{Title: "AI News 2", URL: "http://a.com/2", Source: "src1", Category: "AI", Language: "en", PublishedAt: &now},
		{Title: "LLM Paper", URL: "http://a.com/3", Source: "src2", Category: "LLM", Language: "en", PublishedAt: &now},
		{Title: "中文新闻", URL: "http://a.com/4", Source: "src3", Category: "AI", Language: "zh", PublishedAt: &now},
		{Title: "日本語ニュース", URL: "http://a.com/5", Source: "src4", Category: "AI", Language: "ja", PublishedAt: &now},
	}
	_, _, err := s.BatchInsertArticles(articles)
	if err != nil {
		t.Fatalf("seed data error: %v", err)
	}

	// Filter by category = AI
	results, total, err := s.QueryArticles(ArticleFilter{Category: "AI"})
	if err != nil {
		t.Fatalf("filter by category error: %v", err)
	}
	if total != 4 {
		t.Errorf("category AI: expected 4, got %d", total)
	}
	for _, a := range results {
		if a.Category != "AI" {
			t.Errorf("unexpected category: %s", a.Category)
		}
	}

	// Filter by language = zh
	results, total, err = s.QueryArticles(ArticleFilter{Language: "zh"})
	if err != nil {
		t.Fatalf("filter by language error: %v", err)
	}
	if total != 1 {
		t.Errorf("language zh: expected 1, got %d", total)
	}

	// Pagination: page 1, per_page 2
	results, total, err = s.QueryArticles(ArticleFilter{Page: 1, PerPage: 2})
	if err != nil {
		t.Fatalf("pagination error: %v", err)
	}
	if total != 5 {
		t.Errorf("pagination total: expected 5, got %d", total)
	}
	if len(results) != 2 {
		t.Errorf("pagination page 1: expected 2 results, got %d", len(results))
	}

	// Pagination: page 2
	results, _, err = s.QueryArticles(ArticleFilter{Page: 2, PerPage: 2})
	if err != nil {
		t.Fatalf("pagination page 2 error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("pagination page 2: expected 2 results, got %d", len(results))
	}

	// Pagination: page 3 (last page, 1 item)
	results, _, err = s.QueryArticles(ArticleFilter{Page: 3, PerPage: 2})
	if err != nil {
		t.Fatalf("pagination page 3 error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("pagination page 3: expected 1 result, got %d", len(results))
	}
}

func TestDeleteArticlesBefore(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewArticleStore(db)

	now := "2026-03-17T12:00:00Z"
	old := "2026-01-01T00:00:00Z"

	articles := []Article{
		{Title: "Old 1", URL: "http://a.com/1", Source: "test", Category: "AI", Language: "en", PublishedAt: &old},
		{Title: "Old 2", URL: "http://a.com/2", Source: "test", Category: "AI", Language: "en", PublishedAt: &old},
		{Title: "New 1", URL: "http://a.com/3", Source: "test", Category: "AI", Language: "en", PublishedAt: &now},
	}
	_, _, err := s.BatchInsertArticles(articles)
	if err != nil {
		t.Fatalf("seed error: %v", err)
	}

	// Verify 3 articles
	_, total, err := s.QueryArticles(ArticleFilter{PerPage: 100})
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected 3 articles, got %d", total)
	}

	// Delete articles collected before 2026-03-17
	deleted, err := s.DeleteArticlesBefore("2026-03-17")
	if err != nil {
		t.Fatalf("DeleteArticlesBefore error: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	// Verify 1 remaining
	_, total, err = s.QueryArticles(ArticleFilter{PerPage: 100})
	if err != nil {
		t.Fatalf("query after delete error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 remaining, got %d", total)
	}
}

// Suppress unused import warning for os.
var _ = os.Remove
