package caldav

import (
	"context"
	"net/http"

	"k8s.io/klog/v2"
	"schedio/internal/auth"
	calstore "schedio/internal/store"
)

// principalKey is the unexported context key used to attach the authenticated
// CalDAV user to a request context.
type principalKey struct{}

// principalFromContext returns the authenticated user attached to the request
// context by basicAuthMiddleware. Returns nil when no principal is present.
func principalFromContext(ctx context.Context) *calstore.User {
	u, _ := ctx.Value(principalKey{}).(*calstore.User)
	return u
}

// contextWithPrincipal returns a copy of ctx carrying u as the CalDAV principal.
func contextWithPrincipal(ctx context.Context, u *calstore.User) context.Context {
	return context.WithValue(ctx, principalKey{}, u)
}

// BasicAuthMiddleware wraps next and enforces HTTP Basic Auth using the
// provided Authenticator. Unauthenticated or invalid requests receive a 401
// response with a WWW-Authenticate: Basic header. The authenticated user is
// stored in the request context and can be retrieved with principalFromContext.
func BasicAuthMiddleware(authenticator *auth.Authenticator, next http.Handler) http.Handler {
	return basicAuthMiddleware(authenticator, next)
}

// noAuthMiddleware is a development-only middleware that bypasses all
// authentication.  It injects the first staff user from the store into the
// request context so that CalDAV handlers have a valid principal without
// requiring an Authorization header.  Use only when -noauth is set.
func noAuthMiddleware(domainStore calstore.DomainStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		users, err := domainStore.ListStaff(r.Context())
		if err != nil || len(users) == 0 {
			http.Error(w, "noauth: no staff users configured", http.StatusInternalServerError)
			return
		}
		klog.V(2).Infof("caldav/noauth: injecting principal %q (no credentials required)", users[0].Email)
		r = r.WithContext(contextWithPrincipal(r.Context(), users[0]))
		next.ServeHTTP(w, r)
	})
}

// basicAuthMiddleware wraps next and enforces HTTP Basic Auth using the
// provided Authenticator. Unauthenticated or invalid requests receive a 401
// response with a WWW-Authenticate: Basic header.
func basicAuthMiddleware(authenticator *auth.Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="schedio CalDAV"`)
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		user, err := authenticator.Authenticate(r.Context(), email, password)
		if err != nil {
			klog.V(1).Infof("caldav/auth: authentication failed for %q: %v", email, err)
			w.Header().Set("WWW-Authenticate", `Basic realm="schedio CalDAV"`)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		klog.V(1).Infof("caldav/auth: authenticated %q role=%v id=%q", user.Email, user.Role, user.ID)
		r = r.WithContext(contextWithPrincipal(r.Context(), user))
		next.ServeHTTP(w, r)
	})
}
