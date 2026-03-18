package collector

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// ---------------------------------------------------------------------------
// Readability 正文提取器
// ---------------------------------------------------------------------------

// AllowedTags 保留的标签白名单。
var allowedTags = map[string]bool{
	"p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"img": true, "a": true, "blockquote": true, "ul": true, "ol": true, "li": true,
	"strong": true, "em": true, "br": true, "hr": true, "pre": true, "code": true,
	"table": true, "thead": true, "tbody": true, "tr": true, "th": true, "td": true,
	"figure": true, "figcaption": true, "span": true, "div": true,
}

// RemoveTags 需要移除的标签（含子树）。
var removeTags = map[string]bool{
	"script": true, "style": true, "nav": true, "footer": true, "header": true,
	"aside": true, "iframe": true, "noscript": true, "svg": true, "form": true,
}

// RemoveClassPatterns class/id 中包含这些关键词的元素需要移除。
var removeClassPatterns = []string{
	"sidebar", "comment", "ad-", "social", "share", "related", "newsletter",
	"advertisement", "footer", "header", "nav", "popup", "modal", "cookie",
	"banner", "sponsor", "promotion", "widget",
}

// ExtractedContent Readability 提取结果。
type ExtractedContent struct {
	HTML    string // 提取的正文 HTML
	Title   string // 从页面提取的标题
	BaseURL string // 原文 base URL
}

// FetchAndExtract 抓取页面并提取正文（HTTP + Readability）。
func FetchAndExtract(pageURL string, timeout time.Duration) (*ExtractedContent, error) {
	// SSRF 防护：只允许 http/https
	u, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported protocol: %s", u.Scheme)
	}

	// HTTP 抓取
	httpClient := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", randomUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

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

	if !bytes.HasPrefix(body, []byte{0xEF, 0xBB, 0xBF}) {
		body = ensureUTF8(body, resp.Header.Get("Content-Type"))
	}

	// 提取正文
	return ExtractFromBody(body, pageURL)
}

// ExtractFromBody 从 HTML 字节中提取正文。
func ExtractFromBody(body []byte, pageURL string) (*ExtractedContent, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	baseURL, _ := url.Parse(pageURL)

	// 提取页面标题
	title := extractTitle(doc)

	// 按优先级查找正文容器
	candidate := findContentCandidate(doc)

	// 从候选节点中提取正文 HTML
	contentHTML := extractCleanHTML(candidate, baseURL)

	return &ExtractedContent{
		HTML:    contentHTML,
		Title:   title,
		BaseURL: pageURL,
	}, nil
}

