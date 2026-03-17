package collector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// ---------------------------------------------------------------------------
// HTML Parser 接口 — 每个站点实现自己的解析逻辑
// ---------------------------------------------------------------------------

// HTMLParser 定义单个 HTML 源的解析策略。
// 不同站点 DOM 结构各异，每个源需注册各自的解析器。
type HTMLParser interface {
	// Parse 从 HTML 文档中提取文章列表。
	Parse(pageURL string, body []byte) ([]Article, error)
}

// HTMLParserFunc 函数类型的 HTMLParser 便捷适配器。
type HTMLParserFunc func(pageURL string, body []byte) ([]Article, error)

func (f HTMLParserFunc) Parse(pageURL string, body []byte) ([]Article, error) {
	return f(pageURL, body)
}

// ---------------------------------------------------------------------------
// HTMLCollector HTML 降级采集器
// ---------------------------------------------------------------------------

// HTMLCollector 并发 HTML 采集引擎，用于无稳定 RSS 的数据源。
type HTMLCollector struct {
	maxWorkers   int
	fetchTimeout time.Duration
	parsers      map[string]HTMLParser // source name → parser
}

// HTMLOption HTML 采集器配置选项。
type HTMLOption func(*HTMLCollector)

// WithHTMLMaxWorkers 设置 worker pool 并发度。
func WithHTMLMaxWorkers(n int) HTMLOption {
	return func(c *HTMLCollector) {
		if n > 0 {
			c.maxWorkers = n
		}
	}
}

// WithHTMLFetchTimeout 设置单源采集超时。
func WithHTMLFetchTimeout(d time.Duration) HTMLOption {
	return func(c *HTMLCollector) {
		c.fetchTimeout = d
	}
}

// WithHTMLParser 为指定源注册解析器。
func WithHTMLParser(sourceName string, parser HTMLParser) HTMLOption {
	return func(c *HTMLCollector) {
		if c.parsers == nil {
			c.parsers = make(map[string]HTMLParser)
		}
		c.parsers[sourceName] = parser
	}
}

// NewHTMLCollector 创建 HTML 采集器实例。
func NewHTMLCollector(opts ...HTMLOption) *HTMLCollector {
	c := &HTMLCollector{
		maxWorkers:   3,
		fetchTimeout: 20 * time.Second,
		parsers:      make(map[string]HTMLParser),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// CollectAll 并发采集所有 HTML 源，返回采集结果列表。
func (c *HTMLCollector) CollectAll(sources []Source) []CollectResult {
	if len(sources) == 0 {
		return nil
	}

	jobs := make(chan Source, len(sources))
	results := make(chan CollectResult, len(sources))

	var wg sync.WaitGroup
	for i := 0; i < c.maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for src := range jobs {
				// 采集间隔：2-5 秒随机延迟（反爬应对）
				delay := time.Duration(2000+rand.Intn(3000)) * time.Millisecond
				time.Sleep(delay)

				results <- c.collectOne(src)
			}
		}()
	}

	for _, src := range sources {
		jobs <- src
	}
	close(jobs)

	wg.Wait()
	close(results)

	var all []CollectResult
	for r := range results {
		all = append(all, r)
	}
	return all
}

// collectOne 采集单个 HTML 源。
func (c *HTMLCollector) collectOne(src Source) CollectResult {
	result := CollectResult{Source: src}

	parser, ok := c.parsers[src.Name]
	if !ok {
		parser = &GenericHTMLParser{} // 回退到通用解析器
	}

	body, err := c.fetchPage(src.URL)
	if err != nil {
		slog.Error("html fetch failed",
			"source", src.Name, "url", src.URL, "error", err,
		)
		result.Err = err
		return result
	}

	articles, err := parser.Parse(src.URL, body)
	if err != nil {
		slog.Error("html parse failed",
			"source", src.Name, "url", src.URL, "error", err,
		)
		result.Err = err
		return result
	}

	// 统一填充元信息
	for i := range articles {
		articles[i].SourceName = src.Name
		articles[i].SourceURL = src.URL
		if articles[i].Language == "" {
			articles[i].Language = src.Language
		}
	}

	slog.Info("html collection succeeded",
		"source", src.Name, "articles", len(articles),
	)
	result.Articles = articles
	return result
}

// fetchPage 获取页面 HTML 内容。
func (c *HTMLCollector) fetchPage(pageURL string) ([]byte, error) {
	httpClient := &http.Client{Timeout: c.fetchTimeout}

	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", randomUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// 确保合法 UTF-8
	if !bytes.HasPrefix(body, []byte{0xEF, 0xBB, 0xBF}) {
		body = ensureUTF8(body, resp.Header.Get("Content-Type"))
	}

	return body, nil
}

// ---------------------------------------------------------------------------
// 随机 User-Agent 池
// ---------------------------------------------------------------------------

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
}

