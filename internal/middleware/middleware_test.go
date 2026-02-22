package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestLoggingMiddleware_PassesThroughResponse(t *testing.T) {
	var called int32

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		w.Header().Set("X-Test", "yes")
		w.WriteHeader(http.StatusTeapot) // 418
		_, _ = w.Write([]byte("hello"))
	})

	handler := LoggingMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if c := atomic.LoadInt32(&called); c != 1 {
		t.Fatalf("next handler called %d times, want 1", c)
	}
	if got, want := rec.Code, http.StatusTeapot; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := rec.Body.String(), "hello"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("X-Test"), "yes"; got != want {
		t.Fatalf("header X-Test = %q, want %q", got, want)
	}
}

func TestLoggingMiddleware_DefaultsTo200WhenNoWriteHeader(t *testing.T) {
	var called int32

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		_, _ = w.Write([]byte("ok")) // no explicit WriteHeader; should default to 200
	})

	handler := LoggingMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if c := atomic.LoadInt32(&called); c != 1 {
		t.Fatalf("next handler called %d times, want 1", c)
	}
	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := rec.Body.String(), "ok"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
