package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"ai-news-hub/config"
	"ai-news-hub/internal/collector"
	"ai-news-hub/internal/classifier"
	"ai-news-hub/internal/store"
	"ai-news-hub/internal/static"
)

// Server holds all dependencies for the HTTP server.
type Server struct {
	DB         *sql.DB
	Cfg        *config.Config
	Store      store.ArticleStore
	CollectSvc *CollectService
	Classifier *classifier.Manager
}

// NewServer creates a fully-wired HTTP server with all routes registered.
func NewServer(db *sql.DB, cfg *config.Config) (*Server, error) {
	// Initialize store
	articleStore := store.NewArticleStore(db)

	// Initialize classifier
	clr, err := classifier.NewManager(cfg.Classifier.RulesPath)
	if err != nil {
		return nil, fmt.Errorf("init classifier: %w", err)
	}

	// Initialize collect scheduler
	sched := collector.NewCollectScheduler()

	srv := &Server{
		DB:         db,
		Cfg:        cfg,
		Store:      articleStore,
		Classifier: clr,
		CollectSvc: &CollectService{
			Scheduler:  sched,
			Classifier: clr,
			Store:      articleStore,
		},
	}

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
	mux.HandleFunc("/api/v1/articles/", s.methodRouter(nil, s.articleDetailHandler))
	mux.HandleFunc("/api/v1/categories", s.categoriesHandler)
	mux.HandleFunc("/api/v1/collect", s.CollectSvc.HandleCollect)
	mux.HandleFunc("/api/v1/collect/status", s.CollectSvc.HandleCollectStatus)

	// Static files (embed.FS) — serve at root, API takes precedence
	staticFS := http.FileServer(http.FS(static.FS()))
	mux.Handle("/", staticFS)

	// Apply CORS middleware (development: AllowAll)
	handler := corsMiddleware(mux)

	log.Println("[api] routes registered:")
	log.Println("  GET  /health")
	log.Println("  GET  /healthz")
	log.Println("  GET  /api/v1/articles")
	log.Println("  GET  /api/v1/articles/{id}")
	log.Println("  GET  /api/v1/categories")
	log.Println("  POST /api/v1/collect")
	log.Println("  GET  /api/v1/collect/status")
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

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"service": "ai-news-hub",
		"version": "0.1.0",
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

	// search 参数在 MVP 标记为 unsupported
	if q.Get("search") != "" {
		writeError(w, http.StatusBadRequest, "search is not supported in this version")
		return
	}

	filter := store.ArticleFilter{
		Category: q.Get("category"),
		Sort:     q.Get("sort"),
		Language: q.Get("language"),
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

	articles, total, err := s.Store.QueryArticles(filter)
	if err != nil {
		log.Printf("[api] query articles error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to query articles")
		return
	}

	// Calculate total_pages
	totalPages := 0
	if total > 0 {
		totalPages = (total + filter.PerPage - 1) / filter.PerPage
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"articles":    articles,
		"total":       total,
		"page":        filter.Page,
		"per_page":    filter.PerPage,
		"total_pages": totalPages,
	})
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
