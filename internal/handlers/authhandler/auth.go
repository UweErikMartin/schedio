// Package authhandler implements the HTTP handlers for authentication:
// password login/logout and Apple Sign-In OAuth2 flow.
//
// Route prefix: /auth/ (registered by the server router).
package authhandler

import (
	"encoding/json"
	"errors"
	"net/http"

	"schedio/internal/auth"
	"schedio/internal/store"
)

// Handler groups the auth HTTP handlers.
type Handler struct {
	sessions      *auth.SessionManager
	authenticator *auth.Authenticator
	store         store.DomainStore
}

// NewHandler constructs an auth Handler.
func NewHandler(sessions *auth.SessionManager, authenticator *auth.Authenticator, st store.DomainStore) *Handler {
	return &Handler{sessions: sessions, authenticator: authenticator, store: st}
}

// Login handles POST /auth/login.
//
// Accepts JSON body {"email":"…","password":"…"} and sets a signed session
// cookie on success. Returns 400 for malformed input, 401 for bad credentials.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if body.Email == "" || body.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	user, err := h.authenticator.Authenticate(r.Context(), body.Email, body.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.sessions.SetCookie(w, user); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"email": user.Email,
		"role":  string(user.Role),
	})
}

// Logout handles POST /auth/logout.
//
// Clears the session cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.sessions.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
