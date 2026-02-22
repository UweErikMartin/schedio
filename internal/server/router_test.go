package server

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"schedio/internal/config"
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

func TestRouter_ServesOpenAPISpec(t *testing.T) {
	args := config.Config{
		RootPath: randomRootPath(),
	}
	router := NewRouter(&args, http.NotFoundHandler())

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
	router := NewRouter(&args, http.NotFoundHandler())

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
	router := NewRouter(&args, http.NotFoundHandler())

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
