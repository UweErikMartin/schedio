package caldav

// testhelper_test.go provides helpers for building CalDAV handlers in unit
// tests without the HTTP Basic Auth round-trip. A pre-built administrator
// principal is injected into every request context so the combined store's
// access-control checks pass as if a real admin user had authenticated.

import (
	"net/http"

	calstore "schedio/internal/store"
)

// testAdminUser is a synthetic administrator principal used in CalDAV unit tests.
var testAdminUser = &calstore.User{
	ID:    "test-admin",
	Email: "admin@test.local",
	Role:  calstore.UserRoleAdministrator,
	Name:  "Test Admin",
}

// newTestCaldavHandler builds a CalDAV handler suitable for unit tests.
//
// Unlike NewHandler, it does not wrap the handler with HTTP Basic Auth.
// Instead, testAdminUser is injected as the CalDAV principal into every
// request context before the request reaches the CalDAV logic, so all
// access-control checks pass.
//
// If calStore also implements calstore.DomainStore (e.g. *calstore.MemoryStore),
// it is reused as the domain back-end; otherwise a fresh MemoryStore is created.
func newTestCaldavHandler(calStore calstore.CalendarStore, rootPath string) http.Handler {
	var ds calstore.DomainStore
	if x, ok := calStore.(calstore.DomainStore); ok {
		ds = x
	} else {
		ds = calstore.NewMemoryStore()
	}
	combined := &combinedCalendarStore{base: calStore, domain: ds}
	inner := buildCaldavHandler(combined, rootPath)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(contextWithPrincipal(r.Context(), testAdminUser))
		inner.ServeHTTP(w, r)
	})
}
