# v0.8.0 数据看板 — 运营监控 Dashboard

## 目标
新增数据看板页面，可视化展示系统运行状态：采集数据、文章趋势、分类分布、数据源健康度。让运营方一眼了解系统运转情况。

## 项目路径
`/home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/`

## 一、后端 API

### 1. 新增 `GET /api/v1/dashboard/stats` — 核心统计概览

返回格式：
```json
{
  "total_articles": 1523,
  "today_new": 47,
  "total_sources": 14,
  "active_sources": 12,
  "failed_sources": 2,
  "total_categories": 8,
  "total_collect_runs": 89,
  "last_collect_time": "2026-03-18T19:30:00Z",
  "last_collect_status": "partial",
  "latest_collect": {
    "total_collected": 150,
    "total_new": 47,
    "errors_count": 4,
    "duration_seconds": 32
  }
}
```

### 2. 新增 `GET /api/v1/dashboard/trend?days=7` — 文章增长趋势

参数：days（默认7，最大30）

返回格式：
```json
{
  "days": 7,
  "data": [
    {"date": "2026-03-12", "new_articles": 23, "total_articles": 1340},
    {"date": "2026-03-13", "new_articles": 35, "total_articles": 1375},
    ...
  ]
}
```

实现：按 collected_at 的日期分组 COUNT，同时累计总数。

### 3. 新增 `GET /api/v1/dashboard/categories` — 分类分布

返回格式：
```json
{
  "categories": [
    {"name": "AI/ML", "count": 523, "percentage": 34.3},
    {"name": "科技前沿", "count": 312, "percentage": 20.5},
    {"name": "商业动态", "count": 201, "percentage": 13.2},
    ...
  ]
}
```

### 4. 新增 `GET /api/v1/dashboard/sources` — 数据源健康度

返回格式：
```json
{
  "sources": [
    {"name": "Hacker News", "type": "rss", "article_count": 234, "status": "healthy", "last_success": "2026-03-18T19:30:00Z", "success_rate": 0.95},
    {"name": "OpenAI Blog", "type": "rss", "article_count": 45, "status": "failing", "last_success": "2026-03-15T10:00:00Z", "success_rate": 0.3},
    ...
  ]
}
```

status 取值：healthy（最近24h成功过）、degraded（最近48h成功过）、failing（超过48h未成功）、never（从未成功过）

实现：基于 collect_runs 的 errors 字段（JSON数组，含 source name 和 error message），统计每个源的成功/失败次数。同时在 articles 表统计各源文章数。

### 5. 新增 `GET /api/v1/dashboard/recent-articles?limit=10` — 最新文章速览

与现有 articles 接口类似但简化，只返回 id/title/source/category/published_at，用于看板展示。

### 6. 新增 `GET /api/v1/dashboard/collect-history?limit=10` — 采集历史

最近 N 次采集运行的记录，用于监控采集稳定性。

返回格式：
```json
{
  "runs": [
    {"id": 89, "started_at": "2026-03-18T19:30:00Z", "finished_at": "2026-03-18T19:30:32Z", "status": "partial", "total_collected": 150, "total_new": 47, "errors_count": 4},
    ...
  ]
}
```

## 二、Store 层

在 `internal/store/article.go` 中新增 Dashboard 相关查询方法（或新建 `internal/store/dashboard.go`）。

### 需要的查询：
1. 总文章数、今日新增数
2. 按日期分组的文章增长（过去 N 天）
3. 分类分布统计（复用 GetCategoryStats）
4. 各数据源文章数量
5. 采集运行历史（复用 GetLatestCollectRun，新增 GetRecentCollectRuns）
6. 采集成功/失败统计（解析 collect_runs.errors JSON）

### 关于数据源状态
当前 collect_runs.errors 是 JSON 字符串，格式需要确认。如果格式为 `[{"source":"OpenAI Blog","error":"403"}]`，则解析后可以统计各源失败次数。同时对比所有源，如果某源不在 errors 中则认为成功。

建议新增一个辅助结构体：
```go
type SourceError struct {
    Source string `json:"source"`
    Error  string `json:"error"`
}
```

## 三、前端 — 新增 dashboard.html

