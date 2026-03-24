# AI News Hub 📰

> AI/科技新闻聚合平台 —— 多源采集、智能分类、全文搜索、AI 摘要、个性化推荐、运营看板，纯 Go 单二进制部署。

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![SQLite](https://img.shields.io/badge/SQLite-FTS5-003B57?logo=sqlite)](https://www.sqlite.org/fts5.html)
[![Version](https://img.shields.io/badge/Version-1.2.0-blue)](https://github.com/haciii-agent/ai-news-hub/releases)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

## ✨ 功能特性

| 功能 | 说明 |
|------|------|
| 📡 **14 个数据源** | 12 RSS + 2 HTML 抓取，覆盖中英文 AI/科技媒体 |
| 🏷️ **8 个分类** | AI/ML、科技前沿、商业动态、开源生态、学术研究、政策监管、产品发布、综合资讯 |
| 🤖 **智能分类** | 基于关键词加权匹配 + 数据源预分类加成 |
| 🔍 **全文搜索** | SQLite FTS5 驱动，标题+摘要即时搜索 |
| 📖 **原文预览** | Readability 算法提取正文，详情页内嵌展示，无需跳转原站 |
| 🤖 **AI 智能摘要** | LLM 自动生成中文摘要（OpenAI 兼容 API） |
| 🎯 **个性化推荐** | 基于用户画像的兴趣标签匹配，"猜你喜欢" Feed 流 |
| 👤 **用户画像** | 兴趣标签体系 + 阅读连续天数 + 偏好分类管理 |
| 🔥 **重要度评分** | 多维度文章评分（来源权重+时效性+分类热度+关键词+摘要+图片） |
| 📌 **收藏 & 📖 阅读历史** | 匿名 Token 机制，无需注册登录 |
| 📊 **运营看板** | 采集监控 + 增长趋势 + 分类分布 + 数据源健康度 + AI 摘要覆盖度 |
| 📈 **热点趋势** | 话题排行榜 + 趋势时间线 + 选题推荐 + 关联话题发现 |
| 🖼️ **图片提取** | 自动抓取 og:image / media:content / enclosure 封面图 |
| 🌓 **主题切换** | 深色/浅色模式，localStorage 持久化 |
| 💬 **评论系统** | 文章评论 + 匿名发表 + 删除自己的评论 |
| ❤️ **点赞互动** | 文章点赞/取消 + 卡片快捷点赞 |
| 📤 **分享功能** | 复制链接 + Twitter + 微博分享 |
| 🔄 **定时采集** | 内置调度器 + 手动触发 + 状态监控 |
| 📦 **REST API** | 完整 JSON API，支持分页、分类、语言过滤、推荐、搜索 |
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

### AI 摘要功能（可选）

设置环境变量启用 AI 摘要生成：

```bash
export AI_API_KEY="your-api-key"
export AI_API_BASE="https://open.bigmodel.cn/api/paas/v4"
export AI_MODEL="glm-4-flash"
./ai-news-hub
```

## 📋 API 文档

### 健康检查

```
GET /health
```

### 文章 & 搜索

```
GET /api/v1/articles?category=AI/ML&page=1&per_page=20&sort=published_at&language=en
GET /api/v1/articles?search=量子计算
GET /api/v1/articles/{id}
GET /api/v1/articles/{id}/content
```

### 个性化推荐

```
GET /api/v1/recommendations?page=1&per_page=20          # 推荐列表（需 X-User-Token）
GET /api/v1/user/profile                                 # 用户画像
PUT /api/v1/user/profile                                 # 更新偏好
GET /api/v1/user/streak                                  # 阅读连续天数
```

### 收藏 & 历史

```
POST   /api/v1/user/init                                 # 用户初始化
POST   /api/v1/bookmarks                                 # 收藏文章
DELETE /api/v1/bookmarks/{id}                            # 取消收藏
GET    /api/v1/bookmarks                                 # 收藏列表
GET    /api/v1/bookmarks/status?ids=1,2,3                # 批量收藏状态
POST   /api/v1/history                                   # 记录阅读
GET    /api/v1/history                                   # 阅读历史
```

### 评论 & 点赞

```
POST   /api/v1/articles/{id}/comments                     # 发表评论
GET    /api/v1/articles/{id}/comments?page=1&per_page=20   # 评论列表
DELETE /api/v1/articles/{id}/comments/{comment_id}         # 删除评论
POST   /api/v1/articles/{id}/like                         # 点赞
DELETE /api/v1/articles/{id}/like                         # 取消点赞
GET    /api/v1/articles/{id}/interactions                 # 文章互动状态
GET    /api/v1/articles/interactions?ids=1,2,3            # 批量互动状态
```

### AI 功能

```
POST /api/v1/ai/generate-summaries?limit=50              # 批量生成 AI 摘要
POST /api/v1/ai/generate-summary/{id}                    # 单篇生成摘要
POST /api/v1/ai/recalculate-scores                       # 重算重要度评分
GET  /api/v1/ai/summary-status                           # 摘要状态
```

### 数据看板

```
GET /api/v1/dashboard/stats                              # 核心统计
GET /api/v1/dashboard/trend?days=7                       # 增长趋势
GET /api/v1/dashboard/categories                         # 分类分布
GET /api/v1/dashboard/sources                            # 数据源健康度
GET /api/v1/dashboard/recent-articles?limit=10            # 最新文章
GET /api/v1/dashboard/collect-history?limit=10            # 采集历史
```

### 热点趋势

```
GET /api/v1/trends/hot?period=7d&limit=20                # 热点话题排行
GET /api/v1/trends/timeline?keyword=GPT&days=14          # 关键词趋势时间线
GET /api/v1/trends/story-pitches?limit=10                # 选题推荐列表
GET /api/v1/trends/related?keyword=AI&limit=10           # 关联话题
```

### 采集 & 分类

```
POST /api/v1/collect                                     # 手动触发采集
GET  /api/v1/collect/status                              # 采集状态
GET  /api/v1/categories                                  # 分类列表
GET  /api/v1/sources                                     # 数据源列表
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
  rss_max_items: 30
  request_interval: "2-5s"

classifier:
  rules_path: "./config/rules.yaml"

ai:
  api_base: "https://open.bigmodel.cn/api/paas/v4"
  api_key: ""                # 或通过环境变量 AI_API_KEY 设置
  model: "glm-4-flash"
  max_concurrent: 3
  timeout: 15s

log:
  level: "info"
```

环境变量：
- `NEWS_HUB_CONFIG` — 指定配置文件路径
- `AI_API_KEY` — AI 摘要 API Key（优先级高于 config）
- `AI_API_BASE` — AI API 地址（可选覆盖）
- `AI_MODEL` — 生成摘要的模型（可选覆盖）

## 📁 项目结构

```
ai-news-hub/
├── main.go                           # 入口
├── config/
│   ├── config.go                     # 配置加载
│   ├── config.yaml                   # 主配置
│   └── rules.yaml                    # 分类规则（8类关键词权重）
├── internal/
│   ├── ai/                           # AI 服务
│   │   ├── summarizer.go             # LLM 智能摘要生成
│   │   ├── scorer.go                 # 多维度重要度评分算法
│   │   ├── recommender.go            # 个性化推荐算法
│   │   └── trend.go                  # 热点趋势分析引擎
│   ├── api/                          # HTTP API
│   │   ├── server.go                 # 路由注册 + 中间件
│   │   ├── handler_article.go        # 文章/搜索/原文预览
│   │   ├── handler_collect.go        # 采集控制/数据源/统计
│   │   ├── handler_dashboard.go      # 数据看板 API
│   │   ├── handler_trend.go          # 热点趋势 API
│   │   ├── handler_user.go           # 用户/收藏/历史
│   │   ├── handler_ai.go             # AI 摘要/评分
│   │   └── handler_recommend.go      # 推荐/画像
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
│   │   ├── article.html              # 详情页（正文预览 + AI摘要 + 收藏）
│   │   ├── recommendations.html      # 推荐页（猜你喜欢 Feed 流）
│   │   ├── bookmarks.html            # 收藏列表页
│   │   ├── history.html              # 阅读历史页
│   │   ├── dashboard.html            # 运营看板（统计/趋势/源状态/画像）
│   │   ├── trends.html               # 热点趋势（排行榜/时间线/选题推荐）
│   │   ├── css/style.css             # 深色/浅色双主题样式
│   │   ├── js/app.js                 # 前端交互逻辑
│   │   └── static.go                 # embed 声明
│   └── store/                        # SQLite 数据层
│       ├── sqlite.go                 # 建表/迁移/FTS5 索引
│       ├── article.go                # 文章 CRUD + 搜索 + 统计
│       ├── user.go                   # 用户/收藏/阅读历史
│       ├── profile.go                # 用户画像 + 兴趣标签
│       └── dashboard.go              # 看板数据查询
└── README.md
```

## 📝 版本历史

### v1.2.0 — 评论点赞 + 社交互动
- 评论系统（发表/列表/删除，1-500 字符限制，匿名用户）
- 点赞系统（点赞/取消，卡面快捷点赞，已点赞变红）
- 批量互动状态查询（卡片显示 ❤️15 · 💬8）
- 文章详情页互动栏（点赞数 + 评论数 + 点赞按钮）
- 分享功能（复制链接 + Twitter + 微博，纯前端实现）
- 评论/点赞触发用户画像更新（标签权重 +0.15/+0.10）
- 看板新增互动统计（总评论/今日评论/总点赞/最热文章 TOP5）
- 评论需要 X-User-Token，无 Token 只可查看
- 7 个新 API 接口

### v1.1.0 — AI 热点趋势分析
- 热点话题排行榜（综合热度评分：频率40% + 增速25% + 来源多样性15% + 用户互动20%）
- 趋势方向判断（rising/stable/declining，对比24h前后频率变化）
- 关键词趋势时间线（14天柱状图，纯 CSS 实现）
- 选题推荐列表（可写性评分：文章数+来源多样性+趋势方向+摘要可用性）
- 关联话题发现（基于关键词共现频率）
- 中英文关键词提取（Go 代码分词 + 停用词过滤，无需 NLP 库）
- 时间加权词频统计（越新的文章权重越高）
- 相似词合并（GPT-5 / GPT5 自动合并）
- 新增趋势分析页面（排行榜 + 趋势搜索 + 选题推荐）
- 4 个新 API：/api/v1/trends/hot, timeline, story-pitches, related
- 纯本地计算，不依赖 LLM

### v1.0.0 — 个性化推荐
- 基于内容的推荐算法（分类匹配 + 标签匹配 + 重要度 + 时效性 + 多样性）
- 用户画像系统（兴趣标签 + 偏好分类 + 阅读连续天数）
- 画像自动更新（阅读/收藏触发，增量式权重调整）
- 推荐页 + "猜你喜欢" Feed 流
- 新用户/无 Token 降级为热门文章排序
- Dashboard 用户画像展示（标签云）

### v0.9.0 — AI 内容增强
- LLM 智能摘要生成（OpenAI 兼容 API，支持并发控制 + 429 重试）
- 多维度重要度评分（来源权重30% + 时效性25% + 分类热度15% + 关键词15% + 摘要10% + 图片5%）
- 摘要覆盖度统计 + 批量生成/重算接口
- 文章卡片评分标签 + 详情页 AI 摘要展示

### v0.8.0 — 数据看板
- 运营监控 Dashboard（核心统计/增长趋势/分类分布/数据源健康度）
- 采集历史记录 + 最新文章速览
- 纯 CSS 柱状图 + 进度条 + 状态灯
- AI 摘要覆盖度统计 + 生成按钮

### v0.7.0 — 用户功能
- 匿名 Token 机制（无需注册登录）
- 收藏文章（一键收藏/取消 + 批量状态查询）
- 阅读历史（自动记录 + 历史列表）
- 收藏页 + 历史页 + 收藏心跳动画

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
- **AI：** OpenAI 兼容 API（GLM/GPT/Qwen 等任意模型）

## License

MIT
