package server

import (
	"net/http"

	"schedio/internal/caldav"
	"schedio/internal/config"
	"schedio/internal/handlers"
	calstore "schedio/internal/store"
)

// NewRouter builds the HTTP mux.
// store is the CalendarStore implementation to back the CalDAV endpoint.
func NewRouter(args *config.Config, store calstore.CalendarStore) http.Handler {
	mux := http.NewServeMux()

	discovery := caldav.NewDiscoveryHandler(args.RootPath)
	caldavHandler := caldav.NewHandler(store, args.RootPath)

	// RFC 6764 §5 – well-known redirect: clients probe this first.
	mux.HandleFunc("/.well-known/caldav", discovery.WellKnownHandler)

	// Apple iOS/macOS fallback path – redirect to the real CalDAV root.
	mux.HandleFunc("/calendar/dav/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, args.RootPath+"/caldav/", http.StatusMovedPermanently)
	})

	// Principal URL – returns calendar-home-set so clients find the calendars.
	mux.HandleFunc(args.RootPath+"/principals/", discovery.PrincipalsHandler)

	mux.HandleFunc(args.RootPath+"/healthz", handlers.HandleHealthz)
	mux.Handle(args.RootPath+"/api/docs/", handlers.NewOpenAPIHandler(args.RootPath))
	mux.Handle(args.RootPath+"/caldav/", caldavHandler)

	// Root handler: intercept PROPFIND for discovery step 2, pass everything
	// else through to the web UI.
	webHandler := http.Handler(http.HandlerFunc(handlers.HandleWebUserInterface))
	if args.RootPath != "" {
		webHandler = http.StripPrefix(args.RootPath, webHandler)
	}
	mux.Handle(args.RootPath+"/", discovery.RootPropfindHandler(webHandler))

	return mux
}
