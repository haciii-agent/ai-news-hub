package collector

// ---------------------------------------------------------------------------
// Source 代表一个数据源
// ---------------------------------------------------------------------------

// Source 代表一个数据源。
type Source struct {
	Name       string   `json:"name" yaml:"name"`
	URL        string   `json:"url" yaml:"url"`
	Type       string   `json:"type" yaml:"type"` // "rss" | "html"
	Categories []string `json:"categories" yaml:"categories"`
	Language   string   `json:"language" yaml:"language"`
}

// ---------------------------------------------------------------------------
// RSS 数据源注册表
// ---------------------------------------------------------------------------

// DefaultRSSSources 12 个 RSS/Atom 数据源注册表。
var DefaultRSSSources = []Source{
	{
		Name:       "Hacker News",
		URL:        "https://hnrss.org/frontpage",
		Type:       "rss",
		Categories: []string{"科技前沿", "开源生态"},
		Language:   "en",
	},
	{
		Name:       "TechCrunch AI",
		URL:        "https://techcrunch.com/category/artificial-intelligence/feed/",
		Type:       "rss",
		Categories: []string{"AI/ML", "商业动态"},
		Language:   "en",
	},
	{
		Name:       "The Verge AI",
		URL:        "https://www.theverge.com/rss/ai-artificial-intelligence/index.xml",
		Type:       "rss",
		Categories: []string{"AI/ML", "科技前沿", "产品发布"},
		Language:   "en",
	},
	{
		Name:       "OpenAI Blog",
		URL:        "https://openai.com/blog/rss/",
		Type:       "rss",
		Categories: []string{"AI/ML", "产品发布"},
		Language:   "en",
	},
	{
		Name:       "Google AI Blog",
		URL:        "https://blog.google/technology/ai/rss/",
		Type:       "rss",
		Categories: []string{"AI/ML", "学术研究"},
		Language:   "en",
	},
	{
		Name:       "MIT Tech Review",
		URL:        "https://www.technologyreview.com/feed/",
		Type:       "rss",
		Categories: []string{"AI/ML", "科技前沿", "学术研究"},
		Language:   "en",
	},
	{
		Name:       "Ars Technica",
		URL:        "https://feeds.arstechnica.com/arstechnica/index",
		Type:       "rss",
		Categories: []string{"AI/ML", "科技前沿", "政策监管"},
		Language:   "en",
	},
	{
		Name:       "HuggingFace Blog",
		URL:        "https://huggingface.co/blog/feed.xml",
		Type:       "rss",
		Categories: []string{"AI/ML", "开源生态"},
		Language:   "en",
	},
	{
		Name:       "36氪",
		URL:        "https://36kr.com/feed",
		Type:       "rss",
		Categories: []string{"商业动态", "AI/ML", "科技前沿"},
		Language:   "zh",
	},
	{
		Name:       "InfoQ 中文",
		URL:        "https://www.infoq.cn/feed",
		Type:       "rss",
		Categories: []string{"科技前沿", "开源生态"},
		Language:   "zh",
	},
	{
		Name:       "少数派",
		URL:        "https://sspai.com/feed",
		Type:       "rss",
		Categories: []string{"科技前沿", "产品发布"},
		Language:   "zh",
	},
	{
		Name:       "极客公园",
		URL:        "https://geekpark.net/rss",
		Type:       "rss",
		Categories: []string{"商业动态", "AI/ML", "科技前沿"},
		Language:   "zh",
	},
}

// ---------------------------------------------------------------------------
// HTML 降级数据源注册表（T003 扩展）
// ---------------------------------------------------------------------------

// DefaultHTMLSources HTML 降级采集数据源注册表。
// 这些站点没有稳定的 RSS feed，需要 HTML 列表页抓取。
var DefaultHTMLSources = []Source{
	{
		Name:       "机器之心",
		URL:        "https://www.jiqizhixin.com/",
		Type:       "html",
		Categories: []string{"AI/ML", "科技前沿", "学术研究"},
		Language:   "zh",
	},
	{
		Name:       "量子位",
		URL:        "https://www.qbitai.com/",
		Type:       "html",
		Categories: []string{"AI/ML", "科技前沿", "商业动态"},
		Language:   "zh",
	},
}

// ---------------------------------------------------------------------------
// Source 查询函数
// ---------------------------------------------------------------------------

// RSSSources 返回所有 type=rss 的数据源。
func RSSSources() []Source {
	sources := make([]Source, 0, len(DefaultRSSSources))
	for _, s := range DefaultRSSSources {
		if s.Type == "rss" {
			sources = append(sources, s)
		}
	}
	return sources
}

// HTMLSources 返回所有 type=html 的数据源。
func HTMLSources() []Source {
	sources := make([]Source, 0, len(DefaultHTMLSources))
	for _, s := range DefaultHTMLSources {
		if s.Type == "html" {
			sources = append(sources, s)
		}
	}
	return sources
}

// AllSources 返回所有数据源（RSS + HTML）。
func AllSources() []Source {
	rss := RSSSources()
	html := HTMLSources()
	all := make([]Source, 0, len(rss)+len(html))
	all = append(all, rss...)
	all = append(all, html...)
	return all
}

// ---------------------------------------------------------------------------
// 默认解析器注册表
// ---------------------------------------------------------------------------

// DefaultHTMLParsers 返回 HTML 源对应的默认解析器集合。
func DefaultHTMLParsers() map[string]HTMLParser {
	return map[string]HTMLParser{
		"机器之心": NewJiqizhixinParser(),
		"量子位":  &QbitaiParser{},
	}
}
