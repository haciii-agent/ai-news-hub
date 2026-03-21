package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"ai-news-hub/config"
	"ai-news-hub/internal/ai"
	"ai-news-hub/internal/auth"
	"ai-news-hub/internal/collector"
	"ai-news-hub/internal/classifier"
	"ai-news-hub/internal/store"
	"ai-news-hub/internal/static"
)

// Server holds all dependencies for the HTTP server.
type Server struct {
	DB              *sql.DB
	Cfg             *config.Config
	Store           store.ArticleStore
	UserStore       store.UserStore
	ProfileStore    store.ProfileStore
	AuthMgr         *auth.AuthManager
	AuthStore       store.AuthStore
	LoginLogStore   store.LoginLogStore
	AdminMgr        store.AdminStore // Worker-B will populate this
	CollectSvc      *CollectService
	Classifier      *classifier.Manager
	Summarizer      *ai.Summarizer
	Version         string
}

// NewServer creates a fully-wired HTTP server with all routes registered.
func NewServer(db *sql.DB, cfg *config.Config, version string) (*Server, error) {
	// Initialize store
	articleStore := store.NewArticleStore(db)
	userStore := store.NewUserStore(db)

	// Initialize classifier
	clr, err := classifier.NewManager(cfg.Classifier.RulesPath)
	if err != nil {
		return nil, fmt.Errorf("init classifier: %w", err)
	}

	// Initialize AI summarizer (nil if not configured)
	summarizer := ai.NewSummarizer(cfg.AI)
	if summarizer != nil {
		log.Println("[api] AI summarizer enabled (model: " + cfg.AI.Model + ")")
	} else {
		log.Println("[api] AI summarizer disabled (no API key configured)")
	}

	// Initialize collect scheduler
	sched := collector.NewCollectScheduler(&cfg.Collector)

	srv := &Server{
		DB:         db,
		Cfg:        cfg,
		Store:      articleStore,
		UserStore:  userStore,
		Classifier: clr,
		Summarizer: summarizer,
		Version:    version,
		CollectSvc: &CollectService{
			Scheduler:  sched,
			Classifier: clr,
			Store:      articleStore,
		},
	}

	// Initialize profile store (v1.0.0)
	srv.initProfileStore(db)

	// Initialize auth system (v1.2.0)
	authStore := store.NewAuthStore(db)
	loginLogStore := store.NewLoginLogStore(db)
	authMgr := auth.NewAuthManager(cfg.Auth, authStore)
	srv.AuthStore = authStore
	srv.LoginLogStore = loginLogStore
	srv.AuthMgr = authMgr
	log.Println("[api] auth system initialized (JWT enabled)")

	return srv, nil
}

