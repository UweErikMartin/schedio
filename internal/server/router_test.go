package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"schedio/internal/config"
	calstore "schedio/internal/store"
	"schedio/internal/token"
	"strings"
	"testing"
)

func randomPort() int {
	return 8000 + rand.Intn(1000)
}

func randomRootPath() string {
	paths := []string{"", "/app", "/api-root", "/v1", "/schedio"}
	return paths[rand.Intn(len(paths))]
}

func newTestRouter(args config.Config) http.Handler {
	st := calstore.NewMemoryStore()
	signer, err := token.NewSigner(context.Background(), st)
	if err != nil {
		panic("newTestRouter: " + err.Error())
	}
	return NewRouter(&args, st, st, signer)
}

func TestRouter_ServesOpenAPISpec(t *testing.T) {
	args := config.Config{
		RootPath: randomRootPath(),
	}
	router := newTestRouter(args)

	port := randomPort()
	rootPath := args.RootPath

	hostPort := fmt.Sprintf("example.com:%d", port)
	apiPath := fmt.Sprintf("%s/api/docs/openapi.yaml", rootPath)
	if rootPath == "" {
		apiPath = "/api/docs/openapi.yaml"
	}

	url := fmt.Sprintf("http://%s%s", hostPort, apiPath)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("port=%d, rootPath=%q: status = %d, want %d (body: %s)", port, rootPath, rec.Code, http.StatusOK, rec.Body.String())
	}

	if ct := strings.ToLower(rec.Header().Get("Content-Type")); !strings.Contains(ct, "yaml") {
		t.Fatalf("port=%d, rootPath=%q: content-type = %q, want yaml", port, rootPath, rec.Header().Get("Content-Type"))
	}

	body := rec.Body.String()
	if !strings.Contains(body, "openapi:") {
		t.Fatalf("port=%d, rootPath=%q: response body does not look like OpenAPI YAML: %q", port, rootPath, body[:min(len(body), 200)])
	}
}

func TestRouter_ServesSwaggerUIIndex(t *testing.T) {
	args := config.Config{
		RootPath: randomRootPath(),
		Port:     randomPort(),
	}
	router := newTestRouter(args)

	port := args.Port
	rootPath := args.RootPath

	hostPort := fmt.Sprintf("example.com:%d", port)
	apiPath := fmt.Sprintf("%s/api/docs/index.html", rootPath)
	if rootPath == "" {
		apiPath = "/api/docs/index.html"
	}

	url := fmt.Sprintf("http://%s%s", hostPort, apiPath)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("port=%d, rootPath=%q: status = %d, want %d (body: %s)", port, rootPath, rec.Code, http.StatusOK, rec.Body.String())
	}

	ct := strings.ToLower(rec.Header().Get("Content-Type"))
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("port=%d, rootPath=%q: content-type = %q, want text/html", port, rootPath, rec.Header().Get("Content-Type"))
	}

	body := strings.ToLower(rec.Body.String())
	if !strings.Contains(body, "swagger") {
		t.Fatalf("port=%d, rootPath=%q: response body does not appear to be Swagger UI HTML: %q", port, rootPath, rec.Body.String()[:min(len(rec.Body.String()), 200)])
	}
}

func TestRouter_ServesWebUIWithRootPathPrefix(t *testing.T) {
	args := config.Config{
		RootPath: "/ui",
	}
	router := newTestRouter(args)

	t.Run("index", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/ui/", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
		}

		if ct := strings.ToLower(rec.Header().Get("Content-Type")); !strings.Contains(ct, "text/html") {
			t.Fatalf("content-type = %q, want text/html", rec.Header().Get("Content-Type"))
		}
	})

	t.Run("asset", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/ui/js/default.js", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
		}

		if ct := strings.ToLower(rec.Header().Get("Content-Type")); !strings.Contains(ct, "javascript") {
			t.Fatalf("content-type = %q, want javascript", rec.Header().Get("Content-Type"))
		}
	})
}

func TestRouter_ServesPublicServices(t *testing.T) {
	rootPath := randomRootPath()
	args := config.Config{
		RootPath: rootPath,
		Services: []config.ServiceEntry{
			{
				ID:              "bbbbbbbb-0001-4000-8000-000000000001",
				Name:            "Test Service",
				Summary:         "A summary",
				Description:     "A description",
				Price:           25.00,
				DurationMinutes: 45,
				DailyLimit:      5,
			},
		},
	}
	router := newTestRouter(args)

	path := fmt.Sprintf("%s/api/v1/services", rootPath)
	req := httptest.NewRequest(http.MethodGet, "http://example.com"+path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("rootPath=%q: status = %d, want %d (body: %s)", rootPath, rec.Code, http.StatusOK, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(strings.ToLower(ct), "application/json") {
		t.Errorf("rootPath=%q: Content-Type = %q; want application/json", rootPath, ct)
	}

	var got []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("rootPath=%q: failed to decode JSON: %v", rootPath, err)
	}
	if len(got) != 1 {
		t.Fatalf("rootPath=%q: expected 1 service, got %d", rootPath, len(got))
	}
	if got[0]["name"] != "Test Service" {
		t.Errorf("rootPath=%q: name = %q; want %q", rootPath, got[0]["name"], "Test Service")
	}
	if got[0]["id"] != "bbbbbbbb-0001-4000-8000-000000000001" {
		t.Errorf("rootPath=%q: id = %q; want %q", rootPath, got[0]["id"], "bbbbbbbb-0001-4000-8000-000000000001")
	}
	if _, ok := got[0]["daily_limit"]; ok {
		t.Errorf("rootPath=%q: daily_limit must not be present in the public response", rootPath)
	}
}
