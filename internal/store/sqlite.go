package store

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// SQL schema for creating tables on first run.
// Field names align with Article and CollectRun models in article.go.
var schemaSQL = `
CREATE TABLE IF NOT EXISTS articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,
    source TEXT NOT NULL,
    source_url TEXT,
    category TEXT NOT NULL DEFAULT '综合资讯',
    summary TEXT,
    content_html TEXT,
    image_url TEXT,
    published_at DATETIME,
    collected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    language TEXT DEFAULT 'en'
);

CREATE INDEX IF NOT EXISTS idx_articles_category ON articles(category);
CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published_at DESC);
CREATE INDEX IF NOT EXISTS idx_articles_collected ON articles(collected_at DESC);

-- 采集运行记录表（含 errors_count 字段，方便快速判断是否有错）
CREATE TABLE IF NOT EXISTS collect_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    status TEXT DEFAULT 'running',
    total_collected INTEGER DEFAULT 0,
    total_new INTEGER DEFAULT 0,
    errors_count INTEGER DEFAULT 0,
    errors TEXT
);
`

// NewDB opens (or creates) a SQLite database at dbPath, runs migrations,
// and returns the *sql.DB handle.
func NewDB(dbPath string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}

	// Connection pool tuning for SQLite (single-writer).
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	log.Printf("[store] SQLite database ready: %s", dbPath)
	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(schemaSQL)
	return err
}