### 页面布局

```
┌─────────────────────────────────────────────────┐
│ 📰 AI News Hub    首页  收藏  历史  📊 看板     │
├─────────────────────────────────────────────────┤
│                                                 │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐           │
│  │ 1523 │ │  47  │ │ 12/14│ │  89  │           │
│  │总文章 │ │今日新增│ │活跃源 │ │采集次数│           │
│  └──────┘ └──────┘ └──────┘ └──────┘           │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │ 📈 文章增长趋势（过去7天）                │   │
│  │ ▁▃▅▇█▆▄ (纯CSS柱状图/折线图)            │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  ┌──────────────────┐ ┌──────────────────┐     │
│  │ 🏷️ 分类分布       │ │ 📡 数据源健康度   │     │
│  │ AI/ML     34.3%  │ │ ✅ HN      234篇  │     │
│  │ 科技前沿   20.5%  │ │ ✅ TC      178篇  │     │
│  │ 商业动态   13.2%  │ │ ❌ OpenAI   45篇  │     │
│  │ (水平进度条)       │ │ ⚠️ Google   89篇  │     │
│  └──────────────────┘ └──────────────────┘     │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │ 🕐 最近采集记录                          │   │
│  │ 19:30 采集150篇 新增47 失败4源 ⚠️         │   │
│  │ 12:00 采集142篇 新增38 失败3源 ⚠️         │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │ 📰 最新文章速览（最新10篇标题列表）        │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
└─────────────────────────────────────────────────┘
```

### 图表实现
- **不引入外部图表库**，使用纯 CSS 实现：
  - 柱状图：`<div>` 宽度百分比 + 背景色
  - 分类分布：水平进度条 + 百分比文字
  - 数据源状态：绿/黄/红指示灯 + 文字
- 保持项目"零外部依赖"的原则

### 深色/浅色主题
- 复用现有 CSS 变量体系
- dashboard 页面同样支持主题切换

## 四、路由和导航

### server.go 新增路由
```
GET /api/v1/dashboard/stats
GET /api/v1/dashboard/trend
GET /api/v1/dashboard/categories
GET /api/v1/dashboard/sources
GET /api/v1/dashboard/recent-articles
GET /api/v1/dashboard/collect-history
```

### 导航栏更新
在所有页面的顶栏（index.html / article.html / bookmarks.html / history.html）中：
- 新增"📊 看板"链接，指向 /dashboard.html
- 位置：与"📌 收藏"、"📖 历史"并列

## 实现文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/store/dashboard.go` | **新增** | Dashboard 数据查询（统计/趋势/源状态） |
| `internal/api/handler_dashboard.go` | **新增** | Dashboard 6个 API handler |
| `internal/api/server.go` | 修改 | 注册 dashboard 路由 |
| `internal/static/dashboard.html` | **新增** | 数据看板页面 |
| `internal/static/css/style.css` | 修改 | 看板样式（卡片、柱状图、进度条） |
| `internal/static/index.html` | 修改 | 顶栏导航增加"看板" |
| `internal/static/article.html` | 修改 | 顶栏导航增加"看板" |
| `internal/static/bookmarks.html` | 修改 | 顶栏导航增加"看板" |
| `internal/static/history.html` | 修改 | 顶栏导航增加"看板" |

## 注意事项

- Dashboard 接口不需要用户 Token（公开数据）
- 趋势查询不要做太复杂的 SQL，SQLite 性能有限
- 数据源状态需要解析 collect_runs.errors JSON 字段，注意处理空 JSON/格式不兼容的情况
- 如果数据库是全新的（0篇文章），看板应该展示空状态而不是报错
- 所有页面的顶栏导航保持一致（首页/看板/收藏/历史 + 主题切换）
- 纯 CSS 图表不需要动画效果，静态展示即可

## 验证步骤

1. 编译：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go build -ldflags "-X main.version=0.8.0" -o ai-news-hub .`
2. 运行测试：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go test ./...`
3. 提交并推送：`cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && git add -A && git commit -m "v0.8.0: 数据看板 — 运营监控Dashboard + 趋势图表 + 数据源健康度" && git push`
