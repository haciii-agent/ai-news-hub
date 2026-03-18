package api

import (
	"log"
	"net/http"
	"strconv"

	"ai-news-hub/internal/store"
)

// HandleDashboardStats handles GET /api/v1/dashboard/stats
func (s *Server) HandleDashboardStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	ds := store.NewDashboardStore(s.DB)
	stats, err := ds.GetStats()
	if err != nil {
		log.Printf("[api] dashboard stats error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get dashboard stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// HandleDashboardTrend handles GET /api/v1/dashboard/trend
func (s *Server) HandleDashboardTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v >= 1 && v <= 30 {
			days = v
		} else {
			writeError(w, http.StatusBadRequest, "days must be between 1 and 30")
			return
		}
	}

	ds := store.NewDashboardStore(s.DB)
	trend, err := ds.GetTrend(days)
	if err != nil {
		log.Printf("[api] dashboard trend error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get trend data")
		return
	}

	writeJSON(w, http.StatusOK, trend)
}

// HandleDashboardCategories handles GET /api/v1/dashboard/categories
func (s *Server) HandleDashboardCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	ds := store.NewDashboardStore(s.DB)
	dist, err := ds.GetCategoryDistribution()
	if err != nil {
		log.Printf("[api] dashboard categories error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get category distribution")
		return
	}

	writeJSON(w, http.StatusOK, dist)
}

// HandleDashboardSources handles GET /api/v1/dashboard/sources
func (s *Server) HandleDashboardSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	ds := store.NewDashboardStore(s.DB)
	sources, err := ds.GetSources()
	if err != nil {
		log.Printf("[api] dashboard sources error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get source health")
		return
	}

	writeJSON(w, http.StatusOK, sources)
}

// HandleDashboardRecentArticles handles GET /api/v1/dashboard/recent-articles
func (s *Server) HandleDashboardRecentArticles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v >= 1 && v <= 50 {
			limit = v
		}
	}

	ds := store.NewDashboardStore(s.DB)
	articles, err := ds.GetRecentArticles(limit)
	if err != nil {
		log.Printf("[api] dashboard recent articles error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get recent articles")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"articles": articles,
	})
}

// HandleDashboardCollectHistory handles GET /api/v1/dashboard/collect-history
func (s *Server) HandleDashboardCollectHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v >= 1 && v <= 50 {
			limit = v
		}
	}

	ds := store.NewDashboardStore(s.DB)
	history, err := ds.GetCollectHistory(limit)
	if err != nil {
		log.Printf("[api] dashboard collect history error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get collect history")
		return
	}

	writeJSON(w, http.StatusOK, history)
}

// dashboardRouter routes /api/v1/dashboard/* requests.
func (s *Server) dashboardRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	prefix := "/api/v1/dashboard/"
	rest := path[len(prefix):]

	switch rest {
	case "stats":
		s.HandleDashboardStats(w, r)
	case "trend":
		s.HandleDashboardTrend(w, r)
	case "categories":
		s.HandleDashboardCategories(w, r)
	case "sources":
		s.HandleDashboardSources(w, r)
	case "recent-articles":
		s.HandleDashboardRecentArticles(w, r)
	case "collect-history":
		s.HandleDashboardCollectHistory(w, r)
	default:
		writeError(w, http.StatusNotFound, "dashboard endpoint not found")
	}
}
