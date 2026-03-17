// Package collector provides RSS and HTML news collection capabilities.
package collector

// CollectScheduler 统一调度入口：并发采集所有数据源（RSS + HTML）。
type CollectScheduler struct {
	rssCollector  *RSSCollector
	htmlCollector *HTMLCollector
}

// NewCollectScheduler 创建统一采集调度器，使用默认配置。
func NewCollectScheduler() *CollectScheduler {
	return &CollectScheduler{
		rssCollector: NewRSSCollector(),
		htmlCollector: NewHTMLCollector(
			WithHTMLMaxWorkers(3),
			func(c *HTMLCollector) {
				// 注册默认解析器
				for name, parser := range DefaultHTMLParsers() {
					c.parsers[name] = parser
				}
			},
		),
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
	return allResults
}

// CollectRSS 仅采集 RSS 源。
func (s *CollectScheduler) CollectRSS() []CollectResult {
	return s.rssCollector.CollectAll(RSSSources())
}

// CollectHTML 仅采集 HTML 源。
func (s *CollectScheduler) CollectHTML() []CollectResult {
	return s.htmlCollector.CollectAll(HTMLSources())
}
