package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"ai-news-hub/internal/auth"
	"ai-news-hub/internal/store"
)

// HandleAdminListUsers handles GET /api/v1/admin/users
// Returns a paginated list of users (admin only).
func (s *Server) HandleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	if err := auth.RequireRole(r, "admin"); err != nil {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	q := r.URL.Query()

	// Parse page (default 1)
	page := 1
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v >= 1 {
			page = v
		}
	}

	// Parse per_page (default 20, max 100)
	perPage := 20
	if pp := q.Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v >= 1 && v <= 100 {
			perPage = v
		}
	}

	search := strings.TrimSpace(q.Get("search"))

	users, total, err := s.AdminMgr.ListUsers(page, perPage, search)
	if err != nil {
		log.Printf("[admin] list users error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	// Sanitize response — remove tokens and password hashes
	for i := range users {
		users[i].Token = ""
		users[i].PasswordHash = ""
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"users":    users,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// adminRouter routes /api/v1/admin/users/* requests.
func (s *Server) adminRouter(w http.ResponseWriter, r *http.Request) {
	if err := auth.RequireRole(r, "admin"); err != nil {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")
	path = strings.TrimSuffix(path, "/")

	// /api/v1/admin/users/{id}/role
	if strings.HasSuffix(path, "/role") {
		s.HandleAdminUpdateRole(w, r)
		return
	}

	// /api/v1/admin/users/{id}/status
	if strings.HasSuffix(path, "/status") {
		s.HandleAdminUpdateStatus(w, r)
		return
	}

	// /api/v1/admin/users/{id} — detail
	s.HandleAdminGetUser(w, r)
}

// HandleAdminGetUser handles GET /api/v1/admin/users/{id}
func (s *Server) HandleAdminGetUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	id, err := s.extractUserIDFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	detail, err := s.AdminMgr.GetUserDetail(id)
	if err != nil {
		log.Printf("[admin] get user detail error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get user detail")
		return
	}
	if detail == nil {
		writeError(w, http.StatusNotFound, "用户不存在")
		return
	}

	// Sanitize
	detail.Token = ""
	detail.PasswordHash = ""

	writeJSON(w, http.StatusOK, detail)
}

// HandleAdminUpdateRole handles PUT /api/v1/admin/users/{id}/role
func (s *Server) HandleAdminUpdateRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "only PUT allowed")
		return
	}

	id, err := s.extractUserIDFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Role = strings.TrimSpace(req.Role)
	if req.Role != "viewer" && req.Role != "editor" && req.Role != "admin" {
		writeError(w, http.StatusBadRequest, "无效的角色值")
		return
	}

	// Last admin protection: check if target is admin and being demoted
	targetUser, err := s.AuthStore.GetUserByID(id)
	if err != nil {
		log.Printf("[admin] get user error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if targetUser == nil {
		writeError(w, http.StatusNotFound, "用户不存在")
		return
	}

	if targetUser.Role == "admin" && req.Role != "admin" {
		adminCount, err := s.AdminMgr.CountUsersByRole("admin")
		if err != nil {
			log.Printf("[admin] count admins error: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to count admins")
			return
		}
		if adminCount <= 1 {
			writeError(w, http.StatusBadRequest, "不能降级最后一个管理员")
			return
		}
	}

	if err := s.AdminMgr.UpdateUserRole(id, req.Role); err != nil {
		log.Printf("[admin] update role error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update role")
		return
	}

	log.Printf("[admin] user %d role changed to %s by admin %d", id, req.Role, auth.GetUserInfo(r).UserID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// HandleAdminUpdateStatus handles PUT /api/v1/admin/users/{id}/status
func (s *Server) HandleAdminUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "only PUT allowed")
		return
	}

	id, err := s.extractUserIDFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req struct {
		Disabled bool `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Last admin protection: cannot disable the last admin
	if req.Disabled {
		targetUser, err := s.AuthStore.GetUserByID(id)
		if err != nil {
			log.Printf("[admin] get user error: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to get user")
			return
		}
		if targetUser == nil {
			writeError(w, http.StatusNotFound, "用户不存在")
			return
		}

		if targetUser.Role == "admin" {
			adminCount, err := s.AdminMgr.CountUsersByRole("admin")
			if err != nil {
				log.Printf("[admin] count admins error: %v", err)
				writeError(w, http.StatusInternalServerError, "failed to count admins")
				return
			}
			if adminCount <= 1 {
				writeError(w, http.StatusBadRequest, "不能禁用最后一个管理员")
				return
			}
		}
	}

	if err := s.AdminMgr.UpdateUserStatus(id, req.Disabled); err != nil {
		log.Printf("[admin] update status error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update status")
		return
	}

	log.Printf("[admin] user %d status changed to disabled=%v by admin %d", id, req.Disabled, auth.GetUserInfo(r).UserID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// extractUserIDFromPath extracts user ID from /api/v1/admin/users/{id}[/...]
func (s *Server) extractUserIDFromPath(r *http.Request) (int64, error) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")
	path = strings.TrimSuffix(path, "/")

	// Strip trailing segments for /role and /status
	if idx := strings.Index(path, "/"); idx != -1 {
		path = path[:idx]
	}

	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID")
	}
	return id, nil
}