// Handler returns the main HTTP handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/healthz", s.healthHandler)

	// API v1 routes
	mux.HandleFunc("/api/v1/articles", s.methodRouter(s.articlesHandler, s.articleDetailHandler))
	mux.HandleFunc("/api/v1/articles/", s.articlesPathRouter)
	mux.HandleFunc("/api/v1/articles/", s.methodRouter(nil, s.HandleArticleContent))
	mux.HandleFunc("/api/v1/categories", s.categoriesHandler)
	mux.HandleFunc("/api/v1/collect", s.CollectSvc.HandleCollect)
	mux.HandleFunc("/api/v1/collect/status", s.CollectSvc.HandleCollectStatus)
	mux.HandleFunc("/api/v1/stats", s.CollectSvc.HandleStats)
	mux.HandleFunc("/api/v1/articles/cleanup", s.CollectSvc.HandleCleanup)
	mux.HandleFunc("/api/v1/sources", s.CollectSvc.HandleSources)

	// Dashboard (v0.8.0) — no auth required
	mux.HandleFunc("/api/v1/dashboard/", s.dashboardRouter)

	// AI features (v0.9.0)
	mux.HandleFunc("/api/v1/ai/", s.aiRouter)

	// User features (v0.7.0)
	mux.HandleFunc("/api/v1/user/init", s.HandleUserInit)
	mux.HandleFunc("/api/v1/bookmarks", s.bookmarksRouter)
	mux.HandleFunc("/api/v1/bookmarks/", s.bookmarksPathRouter)
	mux.HandleFunc("/api/v1/history", s.historyRouter)

	// Trend analysis features (v1.1.0)
	mux.HandleFunc("/api/v1/trends/hot", s.HandleTrendHot)
	mux.HandleFunc("/api/v1/trends/timeline", s.HandleTrendTimeline)
	mux.HandleFunc("/api/v1/trends/story-pitches", s.HandleTrendStoryPitches)
	mux.HandleFunc("/api/v1/trends/related", s.HandleTrendRelated)

	// Recommendation features (v1.0.0)
	mux.HandleFunc("/api/v1/recommendations", s.HandleRecommendations)
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.HandleUserProfile(w, r)
		case http.MethodPut:
			s.HandleUpdateUserProfile(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "only GET or PUT allowed")
		}
	})
	mux.HandleFunc("/api/v1/user/streak", s.HandleReadingStreak)
	mux.HandleFunc("/api/v1/user/password", s.HandleUpdatePassword)

	// Auth routes (v1.2.0)
	mux.HandleFunc("/api/v1/auth/register", s.HandleRegister)
	mux.HandleFunc("/api/v1/auth/login", s.HandleLogin)
	mux.HandleFunc("/api/v1/auth/me", s.HandleMe)
	mux.HandleFunc("/api/v1/auth/refresh", s.HandleRefresh)
	mux.HandleFunc("/api/v1/auth/logout", s.HandleLogout)
	mux.HandleFunc("/api/v1/auth/check-username", s.HandleCheckUsername)
	mux.HandleFunc("/api/v1/auth/check-email", s.HandleCheckEmail)

	// Admin routes (v1.2.0) — handlers implemented by Worker-B
	mux.HandleFunc("/api/v1/admin/users", s.HandleAdminListUsers)
	mux.HandleFunc("/api/v1/admin/users/", s.adminRouter)

	// Static files (embed.FS) — serve at root, API takes precedence
	staticFS := http.FileServer(http.FS(static.FS()))
	mux.Handle("/", staticFS)

	// Apply global auth middleware (injects user info into context, does not block)
	globalAuth := s.AuthMgr.AuthMiddleware(mux)

	// Apply CORS middleware (development: AllowAll)
	handler := corsMiddleware(globalAuth)

	log.Println("[api] routes registered:")
	log.Println("  GET  /health")
	log.Println("  GET  /healthz")
	log.Println("  GET  /api/v1/articles")
	log.Println("  GET  /api/v1/articles/{id}")
	log.Println("  GET  /api/v1/articles/{id}/content")
	log.Println("  GET  /api/v1/categories")
	log.Println("  POST /api/v1/collect")
	log.Println("  GET  /api/v1/collect/status")
	log.Println("  GET  /api/v1/sources")
	log.Println("  GET  /api/v1/dashboard/stats")
	log.Println("  GET  /api/v1/dashboard/trend")
	log.Println("  GET  /api/v1/dashboard/categories")
	log.Println("  GET  /api/v1/dashboard/sources")
	log.Println("  GET  /api/v1/dashboard/recent-articles")
	log.Println("  GET  /api/v1/dashboard/collect-history")
	log.Println("  POST /api/v1/user/init")
	log.Println("  POST /api/v1/bookmarks")
	log.Println("  DELETE /api/v1/bookmarks/{id}")
	log.Println("  GET /api/v1/bookmarks (list)")
	log.Println("  GET /api/v1/bookmarks/status")
	log.Println("  POST /api/v1/history")
	log.Println("  GET /api/v1/history (list)")
	log.Println("  POST /api/v1/ai/generate-summaries")
	log.Println("  POST /api/v1/ai/generate-summary/{id}")
	log.Println("  POST /api/v1/ai/recalculate-scores")
	log.Println("  GET  /api/v1/ai/summary-status")
	log.Println("  GET  /api/v1/trends/hot")
	log.Println("  GET  /api/v1/trends/timeline")
	log.Println("  GET  /api/v1/trends/story-pitches")
	log.Println("  GET  /api/v1/trends/related")
	log.Println("  GET  /api/v1/recommendations")
	log.Println("  GET  /api/v1/user/profile")
	log.Println("  PUT  /api/v1/user/profile")
	log.Println("  GET  /api/v1/user/streak")
	log.Println("  GET  /  (static files via embed.FS)")
	log.Println("  CORS: AllowAll (development)")

	return handler
}

// Close cleans up resources (e.g. classifier file watcher).
func (s *Server) Close() {
	if s.Classifier != nil {
		s.Classifier.Stop()
	}
}

// methodRouter dispatches GET /list vs GET /detail based on trailing slash presence.
func (s *Server) methodRouter(listHandler, detailHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// /api/v1/articles/{id} — detail (trailing path segment)
		if detailHandler != nil && strings.Count(path, "/") >= 4 {
			detailHandler(w, r)
			return
		}

		// /api/v1/articles — list
		if listHandler != nil {
			listHandler(w, r)
			return
		}

		writeError(w, http.StatusNotFound, "resource not found")
	}
}

