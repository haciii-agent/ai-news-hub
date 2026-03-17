# v0.3.0 优化任务

## 背景
v0.2.0 已完成基础功能，以下是下一轮迭代的优化点。

## 任务1：版本号同步
health 接口仍返回 "version": "0.1.0"，改为从 go build 注入版本号。

### 修改：main.go
在 main() 中增加 version 变量：
```go
var version = "dev" // 通过 -ldflags "-X main.version=x.y.z" 注入
```
传递给 api.NewServer 或直接在 healthHandler 中使用。

### 修改：internal/api/server.go
Server struct 添加 Version string 字段，healthHandler 中使用 s.Version。

### 修改：Makefile
build target 加入 ldflags：
```makefile
build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o ai-news-hub .
```

## 任务2：store 和 collector 单元测试

### 新文件：internal/store/article_test.go
- TestBatchInsertArticles: 批量插入+去重
- TestQueryArticlesWithFilter: 分类/语言/分页筛选
- TestDeleteArticlesBefore: 清理旧文章

### 新文件：internal/collector/rss_test.go  
- TestCleanHTML: HTML 标签清理
- TestParseTime: 多种时间格式解析
- TestConvertRSSItems: RSS 转 Article

## 任务3：OpenClaw Cron 集成配置示例

### 新文件：cron-example.md
写出 OpenClaw cron job 配置示例，用户可以直接复制配置。
每天 08:00 和 20:00 触发 POST http://localhost:8080/api/v1/collect。

## 任务4：Dockerfile 验证修复
检查 Dockerfile 是否能正确构建（不需要真的 build，但确保语法和步骤正确）。

---

编译验证：export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go build -o ai-news-hub . && go test ./...
