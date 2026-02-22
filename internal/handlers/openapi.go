package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"schedio/api"
	"strings"

	httpSwagger "github.com/swaggo/http-swagger"
	"k8s.io/klog/v2"
)

const specName = "openapi.yaml"
const docsPath = "/api/docs"

// NewOpenAPIHandler returns an http.Handler that serves the OpenAPI specification
// and the Swagger UI. rootPath is the configured URL prefix (e.g. "/ui").
func NewOpenAPIHandler(rootPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			spec, ok := parseOpenApiPath(r.URL.Path)
			if !ok {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if spec == specName {
				handleOpenApiSpec(w, r, rootPath)
				return
			}
			handleSwaggerUI(w, r, rootPath)
		default:
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func handleOpenApiSpec(w http.ResponseWriter, r *http.Request, rootPath string) {
	tmpl, err := template.New("openapi").Parse(string(api.OpenApiSpec))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read OpenAPI spec template: %v", err), http.StatusInternalServerError)
		return
	}

	server := buildServerRootURLFromRequest(r, rootPath)
	klog.V(3).Infof("OpenAPI: Using server URL from request: %s", server)

	data := struct {
		ServerRootURL string
	}{
		ServerRootURL: server,
	}

	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Failed to execute OpenAPI spec template", http.StatusInternalServerError)
	}
}

func handleSwaggerUI(w http.ResponseWriter, r *http.Request, rootPath string) {
	serverRootURL := buildServerRootURLFromRequest(r, rootPath)
	klog.V(3).Infof("OpenAPI: Using server root URL from request: %s", serverRootURL)

	specURL := fmt.Sprintf("%s/api/docs/%s", serverRootURL, specName)
	klog.V(3).Infof("OpenAPI: Serving Swagger UI, spec at %s", specURL)

	httpSwagger.Handler(
		httpSwagger.URL(specURL),
		httpSwagger.Layout("BaseLayout"),
	).ServeHTTP(w, r)
}

// parseOpenApiPath extracts the asset name from a URL-encoded docs path.
func parseOpenApiPath(p string) (spec string, ok bool) {
	decodedPath, err := url.PathUnescape(p)
	if err != nil {
		return "", false
	}

	decodedPath = strings.TrimSpace(decodedPath)
	if decodedPath == "" {
		return "", false
	}

	if !(decodedPath == docsPath || strings.HasSuffix(decodedPath, docsPath) || strings.Contains(decodedPath, docsPath+"/")) {
		return "", false
	}

	if decodedPath == docsPath || strings.HasSuffix(decodedPath, docsPath) || strings.HasSuffix(decodedPath, docsPath+"/") {
		return "", true
	}

	if _, suffix, found := strings.Cut(decodedPath, docsPath+"/"); found {
		return strings.TrimPrefix(suffix, "/"), true
	}

	return "", false
}

// buildServerRootURLFromRequest constructs the absolute server root URL from
// the incoming request headers and the configured rootPath.
func buildServerRootURLFromRequest(r *http.Request, rootPath string) string {
	// Scheme
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	// Host and optional port
	hostPort := r.Header.Get("X-Forwarded-Host")
	if hostPort == "" {
		hostPort = r.Host
	}
	if hostPort == "" && r.URL != nil {
		hostPort = r.URL.Host
	}

	// Optionally append forwarded port if not already present in hostPort
	if port := r.Header.Get("X-Forwarded-Port"); port != "" && !strings.Contains(hostPort, ":") {
		hostPort = hostPort + ":" + port
	}

	return fmt.Sprintf("%s://%s%s", scheme, hostPort, rootPath)
}
