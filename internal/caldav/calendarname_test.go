package caldav

// calendarname_test.go verifies the end-to-end round-trip between the global
// settings DefaultCalendarName field and the display name returned over CalDAV.
//
// An iPad performs the following sequence during initial account setup:
//
//  1. PROPFIND /.well-known/caldav  → redirect to /caldav/
//  2. PROPFIND /                    → current-user-principal
//  3. PROPFIND /caldav/user/        → principal props (calendar-home-set, …)
//  4. PROPFIND /caldav/user/calendars/ (Depth:1) → calendar list incl. displayname
//  5. PROPFIND /caldav/user/calendars/<id>/ (Depth:0) → individual calendar props
//
// Only steps 4 and 5 contain the calendar display name.  Both are covered here.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	calstore "schedio/internal/store"
)

// newCalendarNameMux builds a full test router (discovery + CalDAV) backed by
// the given MemoryStore so that settings changes are reflected immediately.
func newCalendarNameMux(st *calstore.MemoryStore) http.Handler {
	discovery := NewDiscoveryHandler("")
	caldavHandler := newTestCaldavHandler(st, "")

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/caldav", discovery.WellKnownHandler)
	mux.HandleFunc("/principals/", discovery.PrincipalsHandler)
	mux.Handle("/caldav/", caldavHandler)
	mux.Handle("/", discovery.RootPropfindHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	return mux
}

// TestDefaultCalendarName_PropfindHomeSetDepth1 verifies that a fresh store
// returns "Timeslot-Calendar" in the Depth:1 PROPFIND on the calendar home set
// — step 4 of the iPad initial account-setup sequence.
func TestDefaultCalendarName_PropfindHomeSetDepth1(t *testing.T) {
	st := calstore.NewMemoryStore()
	handler := newTestCaldavHandler(st, "")

	req := httptest.NewRequest("PROPFIND", "http://example.com/caldav/user/calendars/", nil)
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d\nbody: %s", rec.Code, http.StatusMultiStatus, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Timeslot-Calendar") {
		t.Fatalf("Depth:1 home PROPFIND missing default name 'Timeslot-Calendar':\n%s", body)
	}
}

// TestCustomCalendarName_PropfindHomeSetDepth1 verifies the core behavioural
// requirement: updating DefaultCalendarName via UpdateSettings causes the new
// name to appear in the very next Depth:1 PROPFIND on the calendar home set
// without a server restart.  This is step 4 of the iPad discovery flow.
func TestCustomCalendarName_PropfindHomeSetDepth1(t *testing.T) {
	st := calstore.NewMemoryStore()

	// Simulate the admin saving a custom name via the admin settings page.
	settings, err := st.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	settings.DefaultCalendarName = "Buchungen"
	if err := st.UpdateSettings(context.Background(), settings); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	handler := newTestCaldavHandler(st, "")

	req := httptest.NewRequest("PROPFIND", "http://example.com/caldav/user/calendars/", nil)
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d\nbody: %s", rec.Code, http.StatusMultiStatus, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Buchungen") {
		t.Fatalf("Depth:1 home PROPFIND missing custom name 'Buchungen':\n%s", body)
	}
	if strings.Contains(body, ">Timeslot-Calendar<") {
		t.Fatalf("Depth:1 home PROPFIND still contains default name 'Timeslot-Calendar' after settings update:\n%s", body)
	}
}

// TestCustomCalendarName_PropfindCollectionDepth0 verifies that the custom name
// also appears in the Depth:0 PROPFIND for the individual calendar collection
// — step 5 of the iPad discovery flow, sent after the Depth:1 enumeration.
func TestCustomCalendarName_PropfindCollectionDepth0(t *testing.T) {
	st := calstore.NewMemoryStore()

	settings, err := st.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	settings.DefaultCalendarName = "Termine"
	if err := st.UpdateSettings(context.Background(), settings); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	handler := newTestCaldavHandler(st, "")

	req := httptest.NewRequest("PROPFIND", "http://example.com/caldav/user/calendars/default/", nil)
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d\nbody: %s", rec.Code, http.StatusMultiStatus, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Termine") {
		t.Fatalf("Depth:0 collection PROPFIND missing custom name 'Termine':\n%s", body)
	}
	if strings.Contains(body, ">Timeslot-Calendar<") {
		t.Fatalf("Depth:0 collection PROPFIND still shows default name 'Timeslot-Calendar':\n%s", body)
	}
}