// extractTitle 从 HTML 中提取页面标题。
func extractTitle(doc *html.Node) string {
	var walk func(*html.Node)
	var title string
	walk = func(n *html.Node) {
		if title != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "title" {
			title = extractText(n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return title
}

// findContentCandidate 按优先级查找正文内容候选节点。
// 策略：article → main → role=main → class启发式 → 文本密度 → body回退
func findContentCandidate(doc *html.Node) *html.Node {
	// 策略 1: <article> 标签
	if node := findFirstTag(doc, "article"); node != nil {
		return node
	}

	// 策略 2: <main> 标签
	if node := findFirstTag(doc, "main"); node != nil {
		return node
	}

	// 策略 3: role="main"
	if node := findRoleMain(doc); node != nil {
		return node
	}

	// 策略 4: class 启发式
	if node := findClassHeuristic(doc); node != nil {
		return node
	}

	// 策略 5: 文本密度最高
	if node := findBestDensityNode(doc); node != nil {
		return node
	}

	// 策略 6: body 回退
	return findFirstTag(doc, "body")
}

// findFirstTag 查找 DOM 中第一个指定标签。
func findFirstTag(doc *html.Node, tag string) *html.Node {
	var result *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if result != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == tag {
			result = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return result
}

// findRoleMain 查找 role="main" 的元素。
func findRoleMain(doc *html.Node) *html.Node {
	var result *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if result != nil {
			return
		}
		if n.Type == html.ElementNode && getAttr(n, "role") == "main" {
			result = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return result
}

// findClassHeuristic 查找 class 含 content/article/post/entry 的 <div>。
func findClassHeuristic(doc *html.Node) *html.Node {
	keywords := []string{"content", "article", "post", "entry", "story", "text", "body"}
	var best *html.Node
	var bestScore int

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			class := strings.ToLower(getAttr(n, "class") + " " + getAttr(n, "id"))
			score := 0
			for _, kw := range keywords {
				if strings.Contains(class, kw) {
					score++
				}
			}
			// 额外加分：不含 noise 关键词
			if !hasClassNoise(class) {
				score += 2
			}
			if score > bestScore {
				bestScore = score
				best = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return best
}

// hasClassNoise 检查 class 是否含 noise 关键词。
func hasClassNoise(class string) bool {
	for _, pattern := range removeClassPatterns {
		if strings.Contains(class, pattern) {
			return true
		}
	}
	return false
}

// densityNode 记录文本密度最高的节点。
type densityNode struct {
	node       *html.Node
	textLen    int
	totalLen   int
	density    float64
}

// findBestDensityNode 计算每个 div 的文本密度，取密度最高且文本量足够的。
func findBestDensityNode(doc *html.Node) *html.Node {
	var candidates []densityNode

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "div", "section", "article":
				class := strings.ToLower(getAttr(n, "class") + " " + getAttr(n, "id"))
				// 跳过明显的非正文容器
				if hasClassNoise(class) {
					// 不遍历子节点（优化性能）
					return
				}

				text, total := calcNodeText(n)
				if total > 200 { // 忽略太小的容器
					density := float64(text) / float64(total)
					candidates = append(candidates, densityNode{
						node:     n,
						textLen:  text,
						totalLen: total,
						density:  density,
					})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if len(candidates) == 0 {
		return nil
	}

	// 按文本量 × 密度 综合排序
	best := candidates[0]
	for _, c := range candidates[1:] {
		score := float64(c.textLen) * c.density
		bestScore := float64(best.textLen) * best.density
		if score > bestScore {
			best = c
		}
	}
	return best.node
}

// calcNodeText 计算节点内的纯文本长度和总字符数（含 HTML 标签）。
func calcNodeText(n *html.Node) (textLen, totalLen int) {
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			textLen += len(strings.TrimSpace(node.Data))
			totalLen += len(node.Data)
		} else if node.Type == html.ElementNode {
			totalLen += len(node.Data) + 2 // 粗略计算标签长度
			for _, attr := range node.Attr {
				totalLen += len(attr.Key) + len(attr.Val) + 4
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return
}

// extractCleanHTML 从候选节点中提取干净的 HTML。
func extractCleanHTML(node *html.Node, baseURL *url.URL) string {
	// 预处理：移除噪音节点
	cleaned := cleanNode(node)

	// 序列化为 HTML
	var buf bytes.Buffer
	renderFiltered(&buf, cleaned, baseURL)
	result := buf.String()

	// 清理多余空行
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	result = strings.TrimSpace(result)

	// 如果结果太短或为空，返回空
	if len(result) < 20 {
		return ""
	}

	return result
}

// cleanNode 深度复制并移除噪音节点（原地修改）。
func cleanNode(node *html.Node) *html.Node {
	// 移除需删除的标签和含 noise class 的元素
	removeNoiseNodes(node)
	return node
}

// removeNoiseNodes 递归移除噪音节点。
func removeNoiseNodes(node *html.Node) {
	if node.Type == html.ElementNode {
		// 移除特定标签
		if removeTags[node.Data] {
			node.Parent.RemoveChild(node)
			return
		}

		// 检查 class/id noise
		class := strings.ToLower(getAttr(node, "class") + " " + getAttr(node, "id"))
		if node.Data == "div" || node.Data == "section" || node.Data == "span" {
			if hasClassNoise(class) {
				if node.Parent != nil {
					node.Parent.RemoveChild(node)
					return
				}
			}
		}
	}

	// 递归处理子节点
	for c := node.FirstChild; c != nil; {
		next := c.NextSibling
		removeNoiseNodes(c)
		c = next
	}
}

// renderFiltered 渲染节点为 HTML，只保留白名单标签。
func renderFiltered(buf *bytes.Buffer, node *html.Node, baseURL *url.URL) {
	if node == nil {
		return
	}

	switch node.Type {
	case html.TextNode:
		text := node.Data
		// 合并空白
		text = strings.ReplaceAll(text, "\t", " ")
		text = collapseSpaces(text)
		buf.WriteString(htmlEscape(text))

	case html.ElementNode:
		if !allowedTags[node.Data] {
			// 标签不在白名单中，但保留子内容
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				renderFiltered(buf, c, baseURL)
			}
			return
		}

		// 特殊处理自闭合标签
		switch node.Data {
		case "br":
			buf.WriteString("<br>")
			return
		case "hr":
			buf.WriteString("<hr>")
			return
		case "img":
			src := getAttr(node, "src")
			if src == "" {
				src = getAttr(node, "data-src") // lazy load
			}
			if src != "" && !strings.HasPrefix(src, "data:") {
				src = resolveURL(baseURL, src)
			}
			alt := getAttr(node, "alt")
			buf.WriteString(fmt.Sprintf(`<img src="%s" alt="%s" loading="lazy">`, htmlAttrEscape(src), htmlAttrEscape(alt)))
			return
		}

		// 开标签
		buf.WriteByte('<')
		buf.WriteString(node.Data)

		// 输出允许的属性
		for _, attr := range node.Attr {
			switch attr.Key {
			case "href":
				if node.Data == "a" {
					href := attr.Val
					if href != "" {
						// 转为绝对路径
						href = resolveURL(baseURL, href)
						buf.WriteString(` href="`)
						buf.WriteString(htmlAttrEscape(href))
						buf.WriteString(`" target="_blank" rel="noopener"`)
					}
				}
			case "src":
				if node.Data == "img" {
					continue // img 已处理
				}
			}
		}
		buf.WriteByte('>')

		// 子节点
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			renderFiltered(buf, c, baseURL)
		}

		// 闭标签（自闭合标签已处理）
		buf.WriteString("</")
		buf.WriteString(node.Data)
		buf.WriteByte('>')
	}

	// CommentNode 和 DoctypeNode 等忽略
}

// htmlEscape 转义 HTML 特殊字符（文本内容）。
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// htmlAttrEscape 转义 HTML 属性值。
func htmlAttrEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// collapseSpaces 合并连续空白字符。
func collapseSpaces(s string) string {
	var buf strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\r' {
			if !prevSpace {
				buf.WriteRune(' ')
				prevSpace = true
			}
		} else {
			buf.WriteRune(r)
			prevSpace = false
		}
	}
	return buf.String()
}
