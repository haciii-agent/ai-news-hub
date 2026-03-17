# v0.3.0 大改版 — 视觉 + 功能体验提升

## 目标
让用户一打开就能感受到和 v0.1.0 的明显区别。

## 任务1：前端全面重做（最重要）

### 风格参考
参考 Product Hunt / Hacker News / 少数派 的混合风格，但要有自己的特色。

### 配色方案（深色模式为主）
```
背景：#0f0f0f（近黑）
卡片：#1a1a2e（深蓝灰）
顶栏：#16213e（深蓝）
强调色：#e94560（红粉）
文字：#eaeaea（浅灰白）
次要文字：#8a8a8a
```

### index.html 重做
```html
- 顶栏：Logo + "AI News Hub" + 实时时钟 + 最后采集状态标签（绿色✓/黄色⚠️）
- 统计栏：一行显示"共 N 篇 · 中文 N · 英文 N · 7 个分类 · 最近采集: Xm前"
- 分类标签栏：药丸形状(pill)，选中高亮，每个带数量气泡，横向可滚动
- 新闻卡片重新设计：
  - 左侧：序号（大字灰色）
  - 右侧：标题（加粗，可点击跳详情）+ 一行摘要（灰色小字）+ 底栏（来源·时间·语言badge·分类tag）
  - hover 效果：卡片左侧加一条彩色边线（按分类颜色）
  - 语言标识：中文用🇨🇳 英文用🇬🇧
- 底部：加载更多按钮 + 版权信息
```

### article.html 重做
```
- 深色背景卡片式布局
- 标题大字 + 来源 + 时间 + 分类标签
- 摘要正文
- 底部大按钮"阅读原文 →"（新窗口打开）
```

### css/style.css 全面重写
- CSS 变量定义配色
- 暗色模式为默认
- 分类标签各自独立配色（8种）
- 卡片 hover 左侧彩条效果
- 药丸形分类按钮
- 滚动条美化
- 响应式（移动端全宽卡片）

### js/app.js 增强
- 页面加载时先调 /api/v1/stats 获取总数/最近采集时间
- 实时时钟（顶栏右侧）
- 分类标签点击：切换时平滑滚动到顶部
- 加载更多：按钮变 loading 动画
- 空状态/错误状态更好看

## 任务2：新增 /api/v1/sources 接口

### handler_collect.go 新增
```go
// HandleSources GET /api/v1/sources — 返回数据源列表和状态
func (s *CollectService) HandleSources(w http.ResponseWriter, r *http.Request)
```

返回格式：
```json
{
  "sources": [
    {"name": "Hacker News", "url": "...", "type": "rss", "language": "en", "status": "active"},
    {"name": "OpenAI Blog", "url": "...", "type": "rss", "language": "en", "status": "failed"}
  ]
}
```

从 sources.go 的数据源列表 + 最近一次 collect_runs 的 errors 构建状态。

### server.go 注册路由
```go
mux.HandleFunc("/api/v1/sources", s.CollectSvc.HandleSources)
```

## 任务3：采集状态标签改进

### js/app.js
- 从 /api/v1/stats 获取 latest_collect.status
- status=success → 绿色标签 "✓ 采集正常"  
- status=partial → 黄色标签 "⚠ 部分源失败"
- status=failed → 红色标签 "✗ 采集失败"
- status=never_run → 灰色标签 "尚未采集"

## 任务4：version 号从 main.go 注入

### main.go
```go
var version = "dev"
```

### api/server.go
Server struct 加 Version 字段，healthHandler 用 s.Version。

### Makefile
```makefile
VERSION ?= 0.3.0
build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o ai-news-hub .
```

---

编译验证：export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go build -ldflags "-X main.version=0.3.0" -o ai-news-hub .
