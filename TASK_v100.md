# v1.0.0 个性化推荐 + 用户画像

## 目标
基于用户的阅读历史和收藏行为，构建兴趣标签画像，实现"猜你喜欢"个性化推荐 Feed 流。这是"体验飞轮"的核心——推荐越准，用户越活跃，数据越多，推荐越准。

## 项目路径
`/home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/`

## 一、用户画像系统

### 兴趣标签体系

预定义标签池（与分类体系互补，更细粒度）：

```go
// 内置兴趣标签
var InterestTags = []string{
    // 大模型
    "GPT", "Claude", "Gemini", "Llama", "GLM", "Qwen", "Mistral", "DeepSeek",
    // 技术方向
    "NLP", "计算机视觉", "强化学习", "多模态", "代码生成", "RAG", "Agent", "AI安全",
    // 行业
    "自动驾驶", "机器人", "医疗AI", "金融AI", "教育AI", "芯片", "云计算",
    // 话题
    "开源", "融资", "产品发布", "学术论文", "政策法规", "创业",
}
```

### 画像构建逻辑

**数据来源**：
- 用户阅读的文章 → 提取分类和关键词 → 加权
- 用户收藏的文章 → 更高权重
- 浏览频率 → 时间衰减

**画像存储**：
```sql
CREATE TABLE IF NOT EXISTS user_profiles (
    user_id INTEGER PRIMARY KEY,
    interests TEXT DEFAULT '{}',       -- JSON: {"GPT": 0.85, "NLP": 0.6, ...}
    preferred_categories TEXT DEFAULT '[]', -- JSON: ["AI/ML", "科技前沿"]
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
```

**画像更新时机**：
- 每次记录阅读历史时，异步更新用户画像
- 用户收藏/取消收藏时，更新画像
- 画像更新基于增量，不每次全量计算

### 关键词提取

从文章中提取兴趣标签的方法（不依赖 LLM，纯本地计算）：
1. **分类映射**：文章分类 → 相关标签（如"AI/ML" → GPT,Claude,Llama,NLP,多模态 等）
2. **标题匹配**：标题中是否包含预定义标签关键词
3. **来源偏好**：经常阅读的来源也作为画像维度

## 二、推荐算法

### 基于内容的推荐（初期方案）

```go
// ScoreForUser 计算单篇文章对指定用户的推荐评分（0-100）
func ScoreForUser(article Article, profile UserProfile) float64
```

**评分维度**：

| 维度 | 权重 | 说明 |
|------|------|------|
| 分类匹配 | 40% | 文章分类是否在用户偏好分类中 |
| 标签匹配 | 30% | 文章相关标签与用户兴趣标签的重叠度 |
| 重要度 | 15% | 文章的 importance_score |
| 时效性 | 10% | 越新的文章权重越高 |
| 来源多样性 | 5% | 降低用户已大量阅读来源的权重，增加多样性 |

**去重**：已阅读的文章不出现在推荐中（基于 read_history 表）

## 三、API 接口

### 推荐文章列表
```
GET /api/v1/recommendations?page=1&per_page=20
Header: X-User-Token: <uuid>
```

返回：
```json
{
  "articles": [...],
  "total": 45,
  "page": 1,
  "per_page": 20,
  "reason": "基于你的阅读偏好推荐"
}
```

逻辑：
1. 获取用户画像（如果没有，返回按重要度排序的文章）
2. 从最近 7 天的文章中筛选
3. 排除已阅读的文章
4. 按 ScoreForUser 评分排序
5. 分页返回

### 用户画像
```
GET /api/v1/user/profile
Header: X-User-Token: <uuid>
```

返回：
```json
{
  "interests": {"GPT": 0.85, "NLP": 0.6, "多模态": 0.4},
  "preferred_categories": ["AI/ML", "科技前沿"],
  "total_reads": 123,
  "total_bookmarks": 15,
  "reading_streak": 3
}
```

### 更新兴趣偏好
```
PUT /api/v1/user/profile
Header: X-User-Token: <uuid>
Body: { "preferred_categories": ["AI/ML", "开源生态"] }
```

允许用户手动调整偏好分类，覆盖算法推断的结果。

