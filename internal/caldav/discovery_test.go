package caldav

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	calstore "schedio/internal/store"
)

func newDiscoveryTestMux(rootPath string) http.Handler {
	discovery := NewDiscoveryHandler(rootPath)
	caldavHandler := NewHandler(calstore.NewDummyStore(), rootPath)

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/caldav", discovery.WellKnownHandler)
	mux.HandleFunc("/calendar/dav/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, rootPath+"/caldav/", http.StatusMovedPermanently)
	})
	mux.HandleFunc(rootPath+"/principals/", discovery.PrincipalsHandler)
	mux.Handle(rootPath+"/caldav/", caldavHandler)
	mux.Handle(rootPath+"/", discovery.RootPropfindHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	})))

	return mux
}

func TestIPadDiscovery_WellKnownCaldavRedirects(t *testing.T) {
	router := newDiscoveryTestMux("")

	req := httptest.NewRequest("PROPFIND", "http://example.com/.well-known/caldav", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMovedPermanently)
	}
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, "/caldav/") {
		t.Fatalf("Location = %q, want suffix /caldav/", loc)
	}
}

func TestIPadDiscovery_RootPropfindReturnsCurrentUserPrincipal(t *testing.T) {
	router := newDiscoveryTestMux("")

	req := httptest.NewRequest("PROPFIND", "http://example.com/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMultiStatus)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "current-user-principal") {
		t.Fatalf("response missing current-user-principal:\n%s", body)
	}
	if !strings.Contains(body, "/caldav/user/") {
		t.Fatalf("response missing /caldav/user/ href:\n%s", body)
	}
}

func TestIPadDiscovery_PrincipalsReturnsCalendarHomeSet(t *testing.T) {
	router := newDiscoveryTestMux("")

	req := httptest.NewRequest("PROPFIND", "http://example.com/principals/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMultiStatus)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "calendar-home-set") {
		t.Fatalf("response missing calendar-home-set:\n%s", body)
	}
	if !strings.Contains(body, "/caldav/user/calendars/") {
		t.Fatalf("response missing /caldav/user/calendars/ href:\n%s", body)
	}
	if !strings.Contains(body, "calendar-user-address-set") {
		t.Fatalf("response missing calendar-user-address-set:\n%s", body)
	}
	if !strings.Contains(body, "schedule-inbox-URL") {
		t.Fatalf("response missing schedule-inbox-URL:\n%s", body)
	}
	if !strings.Contains(body, "schedule-outbox-URL") {
		t.Fatalf("response missing schedule-outbox-URL:\n%s", body)
	}
}

func TestIPadDiscovery_AppleFallbackPathRedirects(t *testing.T) {
	router := newDiscoveryTestMux("")

	req := httptest.NewRequest("PROPFIND", "http://example.com/calendar/dav/alice/user/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMovedPermanently)
	}
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, "/caldav/") {
		t.Fatalf("Location = %q, want suffix /caldav/", loc)
	}
}

func TestIPadDiscovery_CalendarHomeSetListsDummyCalendar(t *testing.T) {
	router := newDiscoveryTestMux("")

	req := httptest.NewRequest("PROPFIND", "http://example.com/caldav/user/calendars/", nil)
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMultiStatus)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/caldav/user/calendars/dummy/") {
		t.Fatalf("response missing dummy calendar href:\n%s", body)
	}
	if !strings.Contains(body, "Dummy Calendar") {
		t.Fatalf("response missing dummy calendar display name:\n%s", body)
	}
	if !strings.Contains(body, "schedule-calendar-transp") {
		t.Fatalf("response missing schedule-calendar-transp (free/busy capability):\n%s", body)
	}
	if !strings.Contains(body, "schedule-inbox") {
		t.Fatalf("response missing schedule-inbox resource:\n%s", body)
	}
	if !strings.Contains(body, "schedule-outbox") {
		t.Fatalf("response missing schedule-outbox resource:\n%s", body)
	}
}

