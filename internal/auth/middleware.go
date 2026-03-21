package auth

import (
	"context"
	"errors"
	"net/http"
)

// Predefined errors for authorization checks.
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("insufficient permissions")
)

type contextKey string

// Context keys for storing user info in request context.
const (
	ContextKeyUserID contextKey = "user_id"
	ContextKeyRole   contextKey = "role"
	ContextKeyIsAuth contextKey = "is_auth"
	ContextKeyIsAnon contextKey = "is_anon"
)

// UserInfo holds information about the current request's user.
type UserInfo struct {
	UserID int64
	Role   string
	IsAuth bool // JWT-authenticated user
	IsAnon bool // Anonymous token user
}

// GetUserInfo extracts user info from the request context.
func GetUserInfo(r *http.Request) UserInfo {
	info := UserInfo{}
	if v := r.Context().Value(ContextKeyUserID); v != nil {
		info.UserID = v.(int64)
	}
	if v := r.Context().Value(ContextKeyRole); v != nil {
		info.Role = v.(string)
	}
	if v := r.Context().Value(ContextKeyIsAuth); v != nil {
		info.IsAuth = v.(bool)
	}
	if v := r.Context().Value(ContextKeyIsAnon); v != nil {
		info.IsAnon = v.(bool)
	}
	return info
}

// RequireAuth checks that the request is from an authenticated (JWT) user.
func RequireAuth(r *http.Request) error {
	info := GetUserInfo(r)
	if !info.IsAuth {
		return ErrUnauthorized
	}
	return nil
}

// RequireRole checks that the request is from a user with one of the specified roles.
func RequireRole(r *http.Request, roles ...string) error {
	info := GetUserInfo(r)
	if !info.IsAuth {
		return ErrUnauthorized
	}
	for _, role := range roles {
		if info.Role == role {
			return nil
		}
	}
	return ErrForbidden
}

// setUserInfo stores user info in the request context.
func setUserInfo(r *http.Request, info UserInfo) *http.Request {
	ctx := context.WithValue(r.Context(), ContextKeyUserID, info.UserID)
	ctx = context.WithValue(ctx, ContextKeyRole, info.Role)
	ctx = context.WithValue(ctx, ContextKeyIsAuth, info.IsAuth)
	ctx = context.WithValue(ctx, ContextKeyIsAnon, info.IsAnon)
	return r.WithContext(ctx)
}
