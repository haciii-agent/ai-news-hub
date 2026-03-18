# v1.1.0 AI 热点趋势分析

## 目标
基于新闻内容和用户行为数据，自动发现和量化科技领域热点趋势。包括话题热度排行、趋势时间线、选题推荐列表——这是"内容生产飞轮"的输入端。

## 项目路径
`/home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/`

## 一、趋势分析引擎

### 新增 `internal/ai/trend.go`

核心结构：
```go
type TrendAnalyzer struct {
    store store.ArticleStore
}

type TrendTopic struct {
    Keyword     string   `json:"keyword"`
    Score       float64  `json:"score"`        // 综合热度 0-100
    ArticleCount int     `json:"article_count"`
    RecentCount int      `json:"recent_count"` // 近24h文章数
    Trend       string   `json:"trend"`        // "rising" / "stable" / "declining"
    RelatedTags []string `json:"related_tags"`
    TopArticles []Article `json:"top_articles"`
}
```

### 分析维度

**1. 话题热度统计**
- 从最近 N 天的文章标题+摘要中提取高频关键词
- 过滤停用词（the, is, a, 的, 了, 在...）
- 关键词频率 * 时间权重（越新权重越高）
- 合并相似词（如 "GPT-5" 和 "GPT5"）

**2. 趋势方向判断**
- 对比最近 24h vs 前 24h 的关键词频率
- rising：频率增长 > 30%
- stable：频率变化 ±30%
- declining：频率下降 > 30%

**3. 综合热度评分**
```
Score = 频率分(40%) + 增速分(25%) + 来源多样性(15%) + 用户互动(20%)
```
- 频率分：关键词出现次数归一化
- 增速分：基于趋势方向和增速
- 来源多样性：出现在多少个不同数据源
- 用户互动：相关文章的总阅读数 + 收藏数

### 时间窗口
- 快速检测：最近 24 小时
- 日报分析：最近 7 天
- 周报分析：最近 30 天

## 二、API 接口

### 热点话题排行
```
GET /api/v1/trends/hot?period=7d&limit=20
```

参数：
- `period`: `24h` / `7d` / `30d`（默认 7d）
- `limit`: 返回数量（默认 20，最大 50）

返回：
```json
{
  "period": "7d",
  "topics": [
    {
      "keyword": "GPT-5",
      "score": 92,
      "article_count": 23,
      "recent_count": 8,
      "trend": "rising",
      "related_tags": ["GPT", "OpenAI", "大模型"],
      "top_articles": [{ "id": 1, "title": "...", "source": "...", "published_at": "..." }]
    },
    ...
  ],
  "generated_at": "2026-03-18T21:00:00Z"
}
```

### 关键词趋势时间线
```
GET /api/v1/trends/timeline?keyword=GPT&days=14
```

返回每天该关键词出现的文章数量：
```json
{
  "keyword": "GPT",
  "days": 14,
  "data": [
    {"date": "2026-03-05", "count": 3},
    {"date": "2026-03-06", "count": 5},
    ...
  ],
  "peak": {"date": "2026-03-15", "count": 12}
}
```

### 选题推荐列表
```
GET /api/v1/trends/story-pitches?limit=10
```

返回适合写深度博客的话题推荐：
```json
{
  "pitches": [
    {
      "topic": "Claude 4 发布分析",
      "score": 88,
      "reason": "近7天相关文章23篇，热度上升45%，覆盖8个数据源",
      "angle": "技术架构对比 + 行业影响分析",
      "related_articles_count": 23,
      "top_sources": ["TechCrunch", "The Verge", "Hacker News"],
      "writability": 85
    },
    ...
  ]
}
```

`writability`（可写性评分）基于：
- 相关文章数量是否足够支撑深度分析（5篇以上）
- 来源多样性（3个以上不同来源）
- 热度趋势是否为上升（rising > stable > declining）
- 是否有 AI 摘要可用（有摘要的更好写）

