package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"schedio/internal/store"
)

const cookieName = "schedio_session"

// cookiePayload is the JSON body embedded in the session cookie.
type cookiePayload struct {
	UserID   string    `json:"uid"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	IssuedAt time.Time `json:"iat"`
}

// SessionManager signs and validates session cookies.
type SessionManager struct {
	signingKey []byte // separate from the booking HMAC secret
	secure     bool   // Secure flag on Set-Cookie
	maxAge     time.Duration
}

// NewSessionManager creates a SessionManager. signingKey must be at least
// 32 bytes; use crypto/rand to generate it.
func NewSessionManager(signingKey []byte, secure bool) *SessionManager {
	return &SessionManager{
		signingKey: signingKey,
		secure:     secure,
		maxAge:     24 * time.Hour,
	}
}

// GenerateSigningKey generates a random 32-byte session signing key.
func GenerateSigningKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("auth: generate signing key: %w", err)
	}
	return key, nil
}

// SetCookie writes a signed session cookie for the given user to w.
func (sm *SessionManager) SetCookie(w http.ResponseWriter, user *store.User) error {
	payload := cookiePayload{
		UserID:   user.ID,
		Email:    user.Email,
		Role:     string(user.Role),
		IssuedAt: time.Now().UTC(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	sig := sm.sign(raw)
	value := base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(sig)

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(sm.maxAge.Seconds()),
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// ClearCookie expires the session cookie.
func (sm *SessionManager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ValidateCookie parses and validates the session cookie from r.
// Returns ErrUnauthenticated when the cookie is absent, expired, or tampered.
func (sm *SessionManager) ValidateCookie(r *http.Request) (*Session, error) {
	cookie, err := r.Cookie(cookieName)
	if errors.Is(err, http.ErrNoCookie) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, ErrUnauthenticated
	}

	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		return nil, ErrUnauthenticated
	}
	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrUnauthenticated
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrUnauthenticated
	}
	if !hmac.Equal(sm.sign(rawPayload), sig) {
		return nil, ErrUnauthenticated
	}

	var payload cookiePayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return nil, ErrUnauthenticated
	}
	if time.Since(payload.IssuedAt) > sm.maxAge {
		return nil, ErrUnauthenticated
	}

	return &Session{
		UserID: payload.UserID,
		Email:  payload.Email,
		Role:   store.UserRole(payload.Role),
	}, nil
}

// RequireAuth returns middleware that validates the session cookie and attaches
// the Session to the request context. API requests receive 401 JSON; browser
// requests are redirected to /auth/login.
func (sm *SessionManager) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := sm.ValidateCookie(r)
		if err != nil {
			if acceptsHTML(r) {
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
				return
			}
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		ctx := context.WithValue(r.Context(), contextKey{}, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole returns middleware that checks the session role. It must be
// chained after RequireAuth.
func RequireRole(role store.UserRole, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := SessionFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if session.Role != role {
			writeError(w, http.StatusForbidden, "insufficient privileges")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// sign returns the HMAC-SHA256 of data using the manager's signing key.
func (sm *SessionManager) sign(data []byte) []byte {
	mac := hmac.New(sha256.New, sm.signingKey)
	mac.Write(data)
	return mac.Sum(nil)
}

// acceptsHTML reports whether the request prefers an HTML response.
func acceptsHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}
