# v0.6.0 原文预览 — 在详情页内嵌展示原文内容

## 目标
用户在文章详情页点击「阅读原文」时，不再跳转到外部网站，而是在当前页面内展示原文内容（通过服务端代理抓取 + Readability 提取正文）。同时保留跳转原站的外链。

## 项目路径
`/home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/`

## 核心功能

### 1. 新增 `/api/v1/articles/:id/content` 接口

**功能**：服务端代理抓取原文页面，提取正文内容后返回。

**逻辑流程**：
1. 从数据库查询文章的 URL
2. 如果 `content_html` 字段已有值（缓存），直接返回
3. 如果没有，启动服务端抓取：
   - HTTP GET 获取原文 HTML
   - 用 Go 实现的 Readability 算法提取正文（提取 `<article>`、`<main>`、或通过启发式规则找正文容器）
   - 清理 HTML：移除 script/style/nav/footer/aside/广告等无关内容
   - 保留基本格式：`<p>`、`<h2>`-`<h4>`、`<img>`、`<blockquote>`、`<ul>`/`<ol>`、`<a>`、`<strong>`、`<em>`
   - 将图片 URL 转为绝对路径
   - 结果写入数据库 `content_html` 字段（缓存）
4. 返回提取的 HTML 内容

**响应格式**：
```json
{
  "html": "<div class=\"article-content\"><h2>标题</h2><p>段落...</p><img src=\"...\"></div>",
  "title": "原文标题（用于确认）",
  "cached": false,
  "fetch_time_ms": 1234
}
```

**错误处理**：
- 抓取超时（10秒）→ 返回 504 + 错误信息
- 无法解析正文 → 返回 200 但 `html` 为空，附带提示
- 防止 SSRF：只允许 http/https 协议

### 2. 实现 Readability 正文提取器

在 `internal/collector/` 下新增 `readability.go`：

**提取策略**（按优先级）：
1. 查找 `<article>` 标签
2. 查找 `<main>` 标签
3. 查找 `role="main"` 的元素
4. 查找 class 含 `content`、`article`、`post`、`entry` 的 `<div>`
5. 启发式：计算每个 `<div>` 的文本密度（文本字符数 / 总字符数），取密度最高且文本量足够的
6. 回退：取 `<body>` 的前 5000 字符

**清理规则**：
- 移除 `script`、`style`、`nav`、`footer`、`header`、`aside`、`iframe`、`noscript`
- 移除 class/id 含 `sidebar`、`comment`、`ad`、`social`、`share`、`related`、`newsletter` 的元素
- 保留的标签白名单：`p, h1, h2, h3, h4, h5, h6, img, a, blockquote, ul, ol, li, strong, em, br, hr, pre, code, table, thead, tbody, tr, th, td, figure, figcaption`
- 所有图片 src 转为绝对路径（基于原文 base URL）

### 3. 前端 article.html 改造

**改造详情页展示逻辑**：

- 「阅读原文 →」按钮改为双按钮：
  - 主按钮「📖 查看正文」→ 点击后调用 `/api/v1/articles/:id/content`，在下方展开正文区域
  - 次按钮「🔗 原文链接」→ 新窗口打开原站（保留现有行为）

**正文区域 UI**：
```
┌─────────────────────────────────────┐
│ 📖 正文预览  来源: xxx.com  耗时: 1.2s │
├─────────────────────────────────────┤
│                                     │
│  [提取的正文 HTML 渲染]              │
│                                     │
│  图片居中显示，宽度自适应             │
│  段落间距适当                       │
│                                     │
├─────────────────────────────────────┤
│ ⚠️ 此内容由 AI 自动提取，如有错漏    │
│    请以原文为准                      │
└─────────────────────────────────────┘
```

**交互细节**：
- 点击「查看正文」→ 按钮变为 loading 状态（转圈）
- 加载完成后：平滑展开正文区域，按钮变为「收起正文」
- 再次点击「收起正文」→ 平滑收起
- 抓取失败时：显示错误提示（"正文提取失败，请直接访问原文"）
- 正文中的链接保持可点击（新窗口打开）

### 4. CSS 样式

在 `css/style.css` 中新增正文预览相关样式：

```css
/* 正文预览容器 */
.content-preview { ... }
.content-preview-header { ... }
.content-preview-body { ... }

/* 正文排版 */
.content-preview-body p { line-height: 1.8; margin-bottom: 1em; }
.content-preview-body h2 { ... }
.content-preview-body img { max-width: 100%; height: auto; border-radius: 8px; margin: 1em auto; display: block; }
.content-preview-body blockquote { ... }
.content-preview-body pre { ... }
.content-preview-body a { color: var(--accent); }
```

## 实现文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/collector/readability.go` | 新增 | Readability 正文提取器 |
| `internal/api/handler_article.go` | 修改 | 新增 HandleArticleContent handler |
| `internal/api/server.go` | 修改 | 注册新路由 |
| `internal/static/article.html` | 修改 | 详情页 UI 改造 |
| `internal/static/css/style.css` | 修改 | 正文预览样式 |
| `internal/static/js/app.js` | 修改 | 不需要（article.html 自包含JS） |

## 注意事项

- 抓取超时设为 10 秒
- User-Agent 随机化（复用 html.go 中的 randomUserAgent()）
- 需要处理编码问题（复用 ensureUTF8）
- 正文提取不需要完美，关键是能用——90% 的文章能提取出可读正文即可
- 不要修改现有的 API 接口行为
- `content_html` 字段已有，直接复用

## 验证步骤

1. 编译：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go build -ldflags "-X main.version=0.6.0" -o ai-news-hub .`
2. 运行测试：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go test ./...`
3. 如果 v0.5.0 的 commit 还没做，先 commit v0.5.0，然后在此基础上开发
4. 完成后 commit：`cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && git add -A && git commit -m "v0.6.0: 原文预览 — Readability 正文提取 + 详情页内嵌展示"`
