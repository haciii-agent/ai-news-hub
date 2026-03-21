package api

import (
	"net/http"

	"ai-news-hub/internal/auth"
)

// HandleAdminListUsers is a placeholder for Worker-B to implement.
// GET /api/v1/admin/users
func (s *Server) HandleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	// TODO: Implemented by Worker-B
	if err := auth.RequireRole(r, "admin"); err != nil {
		if err == auth.ErrUnauthorized {
			writeError(w, http.StatusUnauthorized, "未认证")
		} else {
			writeError(w, http.StatusForbidden, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]interface{}{
		"error":   true,
		"message": "admin users list not yet implemented",
	})
}

// adminRouter routes /api/v1/admin/users/... requests.
// Placeholder — Worker-B will implement the actual handlers.
func (s *Server) adminRouter(w http.ResponseWriter, r *http.Request) {
	// TODO: Implemented by Worker-B
	if err := auth.RequireRole(r, "admin"); err != nil {
		if err == auth.ErrUnauthorized {
			writeError(w, http.StatusUnauthorized, "未认证")
		} else {
			writeError(w, http.StatusForbidden, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]interface{}{
		"error":   true,
		"message": "admin user management not yet implemented",
	})
}
