package collector

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// ---------------------------------------------------------------------------
// 公共 Article 结构（采集结果统一格式）
// ---------------------------------------------------------------------------

// Article 采集结果统一数据结构。
type Article struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Summary     string `json:"summary,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	SourceName  string `json:"source_name"`
	SourceURL   string `json:"source_url"`
	Language    string `json:"language"`
}

// CollectResult 单次采集结果。
type CollectResult struct {
	Source   Source
	Articles []Article
	Err      error
}

// ---------------------------------------------------------------------------
// RSS XML 结构定义（同时支持 RSS 2.0 和 Atom）
// ---------------------------------------------------------------------------

// rssFeed 表示 RSS 2.0 格式。
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// atomFeed 表示 Atom 格式。
type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Link    []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	ID        string     `xml:"id"`
	Updated   string     `xml:"updated"`
	Published string     `xml:"published"`
	Link      []atomLink `xml:"link"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
}

// ---------------------------------------------------------------------------
// RSSCollector RSS/Atom 采集器
// ---------------------------------------------------------------------------

// RSSCollector 并发 RSS 采集引擎。
type RSSCollector struct {
	maxWorkers   int
	userAgent    string
	fetchTimeout time.Duration
}

// RSSOption 采集器配置选项。
type RSSOption func(*RSSCollector)

// WithMaxWorkers 设置 worker pool 并发度。
func WithMaxWorkers(n int) RSSOption {
	return func(c *RSSCollector) {
		if n > 0 {
			c.maxWorkers = n
		}
	}
}

// WithUserAgent 设置 HTTP User-Agent。
func WithUserAgent(ua string) RSSOption {
	return func(c *RSSCollector) {
		c.userAgent = ua
	}
}

// WithFetchTimeout 设置单源采集超时。
func WithFetchTimeout(d time.Duration) RSSOption {
	return func(c *RSSCollector) {
		c.fetchTimeout = d
	}
}

