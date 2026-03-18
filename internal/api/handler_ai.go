package api

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"ai-news-hub/internal/ai"
	"ai-news-hub/internal/store"
)

// aiGenerateTask tracks an in-progress summary generation task.
type aiGenerateTask struct {
	Processing bool  `json:"processing"`
	StartedAt  string `json:"started_at"`
	Success    int    `json:"success"`
	Failed     int    `json:"failed"`
	Done       bool   `json:"done"`
}

var (
	aiTaskMu  sync.Mutex
	aiTask    *aiGenerateTask
)

// HandleAIGenerateSummaries handles POST /api/v1/ai/generate-summaries
// Triggers batch summary generation for articles without AI summaries.
func (s *Server) HandleAIGenerateSummaries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	if s.Summarizer == nil || !s.Summarizer.Available() {
		writeError(w, http.StatusServiceUnavailable, "AI 功能未启用：请配置 AI_API_KEY 环境变量或 config.yaml 中的 ai.api_key")
		return
	}

	aiTaskMu.Lock()
	if aiTask != nil && aiTask.Processing {
		aiTaskMu.Unlock()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"task":       "generate_summaries",
			"pending":    0,
			"processing": true,
			"message":    "摘要生成任务正在进行中，请稍后再试",
		})
		return
	}
	aiTask = &aiGenerateTask{
		Processing: true,
		StartedAt:  time.Now().Format(time.RFC3339),
	}
	aiTaskMu.Unlock()

	// Parse query params
	q := r.URL.Query()
	limit := 20
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v >= 1 && v <= 100 {
			limit = v
		}
	}
	force := q.Get("force") == "true"

	// Get articles to process
	var articles []store.Article
	var err error
	if force {
		// Force: get recent articles (may or may not have summaries)
		var total int
		articles, total, err = s.Store.QueryArticles(store.ArticleFilter{
			Page:    1,
			PerPage: limit,
			Sort:    "time",
		})
		if err != nil {
			articles = nil
		}
		_ = total
	} else {
		articles, err = s.Store.GetArticlesWithoutSummary(limit)
	}
	if err != nil {
		aiTaskMu.Lock()
		aiTask.Processing = false
		aiTask.Done = true
		aiTaskMu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to query articles: "+err.Error())
		return
	}

	// Filter articles that have some content to summarize
	var toProcess []store.Article
	for _, a := range articles {
		if force || a.ContentHTML != "" || a.Summary != "" {
			toProcess = append(toProcess, a)
		}
	}

	pending := len(toProcess)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"task":       "generate_summaries",
		"pending":    pending,
		"processing": true,
		"message":    "开始生成 " + strconv.Itoa(pending) + " 篇文章的摘要...",
	})

	// Run async
	go func() {
		log.Printf("[ai] starting batch summary generation for %d articles (force=%v)", pending, force)
		success, failed := s.Summarizer.GenerateSummariesBatch(toProcess, s.Store)

		// Recalculate importance scores for the processed articles
		for _, a := range toProcess {
			article, err := s.Store.GetArticleByID(a.ID)
			if err != nil || article == nil {
				continue
			}
			score := ai.CalculateImportance(*article)
			_ = s.Store.UpdateImportanceScore(a.ID, score)
		}

		aiTaskMu.Lock()
		if aiTask != nil {
			aiTask.Success = success
			aiTask.Failed = failed
			aiTask.Processing = false
			aiTask.Done = true
		}
		aiTaskMu.Unlock()
		log.Printf("[ai] batch summary generation complete: %d success, %d failed", success, failed)
	}()
}

// HandleAIGenerateSingleSummary handles POST /api/v1/ai/generate-summary/{id}
// Generates a summary for a single article.
func (s *Server) HandleAIGenerateSingleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	if s.Summarizer == nil || !s.Summarizer.Available() {
		writeError(w, http.StatusServiceUnavailable, "AI 功能未启用：请配置 AI_API_KEY 环境变量或 config.yaml 中的 ai.api_key")
		return
	}

	// Extract article ID from path
	path := r.URL.Path
	idStr := path[strings.LastIndex(path, "/")+1:]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid article ID")
		return
	}

	article, err := s.Store.GetArticleByID(id)
	if err != nil || article == nil {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}

	summary, err := s.Summarizer.GenerateSummary(*article)
	if err != nil {
		log.Printf("[ai] failed to generate summary for article %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to generate summary: "+err.Error())
		return
	}

	// Save to database
	if err := s.Store.UpdateAISummary(id, summary); err != nil {
		log.Printf("[ai] failed to save summary for article %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to save summary")
		return
	}

	// Recalculate importance score
	score := ai.CalculateImportance(*article)
	_ = s.Store.UpdateImportanceScore(id, score)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         id,
		"ai_summary": summary,
		"score":      score,
	})
}

// HandleAIRecalculateScores handles POST /api/v1/ai/recalculate-scores
// Batch recalculates importance scores for all articles.
func (s *Server) HandleAIRecalculateScores(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	log.Printf("[ai] starting score recalculation...")
	updated, err := ai.RecalculateScores(s.Store)
	if err != nil {
		log.Printf("[ai] score recalculation error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to recalculate scores: "+err.Error())
		return
	}

	log.Printf("[ai] score recalculation complete: %d articles updated", updated)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"updated": updated,
		"message": "已更新 " + strconv.Itoa(updated) + " 篇文章的评分",
	})
}

// HandleAISummaryStatus handles GET /api/v1/ai/summary-status
// Returns AI summary coverage statistics.
func (s *Server) HandleAISummaryStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	stats, err := s.Store.GetSummaryStats()
	if err != nil {
		log.Printf("[ai] summary stats error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get summary stats")
		return
	}

	// Include current task status
	var taskStatus interface{}
	aiTaskMu.Lock()
	if aiTask != nil {
		taskStatus = aiTask
	}
	aiTaskMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_articles":  stats.TotalArticles,
		"has_ai_summary":  stats.HasaISummary,
		"has_original_summary": stats.HasOriginal,
		"no_summary":      stats.NoSummary,
		"ai_coverage":     stats.AICoverage,
		"task":            taskStatus,
	})
}

// aiRouter routes /api/v1/ai/* requests.
func (s *Server) aiRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	prefix := "/api/v1/ai/"
	rest := path[len(prefix):]

	switch {
	case rest == "generate-summaries":
		s.HandleAIGenerateSummaries(w, r)
	case rest == "recalculate-scores":
		s.HandleAIRecalculateScores(w, r)
	case rest == "summary-status":
		s.HandleAISummaryStatus(w, r)
	case strings.HasPrefix(rest, "generate-summary/"):
		s.HandleAIGenerateSingleSummary(w, r)
	default:
		writeError(w, http.StatusNotFound, "AI endpoint not found")
	}
}
