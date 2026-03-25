package wechat

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"ai-news-hub/config"
	"ai-news-hub/internal/store"
)

// Publisher selects top articles and publishes to WeChat.
type Publisher struct {
	client  *Client
	articleStore store.ArticleStore
}

// NewPublisher creates a Publisher.
func NewPublisher(cfg config.WeChatConfig, articleStore store.ArticleStore) *Publisher {
	return &Publisher{
		client:  NewClient(cfg),
		articleStore: articleStore,
	}
}

// Available returns true if publishing is possible.
func (p *Publisher) Available() bool {
	return p.client != nil && p.client.Available()
}

// SelectTopArticles returns the top N articles by importance score from recent collection.
func (p *Publisher) SelectTopArticles(limit int) ([]store.Article, error) {
	// Get recent articles sorted by collected_at desc (use Sort field not SortBy/SortOrder)
	articles, _, err := p.articleStore.QueryArticles(store.ArticleFilter{
		Sort:    "collected_at desc",
		Page:    1,
		PerPage: 300,
	})
	if err != nil {
		return nil, fmt.Errorf("query recent articles: %w", err)
	}

	// Filter to recent (last 48 hours)
	cutoff := time.Now().Add(-48 * time.Hour).Format("2006-01-02T15:04:05")
	var recent []store.Article
	for _, a := range articles {
		if a.CollectedAt >= cutoff {
			recent = append(recent, a)
		}
	}

	// Sort by importance score descending
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].ImportanceScore > recent[j].ImportanceScore
	})

	if len(recent) > limit {
		recent = recent[:limit]
	}
	return recent, nil
}

// BuildArticleContent builds HTML content for a WeChat article from a list of news.
func BuildArticleContent(articles []store.Article, title string) string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf(`<h2 style="text-align:center;color:#1a1a1a;font-size:22px;margin:20px 0;">%s</h2>`, title))
	buf.WriteString(fmt.Sprintf(`<p style="color:#888;font-size:14px;margin-bottom:30px;text-align:center;">%s · AI 科技资讯精选</p>`, time.Now().Format("2006-01-02"), "AI News Hub"))

	buf.WriteString(`<hr style="border:none;border-top:1px solid #eee;margin:20px 0;">`)

	for i, a := range articles {
		displayTitle := a.Title
		if a.TranslatedTitle != "" {
			displayTitle = a.TranslatedTitle
		}
		summary := a.AISummary
		if summary == "" {
			summary = a.Summary
		}

		buf.WriteString(fmt.Sprintf(`<h3 style="color:#1a1a1a;font-size:18px;margin:25px 0 10px;">%d. %s</h3>`, i+1, displayTitle))
		if a.TranslatedTitle != "" {
			buf.WriteString(fmt.Sprintf(`<p style="color:#888;font-size:13px;margin:0 0 10px;">原文：%s</p>`, escapeHTML(a.Title)))
		}
		buf.WriteString(fmt.Sprintf(`<p style="color:#444;font-size:15px;line-height:1.8;margin:0 0 10px;">%s</p>`, summary))
		if a.Source != "" {
			buf.WriteString(fmt.Sprintf(`<p style="color:#aaa;font-size:13px;margin:0 0 20px;">📍 来源：%s</p>`, a.Source))
		}
		if a.URL != "" {
			buf.WriteString(fmt.Sprintf(`<p style="color:#576b95;font-size:13px;margin:0 0 20px;"><a href="%s">阅读原文</a></p>`, a.URL))
		}
		buf.WriteString(`<hr style="border:none;border-top:1px solid #f0f0f0;margin:15px 0;">`)
	}

	buf.WriteString(fmt.Sprintf(`<p style="color:#aaa;font-size:12px;text-align:center;margin:30px 0 10px;">由 <b>AI News Hub</b> 自动生成 · %s</p>`, time.Now().Format("2006-01-02")))
	return buf.String()
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// PublishTopArticles selects top articles, builds content, and publishes to WeChat.
// Returns the WeChat article URL or error.
func (p *Publisher) PublishTopArticles() error {
	if !p.Available() {
		return fmt.Errorf("wechat publisher not available (check WX_APPID/WX_SECRET/WX_ACCOUNT_ID)")
	}

	// Select top 15 articles
	articles, err := p.SelectTopArticles(15)
	if err != nil {
		return fmt.Errorf("select top articles: %w", err)
	}
	if len(articles) == 0 {
		return fmt.Errorf("no recent articles found")
	}

	log.Printf("[wechat] selected %d top articles for publishing", len(articles))

	// Build article title
	dateStr := time.Now().Format("2006年01月02日")
	articleTitle := fmt.Sprintf("【AI科技日报】%s · 今日%d条精选资讯", dateStr, len(articles))

	// Build content
	content := BuildArticleContent(articles, "")

	// Create digest (first 54 chars of first article summary)
	digest := ""
	for _, a := range articles {
		if s := a.AISummary; s != "" {
			if len(s) > 54 {
				s = s[:54] + "..."
			}
			digest = s
			break
		}
	}

	// Get thumb media_id from first article's image
	thumbMediaID := ""
	for _, a := range articles {
		if a.ImageURL != "" {
			tid, err := p.client.FetchThumbImage(a.ImageURL)
			if err == nil && tid != "" {
				thumbMediaID = tid
				log.Printf("[wechat] using thumb from article %d: %s", a.ID, a.ImageURL)
				break
			}
			log.Printf("[wechat] thumb fetch failed for %s: %v", a.ImageURL, err)
		}
	}

	// Publish
	mediaID, err := p.client.CreateDraft([]ThumbInfo{
		{
			ThumbMediaID:     thumbMediaID,
			Author:           "AI News Hub",
			Title:            articleTitle,
			Content:          content,
			Digest:           digest,
			ContentSourceURL: "",
			CanComment:       1,
			Comment:          1,
		},
	})
	if err != nil {
		return fmt.Errorf("create draft: %w", err)
	}
	if err := p.client.PublishDraft(mediaID); err != nil {
		return fmt.Errorf("publish draft: %w", err)
	}

	log.Printf("[wechat] ✅ article published: %s", articleTitle)
	return nil
}
