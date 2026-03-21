package store

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

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

-- 用户表（匿名 token 机制）
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_token ON users(token);

-- 收藏表
CREATE TABLE IF NOT EXISTS bookmarks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    article_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (article_id) REFERENCES articles(id),
    UNIQUE(user_id, article_id)
);

CREATE INDEX IF NOT EXISTS idx_bookmarks_user ON bookmarks(user_id);

-- 阅读历史表
CREATE TABLE IF NOT EXISTS read_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    article_id INTEGER NOT NULL,
    read_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (article_id) REFERENCES articles(id),
    UNIQUE(user_id, article_id)
);

CREATE INDEX IF NOT EXISTS idx_read_history_user ON read_history(user_id);
CREATE INDEX IF NOT EXISTS idx_read_history_time ON read_history(read_at DESC);

-- 用户画像表（v1.0.0）
CREATE TABLE IF NOT EXISTS user_profiles (
    user_id INTEGER PRIMARY KEY,
    interests TEXT DEFAULT '{}',            -- JSON: {"GPT": 0.85, "NLP": 0.6, ...}
    preferred_categories TEXT DEFAULT '[]', -- JSON: ["AI/ML", "科技前沿"]
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

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

// ftsSQL creates the FTS5 virtual table and triggers for automatic sync.
var ftsSQL = `
-- FTS5 全文搜索虚拟表（content= 表示外部内容表，避免数据冗余）
CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(
    title, summary, content='articles',
    content_rowid='id',
    tokenize='unicode61'
);

-- INSERT 触发器：文章插入后同步到 FTS
CREATE TRIGGER IF NOT EXISTS articles_fts_insert AFTER INSERT ON articles BEGIN
    INSERT INTO articles_fts(rowid, title, summary) VALUES (new.id, new.title, COALESCE(new.summary, ''));
END;

-- DELETE 触发器：文章删除后同步删除 FTS 记录
CREATE TRIGGER IF NOT EXISTS articles_fts_delete AFTER DELETE ON articles BEGIN
    INSERT INTO articles_fts(articles_fts, rowid, title, summary) VALUES('delete', old.id, old.title, COALESCE(old.summary, ''));
END;

-- UPDATE 触发器：文章更新后同步 FTS 记录
CREATE TRIGGER IF NOT EXISTS articles_fts_update AFTER UPDATE ON articles BEGIN
    INSERT INTO articles_fts(articles_fts, rowid, title, summary) VALUES('delete', old.id, old.title, COALESCE(old.summary, ''));
    INSERT INTO articles_fts(rowid, title, summary) VALUES (new.id, new.title, COALESCE(new.summary, ''));
END;
`

// userSystemSQL adds user system tables and columns for v1.2.0.
// Uses ALTER TABLE ADD COLUMN (ignores duplicate column errors) and
// CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS for idempotency.
var userSystemSQL = `
-- users table extensions
ALTER TABLE users ADD COLUMN username TEXT;
ALTER TABLE users ADD COLUMN email TEXT;
ALTER TABLE users ADD COLUMN password_hash TEXT;
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'anonymous';
ALTER TABLE users ADD COLUMN disabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN merged_into INTEGER DEFAULT NULL;

-- indexes for users
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE username IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_disabled ON users(disabled);

-- oauth_accounts table
CREATE TABLE IF NOT EXISTS oauth_accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    access_token TEXT,
    refresh_token TEXT,
    expires_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    UNIQUE(provider, provider_user_id)
);
CREATE INDEX IF NOT EXISTS idx_oauth_user ON oauth_accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_provider ON oauth_accounts(provider, provider_user_id);

-- login_logs table
CREATE TABLE IF NOT EXISTS login_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    username TEXT,
    ip_address TEXT,
    user_agent TEXT,
    success INTEGER NOT NULL DEFAULT 1,
    fail_reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_login_logs_user ON login_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_login_logs_time ON login_logs(created_at DESC);
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
	// 1. Create base tables
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}

	// 2. Create FTS5 table and triggers
	if _, err := db.Exec(ftsSQL); err != nil {
		return fmt.Errorf("exec fts: %w", err)
	}

	// 3. Backfill existing articles into FTS index (if any)
	//    This handles the case where articles exist before FTS was added.
	var articleCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM articles").Scan(&articleCount); err != nil {
		return fmt.Errorf("count articles for fts backfill: %w", err)
	}
	if articleCount > 0 {
		result, err := db.Exec(
			`INSERT INTO articles_fts(rowid, title, summary)
			 SELECT id, title, summary FROM articles
			 WHERE id NOT IN (SELECT rowid FROM articles_fts)`,
		)
		if err != nil {
			return fmt.Errorf("fts backfill: %w", err)
		}
		if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
			log.Printf("[store] FTS backfill: indexed %d existing articles", rowsAffected)
		}
	}

	// 4. v0.9.0: Add AI fields to articles table (ignore duplicate column errors)
	alterStatements := []string{
		"ALTER TABLE articles ADD COLUMN ai_summary TEXT",
		"ALTER TABLE articles ADD COLUMN importance_score REAL DEFAULT 0",
		"ALTER TABLE articles ADD COLUMN summary_generated_at DATETIME",
	}
	for _, stmt := range alterStatements {
		if _, err := db.Exec(stmt); err != nil {
			// Ignore "duplicate column name" errors (SQLite)
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("alter articles table: %w", err)
			}
		}
	}

	// 5. v1.2.0: User system migration
	if _, err := db.Exec(userSystemSQL); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("exec user system migration: %w", err)
		}
	}

	return nil
}
