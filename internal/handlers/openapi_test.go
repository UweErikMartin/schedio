package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"schedio/api"
)

// Test that the OpenAPI spec is served via /rootPath/api/docs/openapi.yaml
func TestServeOpenApiSpec_YAML(t *testing.T) {
	openApiSpec := api.OpenApiSpec
	if len(openApiSpec) == 0 {
		t.Fatal("OpenApiSpec is empty")
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com:8080/r/api/docs/openapi.yaml", nil)
	rec := httptest.NewRecorder()

	NewOpenAPIHandler("/r").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Read the response
	body, _ := io.ReadAll(rec.Body)
	s := string(body)

	// Basic sanity checks: looks like OpenAPI YAML and contains the rendered server URL
	if !strings.Contains(s, "openapi:") {
		t.Fatalf("response does not look like OpenAPI YAML: %q", s[:min(len(s), 120)])
	}

	if !strings.Contains(s, "example.com:8080/r") {
		t.Fatalf("rendered server URL not found in response. body snippet: %q", s[:min(len(s), 160)])
	}
}

// Test that the Swagger UI HTML is served at /rootPath/api/docs/
func TestServeOpenApiDocs_HTML(t *testing.T) {

	HandleOpenAPI := NewOpenAPIHandler("/r")

	// Request the docs path with slash; handler may directly serve HTML or redirect to index.html
	req := httptest.NewRequest(http.MethodGet, "http://example.com:8080/r/api/docs/", nil)
	rec := httptest.NewRecorder()
	HandleOpenAPI.ServeHTTP(rec, req)

	switch rec.Code {
	case http.StatusOK:
		// OK, proceed to validate content
		ct := rec.Header().Get("Content-Type")
		if ct == "" || !strings.Contains(strings.ToLower(ct), "text/html") {
			t.Fatalf("content-type = %q, want to contain text/html", ct)
		}
		body := rec.Body.String()
		if !(strings.Contains(strings.ToLower(body), "<!doctype") || strings.Contains(strings.ToLower(body), "<html")) {
			t.Fatalf("response body does not look like HTML: first 200 chars: %q", body[:min(len(body), 200)])
		}
	case http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect:
		// Follow redirect to index.html
		loc := rec.Header().Get("Location")
		if loc == "" || !strings.HasSuffix(loc, "/r/api/docs/index.html") {
			t.Fatalf("/r/api/docs/ redirect Location = %q, want to end with /r/api/docs/index.html", loc)
		}
		req2 := httptest.NewRequest(http.MethodGet, loc, nil)
		rec2 := httptest.NewRecorder()
		HandleOpenAPI.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Fatalf("status (followed) = %d, want %d", rec2.Code, http.StatusOK)
		}
		ct2 := rec2.Header().Get("Content-Type")
		if ct2 == "" || !strings.Contains(strings.ToLower(ct2), "text/html") {
			t.Fatalf("content-type (followed) = %q, want to contain text/html", ct2)
		}
		body2 := rec2.Body.String()
		if !(strings.Contains(strings.ToLower(body2), "<!doctype") || strings.Contains(strings.ToLower(body2), "<html")) {
			t.Fatalf("response body (followed) does not look like HTML: first 200 chars: %q", body2[:min(len(body2), 200)])
		}
	default:
		t.Fatalf("unexpected status for /r/api/docs/: %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// Test that the Swagger UI index.html is served at /rootPath/api/docs/index.html
func TestServeOpenApiDocs_IndexHTML(t *testing.T) {

	req := httptest.NewRequest(http.MethodGet, "http://example.com:8080/r/api/docs/index.html", nil)
	rec := httptest.NewRecorder()
	NewOpenAPIHandler("/r").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if ct == "" || !strings.Contains(strings.ToLower(ct), "text/html") {
		t.Fatalf("content-type = %q, want to contain text/html", ct)
	}
	body := rec.Body.String()
	if !(strings.Contains(strings.ToLower(body), "<!doctype") || strings.Contains(strings.ToLower(body), "<html")) {
		t.Fatalf("response body does not look like HTML: first 200 chars: %q", body[:min(len(body), 200)])
	}
}

func TestParseOpenApiPath(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantSpec string
		wantOK   bool
	}{
		{name: "stripped root slash", in: "/", wantSpec: "", wantOK: false},
		{name: "stripped spec path", in: "/openapi.yaml", wantSpec: "", wantOK: false},
		{name: "full docs root", in: "/api/docs/", wantSpec: "", wantOK: true},
		{name: "full spec path", in: "/api/docs/openapi.yaml", wantSpec: "openapi.yaml", wantOK: true},
		{name: "url encoded spec", in: "/api/docs/openapi%2Eyaml", wantSpec: "openapi.yaml", wantOK: true},
		{name: "unrelated path", in: "/healthz", wantSpec: "", wantOK: false},
		{name: "bad escape", in: "%", wantSpec: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpec, gotOK := parseOpenApiPath(tt.in)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotSpec != tt.wantSpec {
				t.Fatalf("spec = %q, want %q", gotSpec, tt.wantSpec)
			}
		})
	}
}
