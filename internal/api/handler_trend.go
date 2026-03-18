package api

import (
	"log"
	"net/http"
	"strconv"

	"ai-news-hub/internal/ai"
)

// HandleTrendHot handles GET /api/v1/trends/hot
func (s *Server) HandleTrendHot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	q := r.URL.Query()
	period := q.Get("period")
	if period == "" {
		period = "7d"
	}
	limit := 20
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v >= 1 && v <= 50 {
			limit = v
		}
	}

	analyzer := ai.NewTrendAnalyzer(s.DB)
	resp, err := analyzer.GetHotTopics(period, limit)
	if err != nil {
		log.Printf("[api] trend hot error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get hot topics")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleTrendTimeline handles GET /api/v1/trends/timeline
func (s *Server) HandleTrendTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	q := r.URL.Query()
	keyword := q.Get("keyword")
	if keyword == "" {
		writeError(w, http.StatusBadRequest, "keyword parameter is required")
		return
	}

	days := 14
	if d := q.Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v >= 1 && v <= 30 {
			days = v
		}
	}

	analyzer := ai.NewTrendAnalyzer(s.DB)
	resp, err := analyzer.GetTimeline(keyword, days)
	if err != nil {
		log.Printf("[api] trend timeline error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get timeline")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleTrendStoryPitches handles GET /api/v1/trends/story-pitches
func (s *Server) HandleTrendStoryPitches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	q := r.URL.Query()
	limit := 10
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v >= 1 && v <= 30 {
			limit = v
		}
	}

	analyzer := ai.NewTrendAnalyzer(s.DB)
	pitches, err := analyzer.GetStoryPitches(limit)
	if err != nil {
		log.Printf("[api] story pitches error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get story pitches")
		return
	}

	writeJSON(w, http.StatusOK, ai.StoryPitchesResponse{Pitches: pitches})
}

// HandleTrendRelated handles GET /api/v1/trends/related
func (s *Server) HandleTrendRelated(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	q := r.URL.Query()
	keyword := q.Get("keyword")
	if keyword == "" {
		writeError(w, http.StatusBadRequest, "keyword parameter is required")
		return
	}

	limit := 10
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v >= 1 && v <= 30 {
			limit = v
		}
	}

	analyzer := ai.NewTrendAnalyzer(s.DB)
	resp, err := analyzer.GetRelatedTopics(keyword, limit)
	if err != nil {
		log.Printf("[api] related topics error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get related topics")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
