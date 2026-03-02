package server

import (
	"context"
	"net/http"

	"k8s.io/klog/v2"

	"schedio/internal/auth"
	"schedio/internal/caldav"
	"schedio/internal/config"
	"schedio/internal/email"
	"schedio/internal/handlers"
	"schedio/internal/handlers/admin"
	"schedio/internal/handlers/authhandler"
	"schedio/internal/handlers/customer"
	calstore "schedio/internal/store"
)

// NewRouter builds the HTTP mux.
// calStore backs the CalDAV endpoint; domainStore backs auth and domain APIs.
func NewRouter(args *config.Config, calStore calstore.CalendarStore, domainStore calstore.DomainStore) http.Handler {
	mux := http.NewServeMux()

	// Auth routes
	signingKey, _ := auth.GenerateSigningKey()
	sessions := auth.NewSessionManager(signingKey, false)
	authenticator := auth.NewAuthenticator(domainStore)
	authH := authhandler.NewHandler(sessions, authenticator, domainStore)
	mux.HandleFunc("POST "+args.RootPath+"/auth/login", authH.Login)
	mux.HandleFunc("POST "+args.RootPath+"/auth/logout", authH.Logout)

	discovery := caldav.NewDiscoveryHandler(args.RootPath)
	caldavHandler := caldav.NewHandler(calStore, domainStore, authenticator, args.RootPath, args.NoAuth)

	// RFC 6764 §5 – well-known redirect: clients probe this first.
	mux.HandleFunc("/.well-known/caldav", discovery.WellKnownHandler)

	// Apple iOS/macOS fallback path – redirect to the real CalDAV root.
	mux.HandleFunc("/calendar/dav/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, args.RootPath+"/caldav/", http.StatusMovedPermanently)
	})

	// Principal URL – returns calendar-home-set so clients find the calendars.
	// When -noauth is active (e.g. for plain-HTTP iOS testing) the auth wrapper
	// is skipped so iOS can discover the principal without credentials.
	principalHandler := http.Handler(http.HandlerFunc(discovery.PrincipalsHandler))
	if !args.NoAuth {
		principalHandler = caldav.BasicAuthMiddleware(authenticator, principalHandler)
	}
	mux.Handle(args.RootPath+"/principals/", principalHandler)

	mux.HandleFunc(args.RootPath+"/healthz", handlers.HandleHealthz)
	mux.Handle(args.RootPath+"/api/v1/services", handlers.NewServicesHandler(args))
	mux.Handle(args.RootPath+"/api/v1/availability", customer.NewAvailabilityHandler(domainStore))

	// ── Session lifecycle ────────────────────────────────────────────────────
	// Build an optional email sender; skip silently when SMTP is not configured.
	var emailSender *email.Sender
	if args.SmtpHost != "" {
		s, err := email.NewSender(email.Config{
			Host:        args.SmtpHost,
			Port:        args.SmtpPort,
			Username:    args.SmtpUsername,
			Password:    args.SmtpPassword,
			FromAddress: args.SmtpUsername,
			FromName:    args.SenderName,
		})
		if err != nil {
			klog.Errorf("router: email sender setup failed: %v (emails disabled)", err)
		} else {
			emailSender = s
		}
	}

	// ── Settings bootstrap ───────────────────────────────────────────────────
	// Ensure Settings.SenderName is populated. On first startup the value is
	// the empty string (memory store default), so we seed it from the CLI flag /
	// config-file value. If the admin has previously saved a different name via
	// the settings UI, the stored value takes precedence and we synchronise it
	// into the email sender so the runtime override matches the database.
	if st, err := domainStore.GetSettings(context.Background()); err == nil {
		if st.SenderName == "" && args.SenderName != "" {
			st.SenderName = args.SenderName
			if err := domainStore.UpdateSettings(context.Background(), st); err != nil {
				klog.Errorf("router: seed SenderName in settings: %v", err)
			}
		}
		if emailSender != nil && st.SenderName != "" {
			emailSender.SetFromName(st.SenderName)
		}
	}

	// ── Admin settings routes ────────────────────────────────────────────────
	settingsH := admin.NewSettingsHandler(domainStore, emailSender)
	mux.HandleFunc("GET "+args.RootPath+"/admin/api/v1/settings", settingsH.Get)
	mux.HandleFunc("PUT "+args.RootPath+"/admin/api/v1/settings", settingsH.Put)

	sessionH := customer.NewSessionHandler(domainStore, emailSender, args.AdminMail)
	mux.HandleFunc("POST "+args.RootPath+"/api/v1/sessions", sessionH.Create)
	mux.HandleFunc("POST "+args.RootPath+"/api/v1/sessions/{id}/bookings", sessionH.AddBooking)
	mux.HandleFunc("POST "+args.RootPath+"/api/v1/sessions/{id}/submit", sessionH.Submit)

	mux.Handle(args.RootPath+"/api/docs/", handlers.NewOpenAPIHandler(args.RootPath))
	mux.Handle(args.RootPath+"/caldav/", caldavHandler)

	// Root handler: intercept PROPFIND for discovery step 2, pass everything
	// else through to the web UI.
	// PROPFIND on "/" is wrapped in Basic Auth so iOS is forced to authenticate
	// here before learning the current-user-principal href. This ensures
	// credentials are stored in the iOS keychain during account setup.
	// Non-PROPFIND requests (GET for web UI) bypass auth entirely.
	webHandler := http.Handler(http.HandlerFunc(handlers.HandleWebUserInterface))
	if args.RootPath != "" {
		webHandler = http.StripPrefix(args.RootPath, webHandler)
	}
	rootDiscovery := discovery.RootPropfindHandler(webHandler)
	// When -noauth is set, PROPFIND / is served unauthenticated so that iOS
	// can discover the server over plain HTTP without sending credentials.
	var authedRootDiscovery http.Handler
	if args.NoAuth {
		authedRootDiscovery = rootDiscovery
	} else {
		authedRootDiscovery = caldav.BasicAuthMiddleware(authenticator, rootDiscovery)
	}
	mux.Handle(args.RootPath+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			authedRootDiscovery.ServeHTTP(w, r)
			return
		}
		rootDiscovery.ServeHTTP(w, r)
	}))

	return mux
}
