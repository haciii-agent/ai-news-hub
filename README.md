# AI News Hub 📰

> AI/科技新闻聚合平台 —— 多源采集、智能分类、全文搜索、原文预览，纯 Go 单二进制部署。

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![SQLite](https://img.shields.io/badge/SQLite-FTS5-003B57?logo=sqlite)](https://www.sqlite.org/fts5.html)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

## ✨ 功能特性

| 功能 | 说明 |
|------|------|
| 📡 **14 个数据源** | 12 RSS + 2 HTML 抓取，覆盖中英文 AI/科技媒体 |
| 🏷️ **8 个分类** | AI/ML、科技前沿、商业动态、开源生态、学术研究、政策监管、产品发布、综合资讯 |
| 🤖 **智能分类** | 基于关键词加权匹配 + 数据源预分类加成 |
| 🔍 **全文搜索** | SQLite FTS5 驱动，标题+摘要即时搜索 |
| 📖 **原文预览** | Readability 算法提取正文，详情页内嵌展示，无需跳转原站 |
| 🖼️ **图片提取** | 自动抓取 og:image / media:content / enclosure 封面图 |
| 🌓 **主题切换** | 深色/浅色模式，localStorage 持久化 |
| 🔄 **定时采集** | 内置调度器 + 手动触发 + 状态监控 |
| 📦 **REST API** | 完整 JSON API，支持分页、分类、语言过滤 |
| 🐳 **单二进制** | 前端嵌入 `embed.FS`，一个文件搞定部署 |

## 📡 数据源

| 类型 | 数据源 |
|------|--------|
| RSS | Hacker News, TechCrunch AI, The Verge AI, OpenAI Blog, Google AI Blog, MIT Tech Review, Ars Technica, HuggingFace Blog, 36氪, InfoQ 中文, 少数派, 极客公园 |
| HTML | 机器之心, 量子位 |

## 🚀 快速开始

**前提：** Go 1.25+

```bash
git clone https://github.com/haciii-agent/ai-news-hub.git
cd ai-news-hub

# 编译
go build -ldflags "-X main.version=latest" -o ai-news-hub .

# 启动
./ai-news-hub
```

访问 http://localhost:8080 即可使用。

## 📋 API 文档

### 健康检查

```
GET /health
```

### 文章列表

```
GET /api/v1/articles?category=AI/ML&page=1&per_page=20&sort=published_at&language=en
```

### 全文搜索

```
GET /api/v1/articles?search=量子计算
```

### 文章详情 & 原文预览

```
GET /api/v1/articles/{id}
GET /api/v1/articles/{id}/content   # 获取/提取原文正文
```

### 分类 & 数据源

```
GET /api/v1/categories
GET /api/v1/sources
```

### 采集控制

```
POST /api/v1/collect              # 手动触发采集
GET  /api/v1/collect/status       # 查询采集状态
```

## ⚙️ 配置

主配置文件：`config/config.yaml`

```yaml
server:
  host: "0.0.0.0"
  port: 8080

database:
  path: "./data/news.db"

collector:
  user_agent: "ai-news-hub/1.0"
  timeout: 30s
  max_concurrent: 5
  rss_max_items: 20
  request_interval: "2-5s"

classifier:
  rules_path: "./config/rules.yaml"

log:
  level: "info"
```

环境变量 `NEWS_HUB_CONFIG` 可指定配置文件路径。

## 📁 项目结构

```
ai-news-hub/
├── main.go                           # 入口
├── config/
│   ├── config.go                     # 配置加载
│   ├── config.yaml                   # 主配置
│   └── rules.yaml                    # 分类规则（8类关键词权重）
├── internal/
│   ├── api/                          # HTTP API 路由 + Handler
│   │   ├── server.go                 # 路由注册
│   │   ├── handler_article.go        # 文章/搜索/原文预览
│   │   └── handler_collect.go        # 采集控制/数据源/统计
│   ├── classifier/                   # 智能分类引擎
│   │   ├── classifier.go             # 关键词加权分类
│   │   └── rules.yaml                # 分类规则数据
│   ├── collector/                    # 数据采集
│   │   ├── collector.go              # 采集调度器
│   │   ├── rss.go                    # RSS/Atom 解析
│   │   ├── html.go                   # HTML 页面抓取 + 专用解析器
│   │   ├── readability.go            # Readability 正文提取
│   │   └── sources.go                # 数据源注册表（14源）
│   ├── static/                       # 嵌入式前端 (embed.FS)
│   │   ├── index.html                # 首页（新闻列表 + 搜索 + 分类筛选）
│   │   ├── article.html              # 详情页（正文预览 + 主题切换）
│   │   ├── css/style.css             # 深色/浅色双主题样式
│   │   ├── js/app.js                 # 前端交互逻辑
│   │   └── static.go                 # embed 声明
│   └── store/                        # SQLite 数据层
│       ├── sqlite.go                 # 建表/迁移/FTS5 索引
│       └── article.go                # CRUD + 搜索 + 统计
├── TASK_v050_fix.md                  # 开发任务记录
├── TASK_v060.md                      # 开发任务记录
└── README.md
```

## 📝 版本历史

### v0.6.0 — 原文预览
- Readability 算法提取正文（article → main → 启发式 → 文本密度 → body 回退）
- 详情页内嵌正文展示，支持展开/收起
- 服务端代理抓取 + content_html 缓存
- 标签白名单过滤，图片 URL 绝对化
- SSRF 防护（仅允许 http/https）

### v0.5.0 — 全文搜索
- SQLite FTS5 全文搜索引擎（标题 + 摘要）
- `?search=关键词` 接口支持
- og:image / media:content / enclosure 图片自动提取
- 摘要扩展至 2000 字符

### v0.3.0 — 视觉大改版
- 深色模式 UI 重设计（参考 Product Hunt / 少数派风格）
- 分类药丸标签 + 数量气泡
- 采集状态标签（✓/⚠️/✗）
- 实时时钟 + 统计栏
- `/api/v1/sources` 数据源接口

### v0.2.0 — 体验优化
- RSS 超时/条数配置化
- 前端交互优化 + 接口补全

### v0.1.0 — MVP
- 14 个数据源采集
- 8 个智能分类
- 纯 Go 单二进制 + 嵌入式前端

## 🛠️ 技术栈

- **后端：** Go 1.25+ / SQLite (modernc.org/sqlite, 纯 Go 无 CGO)
- **前端：** 原生 HTML/CSS/JS，通过 `embed.FS` 嵌入二进制
- **数据库：** SQLite + FTS5 全文搜索
- **采集：** RSS/Atom (gofeed) + HTML DOM 解析 (golang.org/x/net/html)

## License

MIT