// NewRSSCollector 创建 RSS 采集器实例。
func NewRSSCollector(opts ...RSSOption) *RSSCollector {
	c := &RSSCollector{
		maxWorkers:   5,
		userAgent:    "ai-news-hub/1.0",
		fetchTimeout: 15 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// CollectAll 并发采集所有 RSS 源，返回采集结果列表。
func (c *RSSCollector) CollectAll(sources []Source) []CollectResult {
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

// collectOne 采集单个 RSS 源（含重试，最多重试 1 次）。
func (c *RSSCollector) collectOne(src Source) CollectResult {
	result := CollectResult{Source: src}

	for attempt := 0; attempt <= 1; attempt++ {
		articles, err := c.fetchAndParse(src)
		if err == nil {
			result.Articles = articles
			return result
		}
		if attempt == 0 {
			slog.Warn("rss fetch failed, retrying",
				"source", src.Name, "url", src.URL,
				"attempt", attempt+1, "error", err,
			)
		} else {
			slog.Error("rss fetch failed after retry",
				"source", src.Name, "url", src.URL, "error", err,
			)
			result.Err = err
		}
	}
	return result
}

// fetchAndParse 获取并解析单个 RSS 源。
func (c *RSSCollector) fetchAndParse(src Source) ([]Article, error) {
	httpClient := &http.Client{Timeout: c.fetchTimeout}

	req, err := http.NewRequest("GET", src.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, */*")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// 确保 UTF-8 编码（自动处理 GBK/GB2312）
	bodyBytes = ensureUTF8(bodyBytes, resp.Header.Get("Content-Type"))

	// 尝试 RSS 2.0 解析
	var rssDoc rssFeed
	if err := xml.Unmarshal(bodyBytes, &rssDoc); err == nil && len(rssDoc.Channel.Items) > 0 {
		return c.convertRSSItems(rssDoc.Channel.Items, src), nil
	}

	// 尝试 Atom 解析
	var atomDoc atomFeed
	if err := xml.Unmarshal(bodyBytes, &atomDoc); err == nil && len(atomDoc.Entries) > 0 {
		return c.convertAtomEntries(atomDoc.Entries, src), nil
	}

	return nil, fmt.Errorf("no parseable RSS or Atom content in %s", src.URL)
}

// ---------------------------------------------------------------------------
// 编码处理
// ---------------------------------------------------------------------------

// ensureUTF8 确保字节切片为合法 UTF-8 编码。
// 优先使用 charset.NewReader 自动检测编码（支持 GBK/GB2312/Big5 等），
// 回退到手动 GBK 解码。
func ensureUTF8(data []byte, contentType string) []byte {
	if utf8.Valid(data) {
		return data
	}

	// 方法 1：golang.org/x/net/html/charset 自动检测
	reader := bytes.NewReader(data)
	utf8Reader, err := charset.NewReader(reader, contentType)
	if err == nil {
		converted, readErr := io.ReadAll(utf8Reader)
		if readErr == nil && len(converted) > 0 && !bytes.Equal(converted, data) {
			return converted
		}
	}

	// 方法 2：手动尝试 GBK → UTF-8
	return tryGBK(data)
}

// tryGBK 手动尝试 GBK → UTF-8 转换。
func tryGBK(data []byte) []byte {
	decoder := simplifiedchinese.GBK.NewDecoder()
	converted, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), decoder))
	if err != nil {
		return data // 转换失败返回原始数据
	}
	return converted
}

// ---------------------------------------------------------------------------
// RSS/Atom → Article 转换
// ---------------------------------------------------------------------------

// convertRSSItems 将 RSS 2.0 items 转换为统一 Article 格式。
func (c *RSSCollector) convertRSSItems(items []rssItem, src Source) []Article {
	articles := make([]Article, 0, len(items))
	for _, item := range items {
		url := item.Link
		if url == "" {
			url = item.GUID
		}
		if url == "" {
			continue
		}
		articles = append(articles, Article{
			Title:       cleanText(item.Title),
			URL:         url,
			Summary:     cleanHTML(item.Description),
			PublishedAt: parseTime(item.PubDate),
			SourceName:  src.Name,
			SourceURL:   src.URL,
			Language:    src.Language,
		})
	}
	return articles
}

// convertAtomEntries 将 Atom entries 转换为统一 Article 格式。
func (c *RSSCollector) convertAtomEntries(entries []atomEntry, src Source) []Article {
	articles := make([]Article, 0, len(entries))
	for _, entry := range entries {
		url := ""
		for _, link := range entry.Link {
			if link.Rel == "alternate" || link.Rel == "" {
				url = link.Href
				break
			}
		}
		if url == "" && len(entry.Link) > 0 {
			url = entry.Link[0].Href
		}
		if url == "" {
			url = entry.ID
		}
		if url == "" {
			continue
		}

		summary := entry.Summary
		if summary == "" {
			summary = entry.Content
		}
		published := entry.Published
		if published == "" {
			published = entry.Updated
		}

		articles = append(articles, Article{
			Title:       cleanText(entry.Title),
			URL:         url,
			Summary:     cleanHTML(summary),
			PublishedAt: parseTime(published),
			SourceName:  src.Name,
			SourceURL:   src.URL,
			Language:    src.Language,
		})
	}
	return articles
}

// ---------------------------------------------------------------------------
// 文本处理工具函数
// ---------------------------------------------------------------------------

// cleanText 清理文本中的多余空白和换行。
func cleanText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

// cleanHTML 去除简单 HTML 标签，返回纯文本摘要（截取前 500 字符）。
func cleanHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				result.WriteRune(r)
			}
		}
	}
	text := cleanText(result.String())
	if len(text) > 500 {
		text = text[:500] + "..."
	}
	return text
}

// parseTime 尝试解析多种时间格式，返回统一 ISO 8601 格式字符串。
func parseTime(s string) string {
	if s == "" {
		return ""
	}
	formats := []string{
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339,
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
		"January 02, 2006",
		"Jan 02, 2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, strings.TrimSpace(s)); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return s
}
