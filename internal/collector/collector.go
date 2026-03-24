// Package collector provides RSS and HTML news collection capabilities.
package collector

import (
	"encoding/json"
	"log"
	"time"

	"ai-news-hub/config"
	"ai-news-hub/internal/store"
)

// CollectScheduler 统一调度入口：并发采集所有数据源（RSS + HTML）。
type CollectScheduler struct {
	rssCollector  *RSSCollector
	htmlCollector *HTMLCollector
	store         ArticleStore
}

// ArticleStore interface for persistence operations needed by scheduler.
type ArticleStore interface {
	InsertCollectRun(run *store.CollectRun) (int64, error)
}

// NewCollectScheduler 创建统一采集调度器。
func NewCollectScheduler(cfg *config.CollectorConfig, store ArticleStore) *CollectScheduler {
	return &CollectScheduler{
		rssCollector: NewRSSCollector(
			WithFetchTimeout(cfg.Timeout),
			WithMaxItems(cfg.RSSMaxItems),
			WithUserAgent(cfg.UserAgent),
			WithMaxWorkers(cfg.MaxConcurrent),
		),
		htmlCollector: NewHTMLCollector(
			WithHTMLMaxWorkers(3),
			func(c *HTMLCollector) {
				// 注册默认解析器
				for name, parser := range DefaultHTMLParsers() {
					c.parsers[name] = parser
				}
			},
		),
		store: store,
	}
}

// CollectAll 并发采集所有数据源，返回采集结果列表。
func (s *CollectScheduler) CollectAll() []CollectResult {
	var allResults []CollectResult

	// 并发采集 RSS 和 HTML 源
	rssCh := make(chan []CollectResult, 1)
	htmlCh := make(chan []CollectResult, 1)

	go func() {
		rssCh <- s.rssCollector.CollectAll(RSSSources())
	}()
	go func() {
		htmlCh <- s.htmlCollector.CollectAll(HTMLSources())
	}()

	// 等待两个采集器完成
	rssResults := <-rssCh
	htmlResults := <-htmlCh

	allResults = append(allResults, rssResults...)
	allResults = append(allResults, htmlResults...)

	// 保存采集历史到数据库
	s.saveCollectRun(allResults)

	return allResults
}

// saveCollectRun writes a collect run record to the database.
func (s *CollectScheduler) saveCollectRun(results []CollectResult) {
	if s.store == nil {
		return
	}
	var totalCollected, totalNew, errCount int
	var errs []string
	for _, r := range results {
		if r.Err != nil {
			errCount++
			errs = append(errs, r.Source.Name+" → "+r.Err.Error())
		} else {
			totalCollected += len(r.Articles)
			for range r.Articles {
				totalNew++
			}
		}
	}
	status := "success"
	if errCount > 0 {
		status = "partial"
	}
	startedAt := time.Now().Format(time.RFC3339)
	errsJSON := joinErrors(errs)
	run := &store.CollectRun{
		StartedAt:      startedAt,
		Status:         status,
		TotalCollected: totalCollected,
		TotalNew:       totalNew,
		ErrorsCount:    errCount,
		Errors:         &errsJSON,
	}
	if _, err := s.store.InsertCollectRun(run); err != nil {
		log.Printf("[scheduler] failed to save collect run: %v", err)
	} else {
		log.Printf("[scheduler] collect run saved: status=%s, collected=%d, new=%d, errors=%d", status, totalCollected, totalNew, errCount)
	}
}

func joinErrors(errs []string) string {
	if len(errs) == 0 {
		return ""
	}
	type errEntry struct {
		Source string `json:"source"`
		Type   string `json:"type"`
		Error  string `json:"error"`
	}
	entries := make([]errEntry, 0, len(errs))
	for _, e := range errs {
		entries = append(entries, errEntry{Source: "source", Type: "collect", Error: e})
	}
	b, _ := json.Marshal(entries)
	return string(b)
}

// CollectRSS 仅采集 RSS 源。
func (s *CollectScheduler) CollectRSS() []CollectResult {
	return s.rssCollector.CollectAll(RSSSources())
}

// CollectHTML 仅采集 HTML 源。
func (s *CollectScheduler) CollectHTML() []CollectResult {
	return s.htmlCollector.CollectAll(HTMLSources())
}
