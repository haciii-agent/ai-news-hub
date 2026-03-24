package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"ai-news-hub/internal/store"
)

// --- Comment Handlers ---

// commentRouter routes /api/v1/articles/{id}/comments[/{comment_id}] requests.
func (s *Server) commentRouter(w http.ResponseWriter, r *http.Request, articleIDStr, commentPath string) {
	articleID, err := strconv.ParseInt(articleIDStr, 10, 64)
	if err != nil || articleID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid article ID")
		return
	}

	commentPath = strings.TrimPrefix(commentPath, "/")

	// DELETE /api/v1/articles/{id}/comments/{comment_id}
	if r.Method == http.MethodDelete && commentPath != "" {
		s.HandleDeleteComment(w, r, commentPath)
		return
	}

	// POST /api/v1/articles/{id}/comments — create comment
	if r.Method == http.MethodPost && commentPath == "" {
		s.HandleCreateComment(w, r, articleID)
		return
	}

	// GET /api/v1/articles/{id}/comments — list comments
	if r.Method == http.MethodGet && commentPath == "" {
		s.HandleListComments(w, r, articleID)
		return
	}

	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// HandleCreateComment creates a comment on an article.
// POST /api/v1/articles/{id}/comments
// Header: X-User-Token: <uuid>
// Body: { "content": "..." }
func (s *Server) HandleCreateComment(w http.ResponseWriter, r *http.Request, articleID int64) {
	userID := s.getUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "X-User-Token header is required")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	content := strings.TrimSpace(req.Content)
	if len(content) < 1 || len(content) > 500 {
		writeError(w, http.StatusBadRequest, "评论内容长度需要在 1-500 字符之间")
		return
	}

	comment, err := s.CommentStore.AddComment(articleID, userID, content)
	if err != nil {
		log.Printf("[api] create comment error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create comment")
		return
	}

	// Trigger profile update asynchronously (comment weight: +0.15)
	if us, ok := s.UserStore.(interface {
		CommentArticleWithProfileUpdate(int64, int64)
	}); ok {
		us.CommentArticleWithProfileUpdate(userID, articleID)
	}

	writeJSON(w, http.StatusCreated, comment)
}

// HandleListComments returns comments for an article.
// GET /api/v1/articles/{id}/comments?page=1&per_page=20
func (s *Server) HandleListComments(w http.ResponseWriter, r *http.Request, articleID int64) {
	q := r.URL.Query()
	page := 1
	perPage := 20

	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v >= 1 {
			page = v
		}
	}
	if pp := q.Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v >= 1 && v <= 100 {
			perPage = v
		}
	}

	comments, total, err := s.CommentStore.ListComments(articleID, page, perPage)
	if err != nil {
		log.Printf("[api] list comments error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list comments")
		return
	}

	if comments == nil {
		comments = []store.Comment{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"comments": comments,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// HandleDeleteComment deletes a comment.
// DELETE /api/v1/articles/{id}/comments/{comment_id}
// Header: X-User-Token: <uuid>
func (s *Server) HandleDeleteComment(w http.ResponseWriter, r *http.Request, commentIDStr string) {
	userID := s.getUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "X-User-Token header is required")
		return
	}

	commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
	if err != nil || commentID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}

	err = s.CommentStore.DeleteComment(commentID, userID)
	if err != nil {
		if err.Error() == "comment not found or not owned by user" {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		log.Printf("[api] delete comment error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete comment")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": true,
	})
}

// --- Like Handlers ---

// likeRouter routes /api/v1/articles/{id}/like requests.
func (s *Server) likeRouter(w http.ResponseWriter, r *http.Request, articleIDStr string) {
	articleID, err := strconv.ParseInt(articleIDStr, 10, 64)
	if err != nil || articleID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid article ID")
		return
	}

	switch r.Method {
	case http.MethodPost:
		s.HandleLikeArticle(w, r, articleID)
	case http.MethodDelete:
		s.HandleUnlikeArticle(w, r, articleID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "only POST or DELETE allowed")
	}
}

// HandleLikeArticle likes an article.
// POST /api/v1/articles/{id}/like
// Header: X-User-Token: <uuid>
func (s *Server) HandleLikeArticle(w http.ResponseWriter, r *http.Request, articleID int64) {
	userID := s.getUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "X-User-Token header is required")
		return
	}

	err := s.CommentStore.LikeArticle(articleID, userID)
	if err != nil {
		log.Printf("[api] like article error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to like article")
		return
	}

	// Trigger profile update asynchronously (like weight: +0.10)
	if us, ok := s.UserStore.(interface {
		LikeArticleWithProfileUpdate(int64, int64)
	}); ok {
		us.LikeArticleWithProfileUpdate(userID, articleID)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"liked": true,
	})
}

// HandleUnlikeArticle unlikes an article.
// DELETE /api/v1/articles/{id}/like
// Header: X-User-Token: <uuid>
func (s *Server) HandleUnlikeArticle(w http.ResponseWriter, r *http.Request, articleID int64) {
	userID := s.getUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "X-User-Token header is required")
		return
	}

	err := s.CommentStore.UnlikeArticle(articleID, userID)
	if err != nil {
		log.Printf("[api] unlike article error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to unlike article")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"liked": false,
	})
}

// --- Interaction Handlers ---

// HandleArticleInteractions returns interaction info for a single article.
// GET /api/v1/articles/{id}/interactions
// Header: X-User-Token: <uuid> (optional, for is_liked status)
func (s *Server) HandleArticleInteractions(w http.ResponseWriter, r *http.Request, articleIDStr string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	articleID, err := strconv.ParseInt(articleIDStr, 10, 64)
	if err != nil || articleID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid article ID")
		return
	}

	userID := s.getUserID(r)

	info, err := s.CommentStore.GetArticleInteractions(articleID, userID)
	if err != nil {
		log.Printf("[api] get interactions error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get interactions")
		return
	}

	writeJSON(w, http.StatusOK, info)
}

// HandleBatchInteractions returns interaction info for multiple articles.
// GET /api/v1/articles/interactions?ids=1,2,3,4,5
// Header: X-User-Token: <uuid> (optional)
func (s *Server) HandleBatchInteractions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
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

	if len(ids) > 100 {
		writeError(w, http.StatusBadRequest, "too many ids (max 100)")
		return
	}

	userID := s.getUserID(r)

	interactions, err := s.CommentStore.BatchGetInteractions(ids, userID)
	if err != nil {
		log.Printf("[api] batch interactions error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get batch interactions")
		return
	}

	// Convert to string keys for JSON
	result := make(map[string]store.InteractionInfo)
	for id, info := range interactions {
		result[strconv.FormatInt(id, 10)] = info
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"interactions": result,
	})
}
