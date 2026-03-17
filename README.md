# AI News Hub ⚔️

AI 领域新闻聚合服务 —— 从多个 RSS/HTML 数据源自动采集、智能分类、Web 展示。

## 功能特性

| 特性 | 说明 |
|------|------|
| 📡 **14 个数据源** | 12 个 RSS + 2 个 HTML 抓取，覆盖中英文 AI/科技媒体 |
| 🏷️ **8 个分类** | AI/ML、科技前沿、商业动态、开源生态、学术研究、政策监管、产品发布、综合资讯 |
| 🤖 **智能分类** | 基于关键词加权匹配 + 数据源预分类加成的分类引擎 |
| 🔄 **定时采集** | 内置调度器，支持手动触发 & 状态查询 |
| 🌐 **Web UI** | 嵌入式前端，打开即用 |
| 📦 **REST API** | 完整的 JSON API，支持分页、分类过滤、排序 |
| 🔥 **规则热更新** | 分类规则修改后无需重启，自动检测并重载 |

### 数据源一览

| 类型 | 数据源 |
|------|--------|
| RSS | Hacker News, TechCrunch AI, The Verge AI, OpenAI Blog, Google AI Blog, MIT Tech Review, Ars Technica, HuggingFace Blog, 36氪, InfoQ 中文, 少数派, 极客公园 |
| HTML | 机器之心, 量子位 |

## 快速开始

### 本地运行

**前提：** Go 1.25+, GCC (CGO required for SQLite)

```bash
# 克隆项目
git clone <repo-url> && cd news-hub

# 编译
make build

# 启动
./ai-news-hub
# 或
make run
```

服务启动后访问 http://localhost:8080

### Docker 部署

```bash
# 一键启动
make docker-up

# 查看日志
make docker-logs

# 停止
make docker-down
```

## API 文档

### 健康检查

```
GET /health
GET /healthz
```

```json
{"status": "ok", "service": "ai-news-hub", "version": "0.1.0"}
```

### 文章列表

```
GET /api/v1/articles?category=ai_ml&page=1&per_page=20&sort=published_at&language=en
```

**参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `category` | string | - | 按分类过滤，可选值见 `/api/v1/categories` |
| `page` | int | 1 | 页码 |
| `per_page` | int | 20 | 每页条数 (1-100) |
| `sort` | string | `published_at` | 排序字段 |
| `language` | string | - | 语言过滤 (`en` / `zh`) |

### 文章详情

```
GET /api/v1/articles/{id}
```

### 分类列表 & 统计

```
GET /api/v1/categories
```

### 手动触发采集

```
POST /api/v1/collect
```

### 采集状态查询

```
GET /api/v1/collect/status
```

## 配置说明

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
  request_interval: "2-5s"

classifier:
  rules_path: "./config/rules.yaml"

log:
  level: "info"
```

环境变量：

| 变量 | 说明 |
|------|------|
| `NEWS_HUB_CONFIG` | 指定配置文件路径（默认 `./config/config.yaml`） |

### 分类规则

分类规则定义在 `config/rules.yaml`，包含 8 个分类的关键词和权重配置。

> ⚠️ **PERIODIC_UPDATE 标记**
>
> 分类规则中标注了 `# PERIODIC_UPDATE` 的关键词为时效性关键词（如产品型号、模型版本号等），需要定期维护更新以保持分类准确性。建议每 1-2 个月审查一次。

## 项目结构

```
news-hub/
├── main.go                          # 入口
├── config/
│   ├── config.go                    # 配置加载
│   ├── config.yaml                  # 主配置
│   └── rules.yaml                   # 分类规则
├── internal/
│   ├── api/                         # HTTP API
│   ├── classifier/                  # 分类引擎
│   ├── collector/                   # 数据采集
│   ├── static/                      # 嵌入式前端
│   └── store/                       # SQLite 存储
├── Makefile
├── Dockerfile
├── docker-compose.yaml
└── README.md
```

## MVP 功能限制

当前版本为 MVP，以下功能暂不可用：

| 功能 | 状态 | 说明 |
|------|------|------|
| 🔍 全文搜索 | ⚠️ 暂不可用 | `search` 参数已预留接口，后端返回 400 |

后续迭代将集成全文搜索引擎（如 SQLite FTS5 或 MeiliSearch）。

## License

MIT
