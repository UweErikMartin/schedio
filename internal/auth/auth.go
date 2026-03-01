// Package auth manages authentication and session state for admin and staff
// users. It provides:
//
//   - Password-based login via bcrypt (password.go)
//   - Apple Sign-In via OAuth 2.0 / OIDC (apple.go)
//   - HTTP-only signed session cookies (session.go)
//
// The cookie value is a base64-encoded JSON payload plus an HMAC-SHA256
// signature. Cookies carry SameSite=Lax; HttpOnly; Secure attributes.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"schedio/internal/store"
)

// contextKey is an unexported type used to attach the session to a request context.
type contextKey struct{}

// Session holds the authenticated identity attached to an incoming request.
type Session struct {
	UserID string
	Email  string
	Role   store.UserRole
}

// ErrUnauthenticated is returned when no valid session cookie is present.
var ErrUnauthenticated = errors.New("auth: unauthenticated")

// ErrForbidden is returned when the session is valid but lacks the required role.
var ErrForbidden = errors.New("auth: forbidden")

// SessionFromContext returns the Session attached by RequireAuth middleware.
// Returns nil, ErrUnauthenticated when no session is present.
func SessionFromContext(ctx context.Context) (*Session, error) {
	s, ok := ctx.Value(contextKey{}).(*Session)
	if !ok || s == nil {
		return nil, ErrUnauthenticated
	}
	return s, nil
}

// writeJSON writes v as a JSON response with statusCode.
func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response: {"error": message}.
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}
