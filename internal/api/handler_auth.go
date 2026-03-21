package api

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"

	"ai-news-hub/internal/auth"
	"ai-news-hub/internal/store"
)

// --- Validation helpers ---

var (
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\p{Han}]{3,20}$`)
	emailRegex   = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	passwordRegex = regexp.MustCompile(`^[a-zA-Z0-9!@#$%^&*()_+\-=\[\]{}|;':",.<>?/~` + "`" + `]{8,128}$`)
)

// --- Auth Handlers ---

// HandleRegister handles user registration.
// POST /api/v1/auth/register
func (s *Server) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate username
	if !usernameRegex.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, "用户名必须为3-20个字符，仅允许字母、数字、下划线和中文")
		return
	}

	// Validate email
	if !emailRegex.MatchString(req.Email) {
		writeError(w, http.StatusBadRequest, "邮箱格式不合法")
		return
	}

	// Validate password: >=8 chars, must contain uppercase, lowercase, and digit
	if len(req.Password) < 8 || len(req.Password) > 128 {
		writeError(w, http.StatusBadRequest, "密码长度必须为8-128个字符")
		return
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range req.Password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		writeError(w, http.StatusBadRequest, "密码必须包含大写字母、小写字母和数字")
		return
	}

	// Check uniqueness
	exists, err := s.AuthStore.CheckUsernameExists(req.Username)
	if err != nil {
		log.Printf("[api] check username exists error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check username")
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "用户名已被使用")
		return
	}

	exists, err = s.AuthStore.CheckEmailExists(req.Email)
	if err != nil {
		log.Printf("[api] check email exists error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check email")
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "邮箱已被注册")
		return
	}

	// Hash password
	passwordHash, err := s.AuthMgr.HashPassword(req.Password)
	if err != nil {
		log.Printf("[api] hash password error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to process password")
		return
	}

	// Begin transaction for user creation + optional data migration
	tx, err := s.DB.Begin()
	if err != nil {
		log.Printf("[api] begin transaction error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	defer tx.Rollback()

	// Create user
	user, err := s.AuthStore.CreateUser(tx, req.Username, req.Email, passwordHash)
	if err != nil {
		log.Printf("[api] create user error: %v", err)
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			writeError(w, http.StatusConflict, "用户名或邮箱已存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Migrate anonymous user data if X-User-Token is present
	anonToken := r.Header.Get("X-User-Token")
	if anonToken != "" {
		anonUser, err := s.AuthStore.GetAnonUserByToken(anonToken)
		if err != nil {
			log.Printf("[api] get anon user error: %v", err)
		} else if anonUser != nil && anonUser.MergedInto == nil && anonUser.ID != user.ID {
			if err := s.AuthStore.MigrateAnonUser(tx, anonUser.ID, user.ID); err != nil {
				log.Printf("[api] migrate anon user error: %v", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[api] commit transaction error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Generate JWT
	token, err := s.AuthMgr.GenerateToken(user)
	if err != nil {
		log.Printf("[api] generate token error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	log.Printf("[api] user registered: %s (id=%d)", req.Username, user.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

// HandleLogin handles user login.
// POST /api/v1/auth/login
func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Login == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "用户名和密码不能为空")
		return
	}

	ip := getClientIP(r)

	// Check rate limit (read-only, does NOT increment counters)
	allowed, retryAfter := s.AuthMgr.RateLimiter.CheckLoginRate(ip, req.Login)
	if !allowed {
		writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
			"error":      true,
			"message":    "登录尝试过于频繁，请稍后再试",
			"retry_after": int(retryAfter.Seconds()),
		})
		return
	}

	// Find user
	user, err := s.AuthStore.GetUserByUsernameOrEmail(req.Login)
	if err != nil {
		log.Printf("[api] login query error: %v", err)
		writeError(w, http.StatusInternalServerError, "登录失败")
		return
	}
	if user == nil || user.PasswordHash == "" {
		s.AuthMgr.RateLimiter.RecordLoginFailure(ip, req.Login)
		s.recordLoginLog(nil, req.Login, ip, r.UserAgent(), false, "用户名或密码错误")
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	// Check if account is disabled
	if user.Disabled {
		s.recordLoginLog(&user.ID, req.Login, ip, r.UserAgent(), false, "账号已被禁用")
		writeError(w, http.StatusForbidden, "账号已被禁用")
		return
	}

	// Verify password
	if !s.AuthMgr.CheckPassword(user.PasswordHash, req.Password) {
		s.AuthMgr.RateLimiter.RecordLoginFailure(ip, req.Login)
		s.recordLoginLog(&user.ID, req.Login, ip, r.UserAgent(), false, "密码错误")
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	// Success — clear rate limits
	s.AuthMgr.RateLimiter.RecordLoginSuccess(ip, req.Login)

	// Update last seen
	_ = s.AuthStore.UpdateLastSeen(user.ID)

	// Generate JWT
	token, err := s.AuthMgr.GenerateToken(user)
	if err != nil {
		log.Printf("[api] generate token error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	s.recordLoginLog(&user.ID, req.Login, ip, r.UserAgent(), true, "")

	log.Printf("[api] user logged in: %s (id=%d)", req.Login, user.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

// HandleMe returns the current user info.
// GET /api/v1/auth/me
func (s *Server) HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	info := auth.GetUserInfo(r)

	if info.IsAuth {
		// Authenticated user — fetch fresh data from DB
		user, err := s.AuthStore.GetUserByID(info.UserID)
		if err != nil || user == nil {
			writeError(w, http.StatusUnauthorized, "用户不存在")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"user": user,
		})
		return
	}

	if info.IsAnon {
		// Anonymous user
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"anonymous": true,
			"user_id":   info.UserID,
		})
		return
	}

	// Guest
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"guest": true,
	})
}

// HandleRefresh refreshes the JWT token.
// POST /api/v1/auth/refresh
func (s *Server) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	authHeader := r.Header.Get("Authorization")
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		writeError(w, http.StatusUnauthorized, "未认证")
		return
	}

	tokenStr := authHeader[7:]
	token, err := s.AuthMgr.RefreshToken(tokenStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Token无效或已过期")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
	})
}

// HandleLogout handles user logout.
// POST /api/v1/auth/logout
func (s *Server) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	// JWT is stateless; logout is primarily for logging purposes
	// Client should discard the token
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// HandleCheckUsername checks if a username is available.
// GET /api/v1/auth/check-username?username=xxx
func (s *Server) HandleCheckUsername(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	ip := getClientIP(r)
	if allowed, retryAfter := s.AuthMgr.QueryLimiter.CheckQueryRate(ip); !allowed {
		writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
			"error":      true,
			"message":    "请求过于频繁",
			"retry_after": int(retryAfter.Seconds()),
		})
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username parameter is required")
		return
	}

	exists, err := s.AuthStore.CheckUsernameExists(username)
	if err != nil {
		log.Printf("[api] check username error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check username")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"available": !exists,
	})
}

