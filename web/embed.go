package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed static/*
var embeddedFiles embed.FS

func Handler() http.Handler {
	staticFS, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServerFS(staticFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/caldav") || strings.HasPrefix(r.URL.Path, "/swagger") || r.URL.Path == "/openapi.yaml" || r.URL.Path == "/healthz" {
			http.NotFound(w, r)
			return
		}

		if r.URL.Path == "/" {
			r.URL.Path = "/index.html"
		}

		cleanPath := path.Clean(r.URL.Path)
		if cleanPath == "." || cleanPath == "/" {
			cleanPath = "/index.html"
		}

		if _, err := fs.Stat(staticFS, strings.TrimPrefix(cleanPath, "/")); err != nil {
			r.URL.Path = "/index.html"
		}

		fileServer.ServeHTTP(w, r)
	})
}
