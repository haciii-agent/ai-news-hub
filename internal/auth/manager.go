package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"ai-news-hub/config"
	"ai-news-hub/internal/store"
)

// AuthManager aggregates JWT, password, and rate limiter components.
type AuthManager struct {
	JWTSecret    []byte
	JWTExpiry    time.Duration
	BcryptCost   int
	Store        store.AuthStore
	RateLimiter  *LoginRateLimiter
	QueryLimiter *QueryRateLimiter
}

// NewAuthManager creates an AuthManager from config.
func NewAuthManager(cfg config.AuthConfig, authStore store.AuthStore) *AuthManager {
	secret := cfg.JWTSecret
	if secret == "" {
		// Auto-generate a random secret (changes on every restart)
		b := make([]byte, 32)
		rand.Read(b)
		secret = hex.EncodeToString(b)
	}

	expiry := cfg.JWTExpiry
	if expiry <= 0 {
		expiry = 168 * time.Hour // 7 days
	}
	cost := cfg.BcryptCost
	if cost <= 0 {
		cost = 10
	}

	return &AuthManager{
		JWTSecret:    []byte(secret),
		JWTExpiry:    expiry,
		BcryptCost:   cost,
		Store:        authStore,
		RateLimiter:  NewLoginRateLimiter(cfg),
		QueryLimiter: NewQueryRateLimiter(cfg),
	}
}

// GenerateToken creates a JWT for the given user.
func (m *AuthManager) GenerateToken(user *store.User) (*TokenPair, error) {
	username := ""
	if user.Username != nil {
		username = *user.Username
	}
	return GenerateToken(m.JWTSecret, m.JWTExpiry, user.ID, username, user.Role)
}

// ValidateToken verifies a JWT and returns its Claims.
func (m *AuthManager) ValidateToken(tokenStr string) (*Claims, error) {
	return ValidateToken(m.JWTSecret, tokenStr)
}

// RefreshToken validates an existing token and issues a new one.
func (m *AuthManager) RefreshToken(tokenStr string) (*TokenPair, error) {
	claims, err := m.ValidateToken(tokenStr)
	if err != nil {
		return nil, err
	}
	return GenerateToken(m.JWTSecret, m.JWTExpiry, claims.UserID, claims.Username, claims.Role)
}

// HashPassword creates a bcrypt hash of the plaintext password.
func (m *AuthManager) HashPassword(password string) (string, error) {
	return HashPassword(password, m.BcryptCost)
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func (m *AuthManager) CheckPassword(hashedPassword, password string) bool {
	return CheckPassword(hashedPassword, password)
}

// AuthMiddleware is the global authentication middleware.
// It injects user info into the request context without blocking.
// Priority: JWT Bearer > Anonymous Token > Guest.
func (m *AuthManager) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Try JWT from Authorization: Bearer <token>
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenStr := authHeader[7:]
			claims, err := m.ValidateToken(tokenStr)
			if err == nil {
				// Verify user still exists and is not disabled
				user, err := m.Store.GetUserByID(claims.UserID)
				if err == nil && user != nil && !user.Disabled {
					info := UserInfo{
						UserID: user.ID,
						Role:   user.Role,
						IsAuth: true,
						IsAnon: false,
					}
					next.ServeHTTP(w, setUserInfo(r, info))
					return
				}
				if err == nil && user != nil && user.Disabled {
					writeAuthError(w, http.StatusForbidden, "账号已被禁用")
					return
				}
			}
		}

		// 2. Try anonymous token from X-User-Token
		anonToken := r.Header.Get("X-User-Token")
		if anonToken != "" {
			user, err := m.Store.GetAnonUserByToken(anonToken)
			if err == nil && user != nil {
				// If the anonymous user has been merged, treat as guest
				if user.MergedInto != nil {
					info := UserInfo{
						UserID: 0,
						Role:   "guest",
						IsAuth: false,
						IsAnon: false,
					}
					next.ServeHTTP(w, setUserInfo(r, info))
					return
				}
				info := UserInfo{
					UserID: user.ID,
					Role:   "anonymous",
					IsAuth: false,
					IsAnon: true,
				}
				next.ServeHTTP(w, setUserInfo(r, info))
				return
			}
		}

		// 3. Guest (no token)
		info := UserInfo{
			UserID: 0,
			Role:   "guest",
			IsAuth: false,
			IsAnon: false,
		}
		next.ServeHTTP(w, setUserInfo(r, info))
	})
}

// writeAuthError is a lightweight JSON error writer for auth middleware.
func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":true,"message":"` + message + `"}`))
}