// articlesPathRouter routes /api/v1/articles/... requests to the correct handler.
// /api/v1/articles/{id} → articleDetailHandler
// /api/v1/articles/{id}/content → HandleArticleContent
func (s *Server) articlesPathRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/articles/")
	path = strings.TrimSuffix(path, "/")

	// /api/v1/articles/{id}/content
	if strings.HasSuffix(path, "/content") {
		s.HandleArticleContent(w, r)
		return
	}

	// /api/v1/articles/{id}
	s.articleDetailHandler(w, r)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"service": "ai-news-hub",
		"version": s.Version,
	})
}

// --- Article handlers ---

func (s *Server) articlesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	// Parse query parameters
	q := r.URL.Query()

	searchQuery := q.Get("search")

	filter := store.ArticleFilter{
		Category: q.Get("category"),
		Sort:     q.Get("sort"),
		Language: q.Get("language"),
		Search:   searchQuery,
	}

	// Parse page
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v >= 1 {
			filter.Page = v
		} else {
			writeError(w, http.StatusBadRequest, "invalid page parameter")
			return
		}
	} else {
		filter.Page = 1
	}

	// Parse per_page
	if pp := q.Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v >= 1 && v <= 100 {
			filter.PerPage = v
		} else {
			writeError(w, http.StatusBadRequest, "invalid per_page parameter (must be 1-100)")
			return
		}
	} else {
		filter.PerPage = 20
	}

	// Execute query: use FTS search if search param provided, otherwise normal query
	var articles []store.Article
	var total int
	var snippets map[int64]string

	if searchQuery != "" {
		var err error
		articles, total, snippets, err = s.Store.SearchArticles(searchQuery, filter)
		if err != nil {
			log.Printf("[api] search articles error: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to search articles")
			return
		}
	} else {
		var err error
		articles, total, err = s.Store.QueryArticles(filter)
		if err != nil {
			log.Printf("[api] query articles error: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to query articles")
			return
		}
	}

	// Calculate total_pages
	totalPages := 0
	if total > 0 {
		totalPages = (total + filter.PerPage - 1) / filter.PerPage
	}

	// Build response
	resp := map[string]interface{}{
		"articles":    articles,
		"total":       total,
		"page":        filter.Page,
		"per_page":    filter.PerPage,
		"total_pages": totalPages,
	}

	// Include snippets for search results
	if searchQuery != "" && snippets != nil {
		resp["snippets"] = snippets
		resp["search"] = searchQuery
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) articleDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	// Extract ID from path: /api/v1/articles/123
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/articles/")
	idStr := strings.TrimSuffix(path, "/")

	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		writeError(w, http.StatusBadRequest, "article ID must be a number")
		return
	}

	article, err := s.Store.GetArticleByID(id)
	if err != nil {
		log.Printf("[api] get article error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get article")
		return
	}

	if article == nil {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}

	writeJSON(w, http.StatusOK, article)
}

func (s *Server) categoriesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	stats, err := s.Store.GetCategoryStats()
	if err != nil {
		log.Printf("[api] category stats error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get category stats")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"categories": stats,
	})
}

// --- Bookmark/History Routers ---

// bookmarksRouter routes /api/v1/bookmarks requests (no trailing path).
// GET → list bookmarks; POST → create bookmark
func (s *Server) bookmarksRouter(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.HandleBookmarkList(w, r)
	case http.MethodPost:
		s.HandleBookmarkCreate(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "only GET or POST allowed")
	}
}

// bookmarksPathRouter routes /api/v1/bookmarks/... requests (with trailing path).
// /api/v1/bookmarks/status → GET bookmark status
// /api/v1/bookmarks/{id} → DELETE bookmark
func (s *Server) bookmarksPathRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/bookmarks/")
	path = strings.TrimSuffix(path, "/")

	if path == "status" {
		s.HandleBookmarkStatus(w, r)
		return
	}

	// Default: treat as /api/v1/bookmarks/{id} for DELETE
	s.HandleBookmarkDelete(w, r)
}

// historyRouter routes /api/v1/history requests.
// GET → list history; POST → record history
func (s *Server) historyRouter(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.HandleHistoryList(w, r)
	case http.MethodPost:
		s.HandleHistoryRecord(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "only GET or POST allowed")
	}
}
