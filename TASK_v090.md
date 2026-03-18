# v0.9.0 AI 内容增强 — 智能摘要生成 + 重要度评分

## 目标
为采集到的新闻文章自动生成高质量中文摘要，并计算重要度评分。这是后续个性化推荐和热点分析的数据基础。

## 项目路径
`/home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/`

## 一、智能摘要生成

### 背景
当前文章摘要来自 RSS feed 的 description 字段，很多是空的、纯 HTML 的、或者只有一两句话。需要 AI 生成结构化的中文摘要。

### 实现方案

#### 后端：摘要生成服务
新增 `internal/ai/summarizer.go`

```go
type Summarizer struct {
    apiKey     string
    apiBase    string   // LLM API 地址
    httpClient *http.Client
    concurrency int     // 并发控制
}

// GenerateSummary 为单篇文章生成中文摘要
// 输入：文章标题 + 原始摘要（如有）+ 正文（如有）
// 输出：150-300字的中文摘要
func (s *Summarizer) GenerateSummary(article Article) (string, error)
```

**LLM 调用策略**：
- 使用 OpenAI 兼容的 Chat Completions API（复用现有的 API 配置模式）
- 通过环境变量或 config.yaml 配置 API Key 和 Base URL
- 摘要 Prompt 模板：
```
你是一个科技新闻编辑。请根据以下信息生成一段150-300字的中文新闻摘要。
要求：信息准确、重点突出、语言简洁专业、适合信息流展示。

标题：{title}
原文摘要：{summary}
来源：{source}

请直接输出摘要文本，不要添加任何前缀或格式。
```
- 超时控制：单篇 15 秒
- 并发限制：同时最多 3 个请求（避免 API 限流）
- 失败降级：如果 AI 摘要失败，保留原始摘要，不覆盖

#### 异步批量生成
在采集完成后，对新增文章（content_html 非空的）异步生成摘要：
- 新增 `POST /api/v1/ai/generate-summaries` 接口，手动触发批量摘要生成
- 采集流程中不自动调用（避免采集耗时过长）
- 前端在 Dashboard 上增加一个"生成摘要"按钮

#### 数据库
- articles 表新增字段 `ai_summary TEXT`（AI 生成的摘要）
- 新增字段 `importance_score REAL DEFAULT 0`（重要度评分 0-100）
- 新增字段 `summary_generated_at DATETIME`（摘要生成时间）

### 数据库迁移
在 `internal/store/sqlite.go` 的 migrate 函数中新增：
```sql
ALTER TABLE articles ADD COLUMN ai_summary TEXT;
ALTER TABLE articles ADD COLUMN importance_score REAL DEFAULT 0;
ALTER TABLE articles ADD COLUMN summary_generated_at DATETIME;
```

注意：使用 `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` 或捕获 "duplicate column" 错误（SQLite 不支持 IF NOT EXISTS for ALTER TABLE，需要忽略特定错误）。

## 二、重要度评分

### 评分算法（纯本地计算，不依赖 LLM）

```go
// CalculateImportance 计算文章重要度评分（0-100）
func CalculateImportance(article Article) float64
```

**评分维度**：

| 维度 | 权重 | 计算方式 |
|------|------|---------|
| 来源权重 | 30% | 核心源（HN/TechCrunch/MIT）= 90分，普通源 = 70分，边缘源 = 50分 |
| 时效性 | 25% | 24h内=100分，48h=80分，7天=50分，30天=20分，>30天=10分 |
| 分类热度 | 15% | 根据分类的文章数量归一化（文章越多=越热门） |
| 标题关键词 | 15% | 包含热点关键词（GPT/Llama/融资/突破/发布等）加分 |
| 摘要长度 | 10% | 有AI摘要 = 100分，有原始摘要 = 60分，无摘要 = 20分 |
| 图片 | 5% | 有封面图 = 100分，无图 = 30分 |

### 评分时机
- 采集入库时计算初始评分
- AI 摘要生成后重新计算（摘要维度分数更新）
- 提供 `POST /api/v1/ai/recalculate-scores` 接口批量重算

## 三、API 接口

