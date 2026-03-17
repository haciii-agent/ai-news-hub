-- 001_init.sql - 初始数据库建表
-- articles 表：存储采集到的新闻文章
-- collect_runs 表：记录每次采集运行状态

-- 文章表
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
