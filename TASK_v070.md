# v0.7.0 用户功能 — 收藏 + 阅读历史

## 目标
为 AI News Hub 添加轻量级用户功能：收藏文章、阅读历史。不需要注册登录，使用浏览器生成的匿名 ID + localStorage 持久化，服务端存储数据。

## 设计思路

**为什么不做注册登录？**
- 新闻聚合平台不需要复杂用户系统
- 避免引入密码存储、邮箱验证、OAuth 等重型依赖
- 保持单二进制、零外部依赖的优势

**方案：匿名 Token 机制**
- 前端首次访问时生成 UUID v4 作为 `user_token`，存入 localStorage
- 所有用户操作通过 `X-User-Token` 请求头携带 token
- 服务端根据 token 识别用户，无需认证流程
- 如果 token 丢失（换浏览器/清缓存），视为新用户（可接受）

## 项目路径
`/home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/`

## 功能清单

### 1. 数据库新增表

在 `internal/store/sqlite.go` 的 `schemaSQL` 中新增：

```sql
-- 用户表（匿名 token 机制）
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_token ON users(token);

-- 收藏表
CREATE TABLE IF NOT EXISTS bookmarks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    article_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (article_id) REFERENCES articles(id),
    UNIQUE(user_id, article_id)
);

CREATE INDEX IF NOT EXISTS idx_bookmarks_user ON bookmarks(user_id);

-- 阅读历史表
CREATE TABLE IF NOT EXISTS read_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    article_id INTEGER NOT NULL,
    read_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (article_id) REFERENCES articles(id),
    UNIQUE(user_id, article_id)
);

CREATE INDEX IF NOT EXISTS idx_read_history_user ON read_history(user_id);
CREATE INDEX IF NOT EXISTS idx_read_history_time ON read_history(read_at DESC);
```

### 2. Store 层新增方法

在 `internal/store/article.go` 中新增：

```go
// 用户管理
GetOrCreateUserByToken(token string) (*User, error)
UpdateUserLastSeen(userID int64) error

// 收藏
BookmarkArticle(userID, articleID int64) error
UnbookmarkArticle(userID, articleID int64) error
IsBookmarked(userID, articleID int64) (bool, error)
ListBookmarks(userID int64, filter ArticleFilter) ([]Article, int, error)

// 阅读历史
RecordReadHistory(userID, articleID int64) error
ListReadHistory(userID int64, filter ArticleFilter) ([]Article, int, error)
```

### 3. API 接口

#### 用户初始化
```
POST /api/v1/user/init
Header: X-User-Token: <uuid>
```
- 如果 token 不存在，创建新用户
- 如果 token 已存在，更新 last_seen_at
- 返回 `{ "user_id": 1, "token": "xxx", "created": false }`

#### 收藏文章
```
POST /api/v1/bookmarks
Header: X-User-Token: <uuid>
Body: { "article_id": 123 }
```
- 返回 `{ "bookmarked": true }`

#### 取消收藏
```
DELETE /api/v1/bookmarks/123
Header: X-User-Token: <uuid>
```
- 返回 `{ "bookmarked": false }`

#### 收藏列表
```
GET /api/v1/bookmarks?page=1&per_page=20
Header: X-User-Token: <uuid>
```
- 返回 `{ "articles": [...], "total": N, "page": 1, "per_page": 20 }`

#### 阅读历史
```
GET /api/v1/history?page=1&per_page=20
Header: X-User-Token: <uuid>
```
- 返回 `{ "articles": [...], "total": N, "page": 1, "per_page": 20 }`

#### 记录阅读
```
POST /api/v1/history
Header: X-User-Token: <uuid>
Body: { "article_id": 123 }
```
- 幂等操作，重复阅读只更新 read_at 时间
- 返回 `{ "recorded": true }`

#### 批量查询收藏状态
```
GET /api/v1/bookmarks/status?ids=1,2,3,4,5
Header: X-User-Token: <uuid>
```
- 返回 `{ "bookmarks": { "1": true, "2": false, "3": true, ... } }`

### 4. 中间件

新增 `getUserID` 辅助函数（非强制中间件）：
- 从 `X-User-Token` header 获取 token
- 查询/创建用户，返回 user_id
- 如果没有 token，返回 0（游客模式，收藏/历史功能不可用但不影响浏览）

### 5. 前端改造

#### index.html 改造
- 顶栏新增收藏按钮 📌（跳转到收藏页）
- 新闻卡片右下角新增收藏按钮（❤️ / 🤍 切换）
- 页面加载时获取当前页文章的收藏状态
- 点击收藏时发送 POST/DELETE，无需刷新

#### 新增 bookmarks.html（收藏页）
- 与首页类似的卡片列表布局
- 显示收藏的文章，按收藏时间倒序
- 每张卡片有取消收藏按钮
- 空状态："还没有收藏任何文章"

#### 新增 history.html（历史页）
- 类似收藏页，显示最近阅读的文章
- 按阅读时间倒序
- 空状态："还没有阅读记录"

#### article.html 改造
- 进入详情页时自动 POST /api/v1/history 记录阅读
- 详情页显示收藏按钮（大按钮，在"阅读原文"旁边）

#### JS 逻辑 (app.js 增强)
- 初始化时生成/获取 user_token（localStorage）
- 所有 API 请求自动附加 X-User-Token header
- 收藏状态管理（批量查询 + 单篇切换）

#### CSS 增强
- 收藏/历史页面样式
- 收藏按钮动画（点击心跳效果）
- 空状态插图样式

## 实现文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/store/sqlite.go` | 修改 | 新增 users/bookmarks/read_history 表 |
| `internal/store/article.go` | 修改 | 新增 User 模型 + 用户/收藏/历史 CRUD |
| `internal/store/user.go` | **新增** | UserStore 接口和实现（收藏+历史的业务逻辑） |
| `internal/api/server.go` | 修改 | 注册新路由 + getUserID 辅助函数 |
| `internal/api/handler_user.go` | **新增** | 用户初始化 + 收藏 + 历史 handler |
| `internal/static/index.html` | 修改 | 卡片收藏按钮 + 顶栏导航 |
| `internal/static/article.html` | 修改 | 自动记录阅读 + 收藏按钮 |
| `internal/static/bookmarks.html` | **新增** | 收藏列表页 |
| `internal/static/history.html` | **新增** | 阅读历史页 |
| `internal/static/css/style.css` | 修改 | 新页面样式 + 收藏动画 |
| `internal/static/js/app.js` | 修改 | user_token 管理 + 收藏交互 |

## 注意事项

- user_token 不需要加密或签名，就是简单的 UUID v4
- 所有涉及用户操作的接口都需要 X-User-Token header
- 文章列表 API 返回时可选包含 is_bookmarked 字段（需要 token 时）
- 阅读历史只记录 article_id 和时间，不记录阅读时长
- 收藏和历史的分页复用现有的 ArticleFilter
- 不要破坏现有 API 的行为
- 前端在 user_token 不存在时静默降级（不弹窗，不影响浏览）

## 验证步骤

1. 编译：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go build -ldflags "-X main.version=0.7.0" -o ai-news-hub .`
2. 运行测试：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go test ./...`
3. 提交并推送：`cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && git add -A && git commit -m "v0.7.0: 用户功能 — 匿名Token + 收藏 + 阅读历史" && git push`