// TestCalendarNameReset_EmptyStringFallsBackToDefault verifies the fallback
// rule: when DefaultCalendarName is set back to "" the name reverts to
// "Timeslot-Calendar" rather than becoming blank.
func TestCalendarNameReset_EmptyStringFallsBackToDefault(t *testing.T) {
	st := calstore.NewMemoryStore()

	// First set a custom name …
	settings, _ := st.GetSettings(context.Background())
	settings.DefaultCalendarName = "Buchungen"
	_ = st.UpdateSettings(context.Background(), settings)

	// … then clear it, simulating the admin blanking the field and saving.
	settings.DefaultCalendarName = ""
	if err := st.UpdateSettings(context.Background(), settings); err != nil {
		t.Fatalf("UpdateSettings(empty): %v", err)
	}

	handler := newTestCaldavHandler(st, "")

	req := httptest.NewRequest("PROPFIND", "http://example.com/caldav/user/calendars/", nil)
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d\nbody: %s", rec.Code, http.StatusMultiStatus, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Timeslot-Calendar") {
		t.Fatalf("Depth:1 home PROPFIND after reset should contain 'Timeslot-Calendar':\n%s", body)
	}
	if strings.Contains(body, "Buchungen") {
		t.Fatalf("Depth:1 home PROPFIND after reset still shows old custom name:\n%s", body)
	}
}

// TestCalendarNameEndToEnd_iPadDiscoveryFlow walks the full iPad initial
// account-setup sequence (steps 1–5) and checks that the configured calendar
// name appears where expected in both enumeration responses.
func TestCalendarNameEndToEnd_iPadDiscoveryFlow(t *testing.T) {
	st := calstore.NewMemoryStore()

	settings, _ := st.GetSettings(context.Background())
	settings.DefaultCalendarName = "Mein Kalender"
	if err := st.UpdateSettings(context.Background(), settings); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	router := newCalendarNameMux(st)

	// Step 1: /.well-known/caldav → redirect (no name here, just check it works)
	req1 := httptest.NewRequest("PROPFIND", "http://example.com/.well-known/caldav", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusMovedPermanently {
		t.Fatalf("step 1: status = %d, want %d", rec1.Code, http.StatusMovedPermanently)
	}

	// Step 2: PROPFIND / → current-user-principal
	req2 := httptest.NewRequest("PROPFIND", "http://example.com/", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusMultiStatus {
		t.Fatalf("step 2: status = %d, want %d", rec2.Code, http.StatusMultiStatus)
	}
	if !strings.Contains(rec2.Body.String(), "current-user-principal") {
		t.Fatalf("step 2: missing current-user-principal:\n%s", rec2.Body)
	}

	// Step 3: PROPFIND /caldav/user/ → principal (calendar-home-set, etc.)
	req3 := httptest.NewRequest("PROPFIND", "http://example.com/caldav/user/", nil)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusMultiStatus {
		t.Fatalf("step 3: status = %d, want %d", rec3.Code, http.StatusMultiStatus)
	}
	if !strings.Contains(rec3.Body.String(), "calendar-home-set") {
		t.Fatalf("step 3: missing calendar-home-set:\n%s", rec3.Body)
	}

	// Step 4: PROPFIND /caldav/user/calendars/ Depth:1 → calendar list with name
	req4 := httptest.NewRequest("PROPFIND", "http://example.com/caldav/user/calendars/", nil)
	req4.Header.Set("Depth", "1")
	rec4 := httptest.NewRecorder()
	router.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusMultiStatus {
		t.Fatalf("step 4: status = %d, want %d\nbody: %s", rec4.Code, http.StatusMultiStatus, rec4.Body)
	}
	body4 := rec4.Body.String()
	if !strings.Contains(body4, "Mein Kalender") {
		t.Fatalf("step 4 (Depth:1 home): missing custom name 'Mein Kalender':\n%s", body4)
	}

	// Step 5: PROPFIND /caldav/user/calendars/default/ Depth:0 → individual calendar
	req5 := httptest.NewRequest("PROPFIND", "http://example.com/caldav/user/calendars/default/", nil)
	req5.Header.Set("Depth", "0")
	rec5 := httptest.NewRecorder()
	router.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusMultiStatus {
		t.Fatalf("step 5: status = %d, want %d\nbody: %s", rec5.Code, http.StatusMultiStatus, rec5.Body)
	}
	body5 := rec5.Body.String()
	if !strings.Contains(body5, "Mein Kalender") {
		t.Fatalf("step 5 (Depth:0 collection): missing custom name 'Mein Kalender':\n%s", body5)
	}
}
