package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"schedio/internal/config"
)

func newServicesConfig(services ...config.ServiceEntry) *config.Config {
	return &config.Config{Services: services}
}

func TestServicesHandler_Get(t *testing.T) {
	cfg := newServicesConfig(
		config.ServiceEntry{
			ID:              "aaaaaaaa-0001-4000-8000-000000000001",
			Name:            "Behandlung A",
			Summary:         "Kurzbeschreibung A",
			Description:     "Lange Beschreibung A",
			Price:           49.50,
			DurationMinutes: 60,
			DailyLimit:      3,
		},
		config.ServiceEntry{
			ID:              "aaaaaaaa-0002-4000-8000-000000000002",
			Name:            "Behandlung B",
			Summary:         "Kurzbeschreibung B",
			Description:     "Lange Beschreibung B",
			Price:           0,
			DurationMinutes: 30,
			DailyLimit:      0,
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	rec := httptest.NewRecorder()
	NewServicesHandler(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q; want %q", ct, "application/json; charset=utf-8")
	}

	var got []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 services, got %d", len(got))
	}

	// Verify first service fields
	s0 := got[0]
	if s0["id"] != "aaaaaaaa-0001-4000-8000-000000000001" {
		t.Errorf("id = %q; want %q", s0["id"], "aaaaaaaa-0001-4000-8000-000000000001")
	}
	if s0["name"] != "Behandlung A" {
		t.Errorf("name = %q; want %q", s0["name"], "Behandlung A")
	}
	if s0["summary"] != "Kurzbeschreibung A" {
		t.Errorf("summary = %q; want %q", s0["summary"], "Kurzbeschreibung A")
	}
	if s0["description"] != "Lange Beschreibung A" {
		t.Errorf("description = %q; want %q", s0["description"], "Lange Beschreibung A")
	}
	if s0["price"] != 49.50 {
		t.Errorf("price = %v; want 49.50", s0["price"])
	}
	if s0["duration_minutes"] != float64(60) {
		t.Errorf("duration_minutes = %v; want 60", s0["duration_minutes"])
	}

	// daily_limit must not be present in the public response
	if _, ok := s0["daily_limit"]; ok {
		t.Error("daily_limit must not be present in the public services response")
	}
}

func TestServicesHandler_EmptyList(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	rec := httptest.NewRecorder()
	NewServicesHandler(&config.Config{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got []any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty array, got %v", got)
	}
}

func TestServicesHandler_MethodNotAllowed(t *testing.T) {
	cfg := newServicesConfig()
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/v1/services", nil)
		rec := httptest.NewRecorder()
		NewServicesHandler(cfg).ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d; want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}
