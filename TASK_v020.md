# v0.2.0 优化任务（精确 diff）

## 任务1：RSS 采集超时和条数从 config 读取

### 问题
`internal/collector/collector.go` 中 `NewCollectScheduler()` 创建 RSSCollector 时没有传入 timeout，导致 RSSCollector 使用默认 15s 而非 config.yaml 的 30s。
也没有传入每源最大条数限制。

### 修改文件1: config/config.go
在 `CollectorConfig` struct 中添加：
```go
RSSMaxItems int `yaml:"rss_max_items"`
```
在 `Load()` 函数 defaults 部分添加：
```go
if cfg.Collector.RSSMaxItems == 0 {
    cfg.Collector.RSSMaxItems = 20
}
```

### 修改文件2: internal/collector/rss.go
1. 给 `RSSCollector` struct 添加字段：
```go
maxItems int
```

2. 添加 option:
```go
func WithMaxItems(n int) RSSOption {
    return func(c *RSSCollector) {
        if n > 0 {
            c.maxItems = n
        }
    }
}
```

3. 在 `NewRSSCollector` 中设置默认值：
```go
maxItems: 20,
```

4. 在 `convertRSSItems` 中，return 之前截断：
```go
if len(articles) > c.maxItems {
    articles = articles[:c.maxItems]
}
return articles, nil
```
同样在 `convertAtomEntries` 中。

### 修改文件3: internal/collector/collector.go
`NewCollectScheduler` 改为接受 config 参数：
```go
func NewCollectScheduler(cfg *config.CollectorConfig) *CollectScheduler {
    return &CollectScheduler{
        rssCollector: NewRSSCollector(
            WithFetchTimeout(cfg.Timeout),
            WithMaxItems(cfg.RSSMaxItems),
            WithUserAgent(cfg.UserAgent),
            WithMaxWorkers(cfg.MaxConcurrent),
        ),
        htmlCollector: NewHTMLCollector(
            WithHTMLMaxWorkers(3),
            func(c *HTMLCollector) {
                for name, parser := range DefaultHTMLParsers() {
                    c.parsers[name] = parser
                }
            },
        ),
    }
}
```

### 修改文件4: internal/api/server.go
`NewServer` 中传入 config：
```go
sched := collector.NewCollectScheduler(&cfg.Collector)
```

### 修改文件5: config/config.yaml
在 collector 部分添加：
```yaml
  rss_max_items: 30
```

---

## 任务2：前端优化

### 修改文件: internal/static/index.html
1. 在 header 中 lang-switcher 前添加最后采集时间显示：
```html
<div class="header-center">
  <span id="lastCollectTime" class="last-collect"></span>
</div>
```

### 修改文件: internal/static/js/app.js
1. 在 init() 中添加调用 loadStats()
2. 添加 loadStats 函数，调用 GET /api/v1/stats，更新 #lastCollectTime 显示
3. 格式化为"最近采集: 2分钟前 · 新增149篇"

### 修改文件: internal/static/css/style.css
1. 添加 .last-collect 样式（灰色小字）
2. 给 .article-card 添加 hover 效果：
```css
.article-card:hover {
    box-shadow: 0 4px 12px rgba(0,0,0,0.08);
    transform: translateY(-1px);
    transition: all 0.2s ease;
}
```
3. 确保 .category-tabs 支持横向滚动：
```css
.category-tabs {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    white-space: nowrap;
}
.cat-tab {
    display: inline-block;
    white-space: nowrap;
}
```

---

## 编译验证
```bash
export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go build -o ai-news-hub .
```