### 摘要生成
```
POST /api/v1/ai/generate-summaries?limit=50
```
- 对最近 N 篇没有 ai_summary 的文章批量生成摘要
- 异步执行，返回任务信息
- `?limit=50` 限制本次处理数量（默认20，最大100）
- `?force=true` 强制重新生成（覆盖已有 ai_summary）

返回：
```json
{
  "task": "generate_summaries",
  "pending": 45,
  "processing": true,
  "message": "开始生成 45 篇文章的摘要..."
}
```

### 单篇摘要
```
POST /api/v1/ai/generate-summary/{id}
```
- 对指定文章生成摘要
- 返回生成的摘要

### 重算评分
```
POST /api/v1/ai/recalculate-scores
```
- 批量重算所有文章的重要度评分
- 返回更新的文章数

### 摘要状态
```
GET /api/v1/ai/summary-status
```
返回：
```json
{
  "total_articles": 1523,
  "has_ai_summary": 890,
  "has_original_summary": 1200,
  "no_summary": 323,
  "ai_coverage": 58.4
}
```

## 四、前端改造

### Dashboard 增强
- 新增"AI 摘要覆盖度"统计卡片
- 新增"生成摘要"按钮（点击后确认，触发 POST /api/v1/ai/generate-summaries）
- 生成过程中显示进度

### 文章列表增强
- 卡片显示重要度评分标签（如 🔥85、⚡62）
- 如果有 ai_summary，优先展示 ai_summary 而非原始 summary
- 评分高的文章可以加一个微妙的边框高亮

### 文章详情增强
- 显示 AI 摘要（如果有）+ 原始摘要（折叠）
- 显示重要度评分

## 五、配置

### config.yaml 新增
```yaml
ai:
  api_base: "https://open.bigmodel.cn/api/paas/v4"  # 或其他兼容 OpenAI 的 API
  api_key: ""           # 也可以通过环境变量 AI_API_KEY 设置
  model: "glm-4-flash"  # 生成摘要的模型（用便宜快速的即可）
  max_concurrent: 3     # 并发请求数
  timeout: 15s          # 单篇超时
```

环境变量优先级：`AI_API_KEY` > config.yaml 中的 ai.api_key

## 实现文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/ai/summarizer.go` | **新增** | LLM 摘要生成服务 |
| `internal/ai/scorer.go` | **新增** | 重要度评分算法 |
| `internal/api/handler_ai.go` | **新增** | AI 相关 API handler |
| `internal/api/server.go` | 修改 | 注册 AI 路由 + 配置注入 |
| `internal/store/sqlite.go` | 修改 | 新增 ai_summary/importance_score/summary_generated_at 字段 |
| `internal/store/article.go` | 修改 | 查询时返回新字段 + 更新摘要方法 |
| `config/config.go` | 修改 | 新增 AI 配置结构 |
| `config/config.yaml` | 修改 | 新增 ai 配置节 |
| `internal/static/dashboard.html` | 修改 | 摘要覆盖度 + 生成按钮 |
| `internal/static/index.html` | 修改 | 卡片显示评分 + AI摘要优先 |
| `internal/static/article.html` | 修改 | AI摘要展示 + 评分 |
| `internal/static/css/style.css` | 修改 | 评分标签样式 |

## 注意事项

- AI API Key 不要硬编码，通过配置文件或环境变量传入
- 摘要生成是异步的，不能阻塞采集流程
- LLM API 调用要有完善的错误处理和重试（429限流时等待重试）
- 重要度评分是纯本地计算，不依赖任何外部服务
- ALTER TABLE 时要兼容已有数据库（忽略 duplicate column 错误）
- 如果没有配置 AI_API_KEY，AI 功能静默不可用，不影响其他功能
- 配置文件中 ai.api_key 留空，用注释说明通过环境变量设置

## 验证步骤

1. 编译：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go build -ldflags "-X main.version=0.9.0" -o ai-news-hub .`
2. 运行测试：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go test ./...`
3. 提交并推送：`cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && git add -A && git commit -m "v0.9.0: AI内容增强 — 智能摘要生成 + 重要度评分" && git push`