func randomUserAgent() string {
	return userAgents[rand.Intn(len(userAgents))]
}

// ---------------------------------------------------------------------------
// 通用 HTML 解析器 — 从 HTML 中提取 <a> 链接和标题
// ---------------------------------------------------------------------------

// GenericHTMLParser 通用 HTML 解析器，提取页面中的文章链接和标题。
// 使用 CSS 选择器启发式规则匹配常见文章列表模式。
type GenericHTMLParser struct {
	// ArticleLinksSelector 用于匹配文章链接的 CSS 选择器（目前通过启发式匹配实现）
	// 支持的选择器：a[href]（文章链接）、h2/h3（标题）
}

// Parse 使用 DOM 解析提取文章列表。
func (p *GenericHTMLParser) Parse(pageURL string, body []byte) ([]Article, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var articles []Article
	baseURL, _ := url.Parse(pageURL)

	// 策略：查找包含 <a> 链接的 <h2>/<h3>/<h4> 标题作为文章标题
	// 同时查找独立的文章卡片结构
	seen := make(map[string]bool)

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "h1", "h2", "h3", "h4":
				// 查找标题内的链接
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "a" {
						link := getAttr(c, "href")
						title := extractText(c)
						if link != "" && title != "" && len(title) > 4 {
							fullURL := resolveURL(baseURL, link)
							if !seen[fullURL] {
								seen[fullURL] = true
								articles = append(articles, Article{
									Title: cleanText(title),
									URL:   fullURL,
								})
							}
						}
					}
				}
				// 查找标题旁边的链接（兄弟节点）
				if link := findAdjacentLink(n); link != "" {
					title := extractText(n)
					fullURL := resolveURL(baseURL, link)
					if title != "" && len(title) > 4 && !seen[fullURL] {
						seen[fullURL] = true
						articles = append(articles, Article{
							Title: cleanText(title),
							URL:   fullURL,
						})
					}
				}
			case "a":
				// 查找独立链接（如 WordPress 文章列表中 a 包含在 article/div 内）
				link := getAttr(n, "href")
				title := extractText(n)
				if link != "" && title != "" && len(title) > 8 && looksLikeArticleURL(link) {
					fullURL := resolveURL(baseURL, link)
					if !seen[fullURL] {
						seen[fullURL] = true
						articles = append(articles, Article{
							Title: cleanText(title),
							URL:   fullURL,
						})
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(doc)
	return articles, nil
}

// ---------------------------------------------------------------------------
// 量子位 (qbitai.com) 专用解析器
// ---------------------------------------------------------------------------

// QbitaiParser 量子位专用 HTML 解析器。
// 量子位基于 WordPress，首页 SSR 渲染文章列表，
// 文章结构：h3 > a（标题链接），URL 模式为 /YYYY/MM/NNNNN.html。
type QbitaiParser struct{}

// Parse 从量子位首页 HTML 中提取文章列表。
func (p *QbitaiParser) Parse(pageURL string, body []byte) ([]Article, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var articles []Article
	baseURL, _ := url.Parse(pageURL)
	seen := make(map[string]bool)

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			link := getAttr(n, "href")
			if link == "" {
				goto next
			}

			// 量子位文章 URL 模式：/YYYY/MM/NNNNN.html
			if !isQbitaiArticleURL(link) {
				goto next
			}

			fullURL := resolveURL(baseURL, link)
			if seen[fullURL] {
				goto next
			}
			seen[fullURL] = true

			// 提取标题
			title := extractText(n)
			if title == "" {
				// 尝试从父级 h3 获取标题
				title = findParentHeadingText(n)
			}

			if title != "" && len(title) > 2 {
				articles = append(articles, Article{
					Title: cleanText(title),
					URL:   fullURL,
				})
			}
		}
	next:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(doc)
	return articles, nil
}

// isQbitaiArticleURL 判断是否为量子位文章链接。
func isQbitaiArticleURL(link string) bool {
	return strings.Contains(link, "qbitai.com") &&
		strings.Contains(link, "/20") &&
		strings.HasSuffix(link, ".html")
}

// ---------------------------------------------------------------------------
// 机器之心 (jiqizhixin.com) 专用解析器
// ---------------------------------------------------------------------------

// JiqizhixinParser 机器之心专用解析器。
// 机器之心首页为 React SPA，但 SSR 渲染了部分内容（如专题/推荐文章链接），
// 同时其 JSON API (/api/v1/articles) 可返回文章标题和描述。
// 策略：优先尝试 JSON API，回退到 HTML 解析。
type JiqizhixinParser struct {
	httpClient *http.Client
}

