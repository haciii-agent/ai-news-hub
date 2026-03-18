package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"ai-news-hub/internal/store"
)

// --- getUserID helper ---

// getUserID extracts user token from X-User-Token header and returns the user ID.
// Returns 0 if no token is provided (guest mode).
func (s *Server) getUserID(r *http.Request) int64 {
	token := r.Header.Get("X-User-Token")
	if token == "" {
		return 0
	}
	user, _, err := s.UserStore.GetOrCreateUserByToken(token)
	if err != nil {
		log.Printf("[api] getUserID error: %v", err)
		return 0
	}
	return user.ID
}

// --- User Init Handler ---

// HandleUserInit initializes or returns the user for the given token.
// POST /api/v1/user/init
// Header: X-User-Token: <uuid>
func (s *Server) HandleUserInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	token := r.Header.Get("X-User-Token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "X-User-Token header is required")
		return
	}

	user, created, err := s.UserStore.GetOrCreateUserByToken(token)
	if err != nil {
		log.Printf("[api] user init error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to init user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": user.ID,
		"token":   user.Token,
		"created": created,
	})
}

// --- Bookmark Handlers ---

// HandleBookmarkCreate bookmarks an article.
// POST /api/v1/bookmarks
// Header: X-User-Token: <uuid>
// Body: {"article_id": 123}
func (s *Server) HandleBookmarkCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	userID := s.getUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "X-User-Token header is required")
		return
	}

	var req struct {
		ArticleID int64 `json:"article_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ArticleID <= 0 {
		writeError(w, http.StatusBadRequest, "article_id must be positive")
		return
	}

	if err := s.UserStore.BookmarkArticle(userID, req.ArticleID); err != nil {
		log.Printf("[api] bookmark error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to bookmark article")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"bookmarked": true,
	})
}

// HandleBookmarkDelete removes a bookmark.
// DELETE /api/v1/bookmarks/{id}
// Header: X-User-Token: <uuid>
func (s *Server) HandleBookmarkDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "only DELETE allowed")
		return
	}

	userID := s.getUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "X-User-Token header is required")
		return
	}

	// Extract article ID from path: /api/v1/bookmarks/123
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/bookmarks/")
	path = strings.TrimSuffix(path, "/")

	articleID, err := strconv.ParseInt(path, 10, 64)
	if err != nil || articleID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid article ID")
		return
	}

	if err := s.UserStore.UnbookmarkArticle(userID, articleID); err != nil {
		log.Printf("[api] unbookmark error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to unbookmark article")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"bookmarked": false,
	})
}

// HandleBookmarkList lists bookmarked articles.
// GET /api/v1/bookmarks?page=1&per_page=20
// Header: X-User-Token: <uuid>
func (s *Server) HandleBookmarkList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	userID := s.getUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "X-User-Token header is required")
		return
	}

	q := r.URL.Query()
	filter := store.ArticleFilter{
		Page:    1,
		PerPage: 20,
	}
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v >= 1 {
			filter.Page = v
		}
	}
	if pp := q.Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v >= 1 && v <= 100 {
			filter.PerPage = v
		}
	}

	articles, total, err := s.UserStore.ListBookmarks(userID, filter)
	if err != nil {
		log.Printf("[api] list bookmarks error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list bookmarks")
		return
	}

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

// HandleBookmarkStatus returns bookmark status for multiple article IDs.
// GET /api/v1/bookmarks/status?ids=1,2,3,4,5
// Header: X-User-Token: <uuid>
func (s *Server) HandleBookmarkStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	userID := s.getUserID(r)
	if userID == 0 {
		// Return all false for guests
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"bookmarks": map[string]bool{},
		})
		return
	}

	idsStr := r.URL.Query().Get("ids")
	if idsStr == "" {
		writeError(w, http.StatusBadRequest, "ids parameter is required")
		return
	}

	ids, err := store.ParseIDs(idsStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ids parameter")
		return
	}

	bookmarked, err := s.UserStore.GetBookmarkedIDs(userID, ids)
	if err != nil {
		log.Printf("[api] bookmark status error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get bookmark status")
		return
	}

	// Convert to string keys for JSON
	result := make(map[string]bool)
	for id, val := range bookmarked {
		result[strconv.FormatInt(id, 10)] = val
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"bookmarks": result,
	})
}

// --- Read History Handlers ---

// HandleHistoryRecord records a read history entry.
// POST /api/v1/history
// Header: X-User-Token: <uuid>
// Body: {"article_id": 123}
func (s *Server) HandleHistoryRecord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	userID := s.getUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "X-User-Token header is required")
		return
	}

	var req struct {
		ArticleID int64 `json:"article_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ArticleID <= 0 {
		writeError(w, http.StatusBadRequest, "article_id must be positive")
		return
	}

	if err := s.UserStore.RecordReadHistory(userID, req.ArticleID); err != nil {
		log.Printf("[api] record history error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to record read history")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"recorded": true,
	})
}

// HandleHistoryList lists recently read articles.
// GET /api/v1/history?page=1&per_page=20
// Header: X-User-Token: <uuid>
func (s *Server) HandleHistoryList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	userID := s.getUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "X-User-Token header is required")
		return
	}

	q := r.URL.Query()
	filter := store.ArticleFilter{
		Page:    1,
		PerPage: 20,
	}
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v >= 1 {
			filter.Page = v
		}
	}
	if pp := q.Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v >= 1 && v <= 100 {
			filter.PerPage = v
		}
	}

	articles, total, err := s.UserStore.ListReadHistory(userID, filter)
	if err != nil {
		log.Printf("[api] list history error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list read history")
		return
	}

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
