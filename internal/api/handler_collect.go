package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"ai-news-hub/internal/collector"
	"ai-news-hub/internal/classifier"
	"ai-news-hub/internal/store"
)

// CollectService 封装采集调度所需的全部依赖。
type CollectService struct {
	Scheduler  *collector.CollectScheduler
	Classifier *classifier.Manager
	Store      store.ArticleStore
}

// CollectRequest 触发采集的请求体（目前所有字段可选）。
type CollectRequest struct {
	Sources []string `json:"sources,omitempty"` // 指定源名，为空则采集全部
}

// CollectResponse 采集结果响应。
type CollectResponse struct {
	StartedAt      string        `json:"started_at"`
	FinishedAt     string        `json:"finished_at"`
	DurationMs     int64         `json:"duration_ms"`
	Status         string        `json:"status"`
	TotalCollected int           `json:"total_collected"`
	TotalNew       int           `json:"total_new"`
	TotalSkipped   int           `json:"total_skipped"`
	ErrorsCount    int           `json:"errors_count"`
	Errors         []SourceError `json:"errors,omitempty"`
	SourceStats    []SourceStat  `json:"source_stats,omitempty"`
}

// SourceError 单个源的采集/分类错误。
type SourceError struct {
	Source string `json:"source"`
	Type   string `json:"type"` // "collect" | "classify" | "store"
	Error  string `json:"error"`
}

// SourceStat 单个源的统计。
type SourceStat struct {
	Source     string `json:"source"`
	Collected  int    `json:"collected"`
	Classified int    `json:"classified"`
}

// HandleCollect POST /api/v1/collect — 触发完整采集流程。
//
// 完整流程: CollectAll() → Classify() → BatchInsertArticles()
// → 记录 collect_runs → 返回统计 JSON。
func (s *CollectService) HandleCollect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	startedAt := time.Now()
	startedAtStr := startedAt.UTC().Format(time.RFC3339)

	log.Printf("[collect] ▶ 采集开始 %s", startedAtStr)

	// 1. 采集所有文章
	collectResults := s.Scheduler.CollectAll()
	log.Printf("[collect] 采集完成，共 %d 个源结果", len(collectResults))

	// 2. 逐源分类 + 转换 + 入库
	var allArticles []store.Article
	var sourceStats []SourceStat
	var sourceErrors []SourceError
	totalCollected := 0

	for _, cr := range collectResults {
		if cr.Err != nil {
			// 采集失败，记录错误但继续处理其他源
			sourceErrors = append(sourceErrors, SourceError{
				Source: cr.Source.Name,
				Type:   "collect",
				Error:  cr.Err.Error(),
			})
			log.Printf("[collect] ❌ 采集失败 %s: %v", cr.Source.Name, cr.Err)
			sourceStats = append(sourceStats, SourceStat{
				Source:     cr.Source.Name,
				Collected:  0,
				Classified: 0,
			})
			continue
		}

		classifiedCount := 0
		for _, ca := range cr.Articles {
			totalCollected++
			classifiedCount++

			// 3. 分类
			category := s.Classifier.Classify(&classifier.ArticleInput{
				Title:    ca.Title,
				Summary:  ca.Summary,
				Category: cr.Source.Categories,
				Source:   ca.SourceName,
				Language: ca.Language,
			})

			var publishedAt *string
			if ca.PublishedAt != "" {
				publishedAt = &ca.PublishedAt
			}

			// 4. 转换为 store.Article
			allArticles = append(allArticles, store.Article{
				Title:       ca.Title,
				URL:         ca.URL,
				Source:      ca.SourceName,
				SourceURL:   ca.SourceURL,
				Category:    category.Category,
				Summary:     ca.Summary,
				PublishedAt: publishedAt,
				CollectedAt: startedAtStr,
				Language:    ca.Language,
			})
		}

		sourceStats = append(sourceStats, SourceStat{
			Source:     cr.Source.Name,
			Collected:  len(cr.Articles),
			Classified: classifiedCount,
		})
		log.Printf("[collect] ✅ %s: 采集 %d 篇", cr.Source.Name, len(cr.Articles))
	}

	// 5. 批量入库（去重）
	totalNew := 0
	totalSkipped := 0
	if len(allArticles) > 0 {
		inserted, skipped, err := s.Store.BatchInsertArticles(allArticles)
		if err != nil {
			sourceErrors = append(sourceErrors, SourceError{
				Source: "store",
				Type:   "store",
				Error:  fmt.Sprintf("batch insert: %v", err),
			})
			log.Printf("[collect] ❌ 批量入库失败: %v", err)
		} else {
			totalNew = inserted
			totalSkipped = skipped
		}
	}

	finishedAt := time.Now()
	finishedAtStr := finishedAt.UTC().Format(time.RFC3339)
	durationMs := finishedAt.Sub(startedAt).Milliseconds()

	// 6. 记录 collect_runs
	runStatus := "success"
	errorsCount := len(sourceErrors)
	if errorsCount > 0 {
		if totalCollected == 0 {
			runStatus = "failed"
		} else {
			runStatus = "partial"
		}
	}

	var errorsJSON *string
	if len(sourceErrors) > 0 {
		errBytes, _ := json.Marshal(sourceErrors)
		errStr := string(errBytes)
		errorsJSON = &errStr
	}

	collectRun := &store.CollectRun{
		StartedAt:      startedAtStr,
		FinishedAt:     &finishedAtStr,
		Status:         runStatus,
		TotalCollected: totalCollected,
		TotalNew:       totalNew,
		ErrorsCount:    errorsCount,
		Errors:         errorsJSON,
	}
	if _, err := s.Store.InsertCollectRun(collectRun); err != nil {
		log.Printf("[collect] ⚠️ 记录 collect_run 失败: %v", err)
	}

	// 7. 构建响应
	resp := CollectResponse{
		StartedAt:      startedAtStr,
		FinishedAt:     finishedAtStr,
		DurationMs:     durationMs,
		Status:         runStatus,
		TotalCollected: totalCollected,
		TotalNew:       totalNew,
		TotalSkipped:   totalSkipped,
		ErrorsCount:    errorsCount,
		Errors:         sourceErrors,
		SourceStats:    sourceStats,
	}

	log.Printf("[collect] ■ 采集完成: 采集=%d 新增=%d 跳过=%d 错误=%d 耗时=%dms",
		totalCollected, totalNew, totalSkipped, errorsCount, durationMs)

	writeJSON(w, http.StatusOK, resp)
}

// HandleCollectStatus GET /api/v1/collect/status — 查询最近一次采集状态。
func (s *CollectService) HandleCollectStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	run, err := s.Store.GetLatestCollectRun()
	if err != nil {
		log.Printf("[collect] 查询最近采集状态失败: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to query collect status")
		return
	}

	if run == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status": "never_run",
		})
		return
	}

	writeJSON(w, http.StatusOK, run)
}