// HandleCheckEmail checks if an email is available.
// GET /api/v1/auth/check-email?email=xxx
func (s *Server) HandleCheckEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	ip := getClientIP(r)
	if allowed, retryAfter := s.AuthMgr.QueryLimiter.CheckQueryRate(ip); !allowed {
		writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
			"error":      true,
			"message":    "请求过于频繁",
			"retry_after": int(retryAfter.Seconds()),
		})
		return
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		writeError(w, http.StatusBadRequest, "email parameter is required")
		return
	}

	exists, err := s.AuthStore.CheckEmailExists(email)
	if err != nil {
		log.Printf("[api] check email error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check email")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"available": !exists,
	})
}

// HandleUpdatePassword handles password change for authenticated users.
// PUT /api/v1/user/password
func (s *Server) HandleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "only PUT allowed")
		return
	}

	if err := auth.RequireAuth(r); err != nil {
		writeError(w, http.StatusUnauthorized, "未认证")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "旧密码和新密码不能为空")
		return
	}

	// Validate new password
	if len(req.NewPassword) < 8 || len(req.NewPassword) > 128 {
		writeError(w, http.StatusBadRequest, "密码长度必须为8-128个字符")
		return
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range req.NewPassword {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		writeError(w, http.StatusBadRequest, "密码必须包含大写字母、小写字母和数字")
		return
	}

	info := auth.GetUserInfo(r)

	// Get current user
	user, err := s.AuthStore.GetUserByID(info.UserID)
	if err != nil || user == nil {
		writeError(w, http.StatusInternalServerError, "用户不存在")
		return
	}

	// Verify old password
	if !s.AuthMgr.CheckPassword(user.PasswordHash, req.OldPassword) {
		writeError(w, http.StatusBadRequest, "当前密码错误")
		return
	}

	// Hash and update new password
	newHash, err := s.AuthMgr.HashPassword(req.NewPassword)
	if err != nil {
		log.Printf("[api] hash password error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to process password")
		return
	}

	if err := s.AuthStore.UpdatePassword(info.UserID, newHash); err != nil {
		log.Printf("[api] update password error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// --- Helpers ---

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// recordLoginLog records a login attempt to the login_logs table.
func (s *Server) recordLoginLog(userID *int64, username, ip, userAgent string, success bool, failReason string) {
	if s.LoginLogStore == nil {
		return
	}
	logEntry := &store.LoginLog{
		UserID:     userID,
		Username:   username,
		IPAddress:  ip,
		UserAgent:  userAgent,
		Success:    success,
		FailReason: failReason,
	}
	if err := s.LoginLogStore.RecordLog(logEntry); err != nil {
		log.Printf("[api] record login log error: %v", err)
	}
}
