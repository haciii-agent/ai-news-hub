# v1.2.0 评论 + 社交互动

## 目标
为文章添加评论、点赞、分享功能，完成用户反馈闭环（子闭环B）。用户互动数据为内容质量评估和采集策略优化提供数据基础。

## 项目路径
`/home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/`

## 一、评论系统

### 数据库
```sql
-- 评论表
CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    article_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_comments_article ON comments(article_id, created_at DESC);

-- 点赞表
CREATE TABLE IF NOT EXISTS likes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    article_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id),
    UNIQUE(article_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_likes_article ON likes(article_id);
```

### API 接口

#### 发表评论
```
POST /api/v1/articles/{id}/comments
Header: X-User-Token: <uuid>
Body: { "content": "这篇文章很有见地..." }
```

#### 评论列表
```
GET /api/v1/articles/{id}/comments?page=1&per_page=20
```

返回：
```json
{
  "comments": [
    {
      "id": 1,
      "user_id": 1,
      "content": "这篇文章很有见地...",
      "created_at": "2026-03-18T20:00:00Z",
      "article_id": 42
    }
  ],
  "total": 5,
  "page": 1,
  "per_page": 20
}
```

#### 删除评论
```
DELETE /api/v1/comments/{id}
Header: X-User-Token: <uuid>
```
只能删除自己的评论。

#### 点赞/取消点赞
```
POST /api/v1/articles/{id}/like     # 点赞
DELETE /api/v1/articles/{id}/like   # 取消点赞
Header: X-User-Token: <uuid>
```

#### 文章互动状态
```
GET /api/v1/articles/{id}/interactions
Header: X-User-Token: <uuid>
```

返回：
```json
{
  "likes_count": 15,
  "comments_count": 8,
  "is_liked": true
}
```

#### 批量查询互动状态
```
GET /api/v1/articles/interactions?ids=1,2,3,4,5
Header: X-User-Token: <uuid>
```

返回：
```json
{
  "interactions": {
    "1": { "likes_count": 15, "comments_count": 8 },
    "2": { "likes_count": 3, "comments_count": 1 },
    ...
  }
}
```

### Store 层

新增 `internal/store/comment.go`：
```go
type CommentStore interface {
    AddComment(articleID, userID int64, content string) (*Comment, error)
    DeleteComment(commentID, userID int64) error
    ListComments(articleID int64, page, perPage int) ([]Comment, int, error)
    
    LikeArticle(articleID, userID int64) error
    UnlikeArticle(articleID, userID int64) error
    IsLiked(articleID, userID int64) (bool, error)
    GetLikesCount(articleID int64) (int64, error)
    GetCommentsCount(articleID int64) (int64, error)
    BatchGetInteractions(articleIDs []int64, userID int64) (map[int64]InteractionInfo, error)
}
```

## 二、评论内容安全

### 简单过滤规则（不依赖外部 AI）
- 长度限制：1-500 字符
- 去除首尾空白
- 过滤纯空白/纯符号评论
- 不做敏感词过滤（初期阶段，信任用户）
- XSS 防护：前端展示时 escape HTML

## 三、前端改造

### article.html 评论区域

在正文预览下方添加评论区：

```
┌─────────────────────────────────────┐
│ ❤️ 15 人点赞    💬 8 条评论          │
├─────────────────────────────────────┤
│                                     │
│ 👤 匿名用户 · 2小时前               │
│ 这篇文章分析得很到位，特别是...      │
│                                     │
│ 👤 匿名用户 · 5小时前               │
│ 同意楼上的观点，补充一下...          │
│                                     │
├─────────────────────────────────────┤
│ [发表评论...]                        │
│ [提交]                              │
└─────────────────────────────────────┘
```

### index.html 卡片互动

文章卡片底部增加互动信息：
- `❤️ 15 · 💬 8`（点赞数 + 评论数）
- 点赞按钮（小爱心图标，已点赞时变红）
- 评论数点击跳转到详情页评论区

### 分享功能

在文章详情页添加分享按钮：
- 复制链接（`navigator.clipboard.writeText`）
- 分享到 Twitter（`https://twitter.com/intent/tweet?text=...&url=...`）
- 分享到微博（`https://service.weibo.com/share/share.php?...`）

分享按钮不需要后端支持，纯前端实现。

## 四、用户画像更新

评论和点赞行为也应该更新用户画像：
- 评论的文章 → 标签权重 +0.15（比阅读更高，说明用户有深度参与）
- 点赞的文章 → 标签权重 +0.1

## 五、看板增强

Dashboard 新增互动数据：
- 总评论数
- 总点赞数
- 最热文章 TOP5（按互动数排序）
- 今日新增评论数

## 实现文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/store/sqlite.go` | 修改 | 新增 comments/likes 表 |
| `internal/store/comment.go` | **新增** | 评论+点赞 CRUD |
| `internal/api/handler_comment.go` | **新增** | 评论/点赞 API |
| `internal/api/server.go` | 修改 | 注册评论路由 |
| `internal/store/user.go` | 修改 | 评论/点赞时触发画像更新 |
| `internal/store/dashboard.go` | 修改 | 新增互动统计查询 |
| `internal/api/handler_dashboard.go` | 修改 | Dashboard 增加互动数据 |
| `internal/static/article.html` | 修改 | 评论区 + 点赞 + 分享按钮 |
| `internal/static/index.html` | 修改 | 卡片互动信息 + 点赞 |
| `internal/static/css/style.css` | 修改 | 评论/点赞/分享样式 |
| `internal/static/js/app.js` | 修改 | 评论交互 + 点赞交互 |

## 注意事项

- 评论和点赞都需要 X-User-Token
- 无 Token 时可以看到评论和点赞数，但不能操作
- 匿名用户显示"匿名用户"而非 ID
- 评论按时间倒序排列（最新在前）
- 删除评论时验证 user_id（只能删自己的）
- 分享功能纯前端，不需要后端 API
- 更新 README.md

## 验证步骤

1. 编译：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go build -ldflags "-X main.version=1.2.0" -o ai-news-hub .`
2. 运行测试：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go test ./...`
3. 提交并推送：`cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && git add -A && git commit -m "v1.2.0: 评论点赞 + 社交互动 + 分享" && git push`