// NewJiqizhixinParser 创建机器之心解析器。
func NewJiqizhixinParser() *JiqizhixinParser {
	return &JiqizhixinParser{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// jiqizhixinArticle 机器之心 API 返回的文章结构。
type jiqizhixinArticle struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Slug        string `json:"slug"`
}

// Parse 从机器之心页面中提取文章列表。
// 优先尝试 JSON API，回退到 HTML 解析。
func (p *JiqizhixinParser) Parse(pageURL string, body []byte) ([]Article, error) {
	// 策略 1：尝试 JSON API
	apiURL := "https://www.jiqizhixin.com/api/v1/articles?limit=20"
	apiArticles, err := p.fetchAPI(apiURL)
	if err != nil {
		slog.Debug("jiqizhixin API failed, falling back to HTML", "error", err)
	} else if len(apiArticles) > 0 {
		return apiArticles, nil
	}

	// 策略 2：从 HTML 中提取文章链接
	return p.parseHTML(pageURL, body)
}

func (p *JiqizhixinParser) fetchAPI(apiURL string) ([]Article, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", randomUserAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	var rawArticles []jiqizhixinArticle
	if err := json.NewDecoder(resp.Body).Decode(&rawArticles); err != nil {
		return nil, err
	}

	var articles []Article
	for _, a := range rawArticles {
		// 跳过无效条目
		if a.Title == "" || strings.HasPrefix(a.Title, "title-") {
			continue
		}

		// API 不返回完整 URL，尝试从 slug 构造
		articleURL := a.URL
		if articleURL == "" && a.Slug != "" {
			articleURL = "https://www.jiqizhixin.com/articles/" + a.Slug
		}
		if articleURL == "" {
			continue
		}

		articles = append(articles, Article{
			Title:   cleanText(a.Title),
			URL:     articleURL,
			Summary: cleanText(a.Description),
		})
	}
	return articles, nil
}

func (p *JiqizhixinParser) parseHTML(pageURL string, body []byte) ([]Article, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var articles []Article
	baseURL, _ := url.Parse(pageURL)
	seen := make(map[string]bool)

	// 机器之心 SSR 中包含部分文章链接，从 data-react-props 和 href 中提取
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			link := getAttr(n, "href")
			if link == "" {
				goto next
			}

			// 机器之心文章链接特征
			if !isJiqizhixinArticleURL(link) {
				goto next
			}

			fullURL := resolveURL(baseURL, link)
			if seen[fullURL] {
				goto next
			}
			seen[fullURL] = true

			title := extractText(n)
			if title == "" {
				title = findParentHeadingText(n)
			}

			if title != "" && len(title) > 2 {
				articles = append(articles, Article{
					Title: cleanText(title),
					URL:   fullURL,
				})
			}
		}
	next:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(doc)
	return articles, nil
}

// isJiqizhixinArticleURL 判断是否为机器之心文章链接。
func isJiqizhixinArticleURL(link string) bool {
	return strings.Contains(link, "jiqizhixin.com") &&
		(strings.Contains(link, "/articles/") ||
			strings.Contains(link, "/pro/"))
}

// ---------------------------------------------------------------------------
// DOM 辅助函数
// ---------------------------------------------------------------------------

// getAttr 获取元素的指定属性值。
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// extractText 递归提取元素内的纯文本内容。
func extractText(n *html.Node) string {
	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			buf.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(buf.String())
}

// findParentHeadingText 查找父级中最近的 h1-h4 标题文本。
func findParentHeadingText(n *html.Node) string {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode {
			switch p.Data {
			case "h1", "h2", "h3", "h4":
				return extractText(p)
			}
		}
	}
	return ""
}

// findAdjacentLink 查找节点附近（父级或兄弟）的链接。
func findAdjacentLink(n *html.Node) string {
	// 检查父级的第一个子 a
	for p := n.Parent; p != nil; p = p.Parent {
		for c := p.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "a" {
				link := getAttr(c, "href")
				if link != "" && looksLikeArticleURL(link) {
					return link
				}
			}
		}
	}
	return ""
}

// resolveURL 将相对 URL 解析为绝对 URL。
func resolveURL(base *url.URL, ref string) string {
	if ref == "" {
		return ""
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(refURL).String()
}

// looksLikeArticleURL 启发式判断链接是否像一篇文章 URL。
func looksLikeArticleURL(link string) bool {
	// 排除明显的非文章链接
	if strings.Contains(link, "#") ||
		strings.Contains(link, "javascript:") ||
		strings.Contains(link, "mailto:") ||
		strings.HasSuffix(link, "/") ||
		strings.HasSuffix(link, ".css") ||
		strings.HasSuffix(link, ".js") ||
		strings.HasSuffix(link, ".png") ||
		strings.HasSuffix(link, ".jpg") {
		return false
	}
	// 文章 URL 通常包含数字、日期或特定路径
	return strings.Contains(link, "/20") || // 包含日期 /2026/03/
		strings.Contains(link, "/article") ||
		strings.Contains(link, "/post") ||
		strings.Contains(link, "/news") ||
		strings.Contains(link, ".html") ||
		strings.Contains(link, "?p=")
}