### 相关话题
```
GET /api/v1/trends/related?keyword=AI&limit=10
```

返回与指定关键词经常同时出现的关联话题。

## 三、数据库优化

### 趋势缓存表（可选，先不做）
初期直接实时查询，如果性能不够再考虑缓存。

### 所需查询
- 按时间范围统计文章数量（已有）
- 从标题+摘要中提取关键词频率（需要 SQL LIKE 或全文搜索）
- 按数据源分组统计（已有 GetSources）
- 读取阅读历史统计（已有 read_history 表）

### 关键词提取 SQL
利用 FTS5 的匹配功能：
```sql
-- 近7天文章
SELECT title, COALESCE(ai_summary, summary) FROM articles
WHERE collected_at >= datetime('now', '-7 days')

-- 用 Go 代码做关键词提取和频率统计
```

## 四、前端 — 新增 trends.html

### 页面布局

```
┌─────────────────────────────────────────────────┐
│ 🎯推荐 | 📊看板 | 📈趋势 | 📌收藏 | 📖历史    │
├─────────────────────────────────────────────────┤
│                                                 │
│  时间切换：[24小时] [7天] [30天]                  │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │ 🔥 热点话题排行榜                       │   │
│  │ #1 GPT-5        🔥92 ↑ rising   23篇   │   │
│  │ #2 Claude 4     🔥87 ↑ rising   18篇   │   │
│  │ #3 量子计算     ⚡72 → stable   12篇   │   │
│  │ #4 自动驾驶     ⚡65 ↓ declining  8篇   │   │
│  │ ...                                     │   │
│  │ 每个可展开查看相关文章列表               │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │ 📈 关键词趋势搜索                       │   │
│  │ [输入关键词...]  → 显示14天趋势柱状图    │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │ ✍️ 选题推荐（适合写博客的话题）          │   │
│  │ 1. Claude 4 发布分析  ⭐88  可写性85    │   │
│  │ 2. 开源大模型现状     ⭐82  可写性78    │   │
│  │ ...                                     │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
└─────────────────────────────────────────────────┘
```

### 趋势指示器
- 🔴 rising（上升）— 红色背景
- 🟡 stable（平稳）— 黄色背景
- 🟢 declining（下降）— 绿色背景（热度下降不一定是坏事）

### 关键词趋势图
- 纯 CSS 柱状图（复用 dashboard 的样式）
- 显示每天的柱子 + 峰值标注

## 五、导航更新

所有页面导航栏统一为：
`🎯推荐 | 📊看板 | 📈趋势 | 📌收藏 | 📖历史 | 🌓主题`

## 实现文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/ai/trend.go` | **新增** | 趋势分析引擎（关键词提取+频率统计+趋势判断+选题评分） |
| `internal/api/handler_trend.go` | **新增** | 趋势 API handler（热点排行/时间线/选题推荐/关联话题） |
| `internal/api/server.go` | 修改 | 注册趋势路由 |
| `internal/static/trends.html` | **新增** | 趋势分析页面 |
| `internal/static/css/style.css` | 修改 | 趋势页面样式 + 排行榜 + 指示器 |
| 所有 HTML 页面 | 修改 | 导航栏新增📈趋势 |

## 注意事项

- 趋势分析是纯本地计算，不依赖 LLM
- 关键词提取用 Go 代码实现（分词 + 频率统计），不需要复杂的 NLP 库
- 停用词列表内置（中英文常见停用词）
- 中英文关键词分开统计
- 如果文章数量太少（< 50篇），趋势分析结果可能不准确，前端展示提示
- 选题推荐的可写性评分要合理，不能全是高分
- 更新 README.md 版本历史

## 验证步骤

1. 编译：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go build -ldflags "-X main.version=1.1.0" -o ai-news-hub .`
2. 运行测试：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go test ./...`
3. 提交并推送：`cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && git add -A && git commit -m "v1.1.0: AI热点趋势分析 — 话题排行/趋势时间线/选题推荐" && git push`