func TestIPadDiscovery_ReportReturnsEventData(t *testing.T) {
	router := newDiscoveryTestMux("")

	reportBody := `<?xml version="1.0" encoding="UTF-8"?>
<c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop>
    <d:getetag/>
    <c:calendar-data/>
  </d:prop>
  <c:filter>
    <c:comp-filter name="VCALENDAR">
      <c:comp-filter name="VEVENT"/>
    </c:comp-filter>
  </c:filter>
</c:calendar-query>`

	req := httptest.NewRequest("REPORT", "http://example.com/caldav/user/calendars/dummy/", strings.NewReader(reportBody))
	req.Header.Set("Content-Type", "application/xml")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMultiStatus)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "BEGIN:VEVENT") {
		t.Fatalf("response missing VEVENT payload:\n%s", body)
	}
	if !strings.Contains(body, "TRANSP:") {
		t.Fatalf("response missing TRANSP (free/busy opacity) field:\n%s", body)
	}
}

func TestIPadDiscovery_DeleteOnDummyStoreDoesNotReturnForbidden(t *testing.T) {
	router := newDiscoveryTestMux("")

	req := httptest.NewRequest(http.MethodDelete, "http://example.com/caldav/user/calendars/dummy/dummy-202602-09.ics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusForbidden {
		t.Fatalf("status = %d, want any non-403 status", rec.Code)
	}
}

func TestIPadDiscovery_ProppatchOnCalendarReturnsMultiStatus(t *testing.T) {
	router := newDiscoveryTestMux("")

	body := `<?xml version="1.0" encoding="UTF-8"?>
<d:propertyupdate xmlns:d="DAV:" xmlns:cs="http://calendarserver.org/ns/">
  <d:set>
    <d:prop>
      <cs:calendar-color>#4A90D9</cs:calendar-color>
    </d:prop>
  </d:set>
</d:propertyupdate>`

	req := httptest.NewRequest("PROPPATCH", "http://example.com/caldav/user/calendars/dummy/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/xml")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMultiStatus)
	}
	if !strings.Contains(rec.Body.String(), "multistatus") {
		t.Fatalf("response missing multistatus body: %s", rec.Body.String())
	}
}

func TestIPadDiscovery_PrincipalExposesSchedulingProperties(t *testing.T) {
	router := newDiscoveryTestMux("")

	req := httptest.NewRequest("PROPFIND", "http://example.com/caldav/user/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMultiStatus)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "schedule-inbox-URL") {
		t.Fatalf("response missing schedule-inbox-URL: %s", body)
	}
	if !strings.Contains(body, "schedule-outbox-URL") {
		t.Fatalf("response missing schedule-outbox-URL: %s", body)
	}
	if !strings.Contains(body, "calendar-user-address-set") {
		t.Fatalf("response missing calendar-user-address-set: %s", body)
	}
}

func TestIPadDiscovery_CalendarCollectionPropfindExposesFreeBusySetting(t *testing.T) {
	router := newDiscoveryTestMux("")

	req := httptest.NewRequest("PROPFIND", "http://example.com/caldav/user/calendars/dummy/", nil)
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMultiStatus)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "schedule-calendar-transp") {
		t.Fatalf("response missing schedule-calendar-transp: %s", body)
	}
	if !strings.Contains(body, "<cal:opaque/>") {
		t.Fatalf("response missing opaque schedule-calendar-transp value: %s", body)
	}
	if !strings.Contains(body, "read-free-busy") {
		t.Fatalf("response missing read-free-busy privilege: %s", body)
	}
	if !strings.Contains(body, "free-busy-query") {
		t.Fatalf("response missing free-busy-query in supported-report-set: %s", body)
	}
}

func TestIPadDiscovery_CalendarHomeDepthZeroDoesNotListChildren(t *testing.T) {
	router := newDiscoveryTestMux("")

	req := httptest.NewRequest("PROPFIND", "http://example.com/caldav/user/calendars/", nil)
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMultiStatus)
	}
	body := rec.Body.String()
	// The home entry at depth=0 may legitimately contain the dummy calendar
	// href inside properties like schedule-default-calendar-URL.
	// What must NOT appear is a separate <d:response> element for the child.
	if strings.Contains(body, "<d:response>") {
		// Count response elements: only the home itself should appear.
		count := strings.Count(body, "<d:response>")
		if count > 1 {
			t.Fatalf("depth 0 response contains %d <d:response> entries, want 1: %s", count, body)
		}
	}
}
