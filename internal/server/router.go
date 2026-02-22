package server

import (
	"net/http"

	"schedio/internal/handlers"
	"schedio/web"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /openapi.yaml", handlers.OpenAPISpec)
	mux.Handle("/swagger", handlers.SwaggerUI())
	mux.Handle("/swagger/", handlers.SwaggerUI())

	mux.Handle("/caldav", handlers.CALDAV())
	mux.Handle("/caldav/", handlers.CALDAV())

	mux.Handle("/", web.Handler())

	return mux
}
