package authhandler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"schedio/internal/auth"
	"schedio/internal/store"
)

// newTestHandler builds a Handler backed by an in-memory store that contains
// one staff user and one administrator user with known passwords (hashed at
// bcrypt.MinCost to keep tests fast).
func newTestHandler(t *testing.T) (*Handler, *store.MemoryStore) {
	t.Helper()

	hashFor := func(password string) string {
		h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		if err != nil {
			t.Fatalf("bcrypt: %v", err)
		}
		return string(h)
	}

	users := []*store.User{
		{
			ID:           "uid-staff-1",
			Email:        "staff@example.de",
			PasswordHash: hashFor("staffpass"),
			Role:         store.UserRoleStaff,
			CreatedAt:    time.Now(),
		},
		{
			ID:           "uid-admin-1",
			Email:        "admin@example.de",
			PasswordHash: hashFor("adminpass"),
			Role:         store.UserRoleAdministrator,
			CreatedAt:    time.Now(),
		},
	}

	st := store.NewMemoryStore()
	if err := st.SyncUsers(context.Background(), users); err != nil {
		t.Fatalf("SyncUsers: %v", err)
	}

	key, err := auth.GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey: %v", err)
	}
	sessions := auth.NewSessionManager(key, false)
	authenticator := auth.NewAuthenticator(st)

	return NewHandler(sessions, authenticator, st), st
}

func postLogin(handler *Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.Login(rec, req)
	return rec
}

// ── Login success ─────────────────────────────────────────────────────────────

func TestLogin_StaffSuccess(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postLogin(h, `{"email":"staff@example.de","password":"staffpass"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q; want application/json", ct)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["email"] != "staff@example.de" {
		t.Errorf("email = %q; want %q", resp["email"], "staff@example.de")
	}
	if resp["role"] != string(store.UserRoleStaff) {
		t.Errorf("role = %q; want %q", resp["role"], store.UserRoleStaff)
	}
}

func TestLogin_AdminSuccess(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postLogin(h, `{"email":"admin@example.de","password":"adminpass"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["role"] != string(store.UserRoleAdministrator) {
		t.Errorf("role = %q; want %q", resp["role"], store.UserRoleAdministrator)
	}
}

func TestLogin_SetsSessionCookie(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postLogin(h, `{"email":"staff@example.de","password":"staffpass"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "schedio_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected schedio_session cookie, got none")
	}
	if !sessionCookie.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
	if sessionCookie.MaxAge <= 0 {
		t.Errorf("session cookie MaxAge = %d; want > 0", sessionCookie.MaxAge)
	}
}

// ── Login failure ─────────────────────────────────────────────────────────────

func TestLogin_WrongPassword(t *testing.T) {
	h, _ := newTestHandler(t)
	// constantDelay adds ~300ms - acceptable for one test
	rec := postLogin(h, `{"email":"staff@example.de","password":"wrong"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	h, _ := newTestHandler(t)
	// constantDelay adds ~300ms - acceptable for one test
	rec := postLogin(h, `{"email":"nobody@example.de","password":"irrelevant"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}
}

// ── Input validation ──────────────────────────────────────────────────────────

func TestLogin_MissingEmail(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postLogin(h, `{"password":"staffpass"}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLogin_MissingPassword(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postLogin(h, `{"email":"staff@example.de"}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLogin_EmptyBody(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postLogin(h, `{}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postLogin(h, `not-json`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

// ── Logout ────────────────────────────────────────────────────────────────────

func TestLogout_ClearsCookie(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rec := httptest.NewRecorder()
	h.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNoContent)
	}

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "schedio_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected schedio_session cookie in response (to clear it), got none")
	}
	if sessionCookie.MaxAge != -1 {
		t.Errorf("session cookie MaxAge = %d; want -1 (delete)", sessionCookie.MaxAge)
	}
}