### 阅读连续天数
```
GET /api/v1/user/streak
Header: X-User-Token: <uuid>
```

返回：
```json
{
  "current_streak": 3,
  "longest_streak": 7,
  "total_reading_days": 25
}
```

## 四、前端改造

### 新增 recommendations.html（推荐页）
- 首页"推荐"频道
- 显示"基于你的阅读偏好推荐"提示
- 文章卡片布局与首页相同，增加推荐原因标签
- 空状态（新用户）：展示热门文章 + "阅读更多文章，推荐会越来越准"

### index.html 改造
- 顶栏导航新增"🎯 推荐"链接（放在最前面，高亮显示）
- 首页默认 tab 改为"推荐"（有 token 时），无 token 时显示"最新"
- 分类标签栏新增"🎯 猜你喜欢"选项

### 顶栏导航最终顺序
`🎯推荐 | 📊看板 | 📌收藏 | 📖历史`

### 用户画像展示
- 在看板页面新增"我的画像"区域
- 展示兴趣标签（大小不同的标签云）
- 展示偏好分类
- 展示阅读连续天数

## 五、实现细节

### 画像更新算法（增量式）

每次用户阅读一篇文章时：
1. 从文章的分类映射到相关标签
2. 从文章标题提取匹配的标签
3. 对匹配的标签增加权重（+0.1），未匹配的标签权重衰减（×0.95）
4. 权重归一化到 [0, 1]
5. 写入 user_profiles 表

收藏操作时，匹配标签权重增加更多（+0.2）。

### 分类到标签的映射

```go
var CategoryTagMap = map[string][]string{
    "AI/ML":     {"GPT", "Claude", "Gemini", "Llama", "GLM", "Qwen", "NLP", "多模态", "代码生成", "RAG", "Agent"},
    "科技前沿":   {"芯片", "云计算", "量子计算", "区块链", "机器人"},
    "商业动态":   {"融资", "创业", "IPO", "收购"},
    "开源生态":   {"开源", "Llama", "Mistral", "DeepSeek"},
    "学术研究":   {"学术论文", "论文", "研究"},
    "政策监管":   {"政策法规", "监管", "AI安全", "隐私"},
    "产品发布":   {"产品发布", "GPT", "Claude", "Gemini"},
    "综合资讯":   {},
}
```

## 实现文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/store/sqlite.go` | 修改 | 新增 user_profiles 表 |
| `internal/store/profile.go` | **新增** | 用户画像 CRUD + 兴趣标签管理 |
| `internal/ai/recommender.go` | **新增** | 推荐评分算法 + 文章筛选 |
| `internal/api/handler_recommend.go` | **新增** | 推荐/画像/streak API |
| `internal/api/server.go` | 修改 | 注册推荐路由 |
| `internal/store/user.go` | 修改 | 阅读历史记录时触发画像更新 |
| `internal/static/recommendations.html` | **新增** | 推荐 Feed 流页面 |
| `internal/static/index.html` | 修改 | 导航新增"推荐" + 首页默认tab |
| `internal/static/dashboard.html` | 修改 | 用户画像展示区 |
| `internal/static/article.html` | 修改 | 导航更新 |
| `internal/static/bookmarks.html` | 修改 | 导航更新 |
| `internal/static/history.html` | 修改 | 导航更新 |
| `internal/static/css/style.css` | 修改 | 推荐页面样式 + 标签云 |

## 注意事项

- 没有 user_token 的情况下，推荐页显示热门文章（按 importance_score 排序）
- 新用户（无阅读历史）也显示热门文章 + 引导文案
- 画像更新是异步的，不阻塞阅读/收藏操作
- 推荐算法不依赖 LLM，全部本地计算
- 所有页面导航统一：🎯推荐 | 📊看板 | 📌收藏 | 📖历史 | 🌓主题

## 验证步骤

1. 编译：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go build -ldflags "-X main.version=1.0.0" -o ai-news-hub .`
2. 运行测试：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go test ./...`
3. 提交并推送：`cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && git add -A && git commit -m "v1.0.0: 个性化推荐 — 用户画像 + 兴趣标签 + 猜你喜欢" && git push`
