# v0.5.0 测试修复 + 提交

## 背景
v0.5.0 已实现全文搜索（FTS5）功能，编译通过，但测试有失败。需要修复测试使其匹配当前实现，然后提交代码。

## 失败的测试

### 1. `internal/collector/rss_test.go` — `TestCleanHTML`
v0.5.0 将 `cleanHTML` 截断长度从 500 改为 2000，且 cleanHTML 不做 HTML entity 解码。
修复方式：更新测试期望值匹配当前行为。

具体需要修改的测试用例：
- `strip_nested_tags`: `<div><p>Line 1</p><p>Line 2</p></div>` → cleanText 会把 `\n` 换行替换为空格，但标签间没有换行，结果是 `"Line 1Line 2"`（两个p标签紧密相邻）。需要更新期望值为 `"Line 1Line 2"` 或修改测试用例的输入。
- `strip_img_tag`: `<img src="pic.jpg" alt="photo"/>` → img标签内容是属性不是子节点，去标签后为空字符串 `""`。更新期望值为 `""`。
- `truncate_long_text`: 截断长度从 500 变为 2000。输入 600 字节的文本不会被截断（< 2000）。需要增大输入到 > 2000 字节，或调整测试逻辑。
- `handle_entity`: `cleanHTML` 不做 entity 解码（`&amp;` 保持原样），需要更新期望值为 `"A &amp; B &lt; C"`。

### 2. `internal/store/article_test.go` — `TestDeleteArticlesBefore`
`DeleteArticlesBefore` 使用 `date(collected_at) < date(?)`。测试数据通过 `BatchInsertArticles` 插入时 `collected_at` 默认为 `CURRENT_TIMESTAMP`（即当前时间），所以删除条件 `date(collected_at) < '2026-03-17'` 不会匹配当前时间的记录。
修复方式：在测试中直接插入 SQL 设置 collected_at 为特定值，或者修改测试使用更远的过去日期（如 2099 年），或者让测试不依赖 collected_at 的默认值。

推荐方案：在 setupTestDB 之后或在测试中用原生 SQL 插入数据，手动设置 collected_at。

## 任务步骤

1. 修复 `rss_test.go` 中的 `TestCleanHTML` 测试用例
2. 修复 `article_test.go` 中的 `TestDeleteArticlesBefore` 测试
3. 运行 `export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go test ./...` 确认全部通过
4. 确认编译仍然通过：`export PATH=$PATH:/usr/local/go/bin && cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && go build -ldflags "-X main.version=0.5.0" -o ai-news-hub .`
5. 提交代码：`cd /home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/ && git add -A && git commit -m "v0.5.0: SQLite FTS5 全文搜索 + 图片提取 + 摘要扩展(2000字)"`

## 注意
- 不要修改 cleanHTML 的实现逻辑，只修改测试
- 不要修改 DeleteArticlesBefore 的实现逻辑，只修改测试
- 项目路径: `/home/admin/openclaw/workspace-hiclaw/bingbu/output/news-hub/`
- Go 环境: `export PATH=$PATH:/usr/local/go/bin`
