package caldav

// timeslot_readpath_test.go verifies CDV-TS-READ-1/2/3:
//
//   - A free timeslot is synthesised with SUMMARY:"Free" and TRANSP:TRANSPARENT.
//   - A booked timeslot is synthesised with SUMMARY:"<contact> (<service>)"
//     and TRANSP:OPAQUE.
//   - A recurring series root is always synthesised as free+transparent,
//     regardless of booking state.
//
// Tests exercise both the combinedCalendarStore methods directly (unit level)
// and the HTTP GET endpoint (integration level) to confirm the iCal body
// returned to a CalDAV client contains the expected properties.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	calstore "schedio/internal/store"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// slotStart / slotEnd define a fixed availability window used in all tests.
var (
	slotStart = time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	slotEnd   = time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
)

// newReadPathFixture builds a MemoryStore pre-seeded with:
//   - one staff user (ID "staff-1")
//   - one service (ID "svc-1", name "Massage")
//   - one timeslot for staff-1 covering [slotStart, slotEnd)
//
// Returns the store and the avail calendar ID for staff-1.
func newReadPathFixture(t *testing.T) (*calstore.MemoryStore, string) {
	t.Helper()
	ctx := context.Background()
	st := calstore.NewMemoryStore()

	// Seed staff user.
	if err := st.SyncUsers(ctx, []*calstore.User{
		{ID: "staff-1", Email: "staff@test.local", Role: calstore.UserRoleStaff, Name: "Test Staff"},
	}); err != nil {
		t.Fatalf("SyncUsers: %v", err)
	}

	// Seed service.
	if err := st.CreateService(ctx, &calstore.Service{
		ID: "svc-1", Name: "Massage", DurationMinutes: 60,
	}); err != nil {
		t.Fatalf("CreateService: %v", err)
	}

	// Seed timeslot for staff-1.
	if err := st.UpsertTimeslot(ctx, &calstore.Timeslot{
		UserID:    "staff-1",
		CalDAVUID: "ts-free-1",
		StartAt:   slotStart,
		EndAt:     slotEnd,
	}); err != nil {
		t.Fatalf("UpsertTimeslot: %v", err)
	}

	calID := availCalendarID("staff-1")
	return st, calID
}

// seedBooking inserts a booking that covers the timeslot window in the fixture.
// Returns the booking ID.
func seedBooking(t *testing.T, st *calstore.MemoryStore) string {
	t.Helper()
	ctx := context.Background()

	contact, err := st.GetOrCreateContact(ctx, "customer@test.local", &calstore.Contact{
		FirstName: "Anna",
		LastName:  "Schmidt",
	})
	if err != nil {
		t.Fatalf("GetOrCreateContact: %v", err)
	}

	// CreateSession is required as a foreign-key parent for some store
	// implementations; use an in-session booking via UpdateBooking path.
	// For MemoryStore we can create the session first.
	session := &calstore.BookingSession{
		ID:          "sess-1",
		ServiceID:   "svc-1",
		ContactID:   contact.ID,
		State:       calstore.SessionStateSubmitted,
		SubmittedAt: time.Now().UTC(),
	}
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	booking := &calstore.Booking{
		ID:        "booking-1",
		SessionID: session.ID,
		UserID:    "staff-1",
		ServiceID: "svc-1",
		ContactID: contact.ID,
		StartAt:   slotStart,
		EndAt:     slotEnd,
		State:     calstore.BookingStateReserved,
	}
	if err := st.CreateBooking(ctx, booking); err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}
	return booking.ID
}

// ── unit tests on combinedCalendarStore ───────────────────────────────────────

// TestTimeslotReadPath_FreeEvent verifies CDV-TS-READ-2: a timeslot with no
// overlapping active booking is synthesised as free (Summary:"Free",
// Opacity:Transparent).
func TestTimeslotReadPath_FreeEvent(t *testing.T) {
	st, calID := newReadPathFixture(t)
	ctx := context.Background()
	ownerID := "staff-1"

	combined := &combinedCalendarStore{base: st, domain: st}

	ts := &calstore.Timeslot{
		UserID:    ownerID,
		CalDAVUID: "ts-free-1",
		StartAt:   slotStart,
		EndAt:     slotEnd,
	}

	ev, err := combined.timeslotToEventWithOccupancy(ctx, calID, ownerID, ts)
	if err != nil {
		t.Fatalf("timeslotToEventWithOccupancy: %v", err)
	}
	if ev.Summary != "Free" {
		t.Errorf("Summary = %q, want %q", ev.Summary, "Free")
	}
	if ev.Opacity != calstore.OpacityTransparent {
		t.Errorf("Opacity = %v, want Transparent", ev.Opacity)
	}
}

// TestTimeslotReadPath_BookedEvent verifies CDV-TS-READ-3: a timeslot with an
// overlapping active booking is synthesised with the customer+service summary
// and Opacity:Opaque.
func TestTimeslotReadPath_BookedEvent(t *testing.T) {
	st, calID := newReadPathFixture(t)
	seedBooking(t, st)

	ctx := context.Background()
	ownerID := "staff-1"
	combined := &combinedCalendarStore{base: st, domain: st}

	ts := &calstore.Timeslot{
		UserID:    ownerID,
		CalDAVUID: "ts-free-1",
		StartAt:   slotStart,
		EndAt:     slotEnd,
	}

	ev, err := combined.timeslotToEventWithOccupancy(ctx, calID, ownerID, ts)
	if err != nil {
		t.Fatalf("timeslotToEventWithOccupancy: %v", err)
	}
	wantSummary := "Anna Schmidt (Massage)"
	if ev.Summary != wantSummary {
		t.Errorf("Summary = %q, want %q", ev.Summary, wantSummary)
	}
	if ev.Opacity != calstore.OpacityOpaque {
		t.Errorf("Opacity = %v, want Opaque", ev.Opacity)
	}
}

// TestTimeslotReadPath_SeriesRootAlwaysFree verifies CDV-TS-READ-1: a
// recurring series root is always synthesised as free regardless of whether
// a booking overlaps one of its occurrences.
func TestTimeslotReadPath_SeriesRootAlwaysFree(t *testing.T) {
	st, calID := newReadPathFixture(t)
	seedBooking(t, st)

	ctx := context.Background()
	ownerID := "staff-1"
	combined := &combinedCalendarStore{base: st, domain: st}

	// Series root: has RRule, no RecurrenceID.
	ts := &calstore.Timeslot{
		UserID:    ownerID,
		CalDAVUID: "ts-free-1",
		StartAt:   slotStart,
		EndAt:     slotEnd,
		RRule:     "FREQ=WEEKLY;COUNT=4",
	}

	ev, err := combined.timeslotToEventWithOccupancy(ctx, calID, ownerID, ts)
	if err != nil {
		t.Fatalf("timeslotToEventWithOccupancy: %v", err)
	}
	if ev.Summary != "Free" {
		t.Errorf("series root Summary = %q, want %q (CDV-TS-READ-1)", ev.Summary, "Free")
	}
	if ev.Opacity != calstore.OpacityTransparent {
		t.Errorf("series root Opacity = %v, want Transparent (CDV-TS-READ-1)", ev.Opacity)
	}
}

// TestTimeslotBookedSummary verifies the helper that formats the display title.
func TestTimeslotBookedSummary(t *testing.T) {
	cases := []struct {
		contact, service, want string
	}{
		{"Anna Schmidt", "Massage", "Anna Schmidt (Massage)"},
		{"Anna Schmidt", "", "Anna Schmidt"},
		{"", "Massage", "(Massage)"},
		{"", "", "Booked"},
	}
	for _, tc := range cases {
		got := timeslotBookedSummary(tc.contact, tc.service)
		if got != tc.want {
			t.Errorf("timeslotBookedSummary(%q, %q) = %q, want %q", tc.contact, tc.service, got, tc.want)
		}
	}
}

// ── HTTP-level integration tests ──────────────────────────────────────────────

// TestTimeslotReadPath_HTTP_FreeICS verifies that a GET on a free timeslot
// .ics URL returns an iCal body with SUMMARY:Free and TRANSP:TRANSPARENT.
func TestTimeslotReadPath_HTTP_FreeICS(t *testing.T) {
	st, _ := newReadPathFixture(t)
	handler := newTestCaldavHandler(st, "")

	url := "http://example.com/caldav/user/calendars/avail-staff-1/ts-free-1.ics"
	req := httptest.NewRequest("GET", url, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET free .ics: status = %d, want 200\nbody: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SUMMARY:Free") {
		t.Errorf("free .ics missing SUMMARY:Free\n%s", body)
	}
	if !strings.Contains(body, "TRANSP:TRANSPARENT") {
		t.Errorf("free .ics missing TRANSP:TRANSPARENT\n%s", body)
	}
	if strings.Contains(body, "TRANSP:OPAQUE") {
		t.Errorf("free .ics should not contain TRANSP:OPAQUE\n%s", body)
	}
}

// TestTimeslotReadPath_HTTP_BookedICS verifies that a GET on a booked timeslot
// .ics URL returns an iCal body with SUMMARY:"Anna Schmidt (Massage)" and
// TRANSP:OPAQUE.
func TestTimeslotReadPath_HTTP_BookedICS(t *testing.T) {
	st, _ := newReadPathFixture(t)
	seedBooking(t, st)
	handler := newTestCaldavHandler(st, "")

	url := "http://example.com/caldav/user/calendars/avail-staff-1/ts-free-1.ics"
	req := httptest.NewRequest("GET", url, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET booked .ics: status = %d, want 200\nbody: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SUMMARY:Anna Schmidt (Massage)") {
		t.Errorf("booked .ics missing expected SUMMARY\n%s", body)
	}
	if !strings.Contains(body, "TRANSP:OPAQUE") {
		t.Errorf("booked .ics missing TRANSP:OPAQUE\n%s", body)
	}
	if strings.Contains(body, "SUMMARY:Free") {
		t.Errorf("booked .ics should not contain SUMMARY:Free\n%s", body)
	}
	if strings.Contains(body, "TRANSP:TRANSPARENT") {
		t.Errorf("booked .ics should not contain TRANSP:TRANSPARENT\n%s", body)
	}
}

// TestTimeslotReadPath_FreeThenBooked verifies the state transition: after a
// booking is created for a timeslot, the iCal representation flips from free
// to booked on the very next GET (no cache invalidation needed because the
// store is shared in memory).
func TestTimeslotReadPath_FreeThenBooked(t *testing.T) {
	st, _ := newReadPathFixture(t)
	handler := newTestCaldavHandler(st, "")
	url := "http://example.com/caldav/user/calendars/avail-staff-1/ts-free-1.ics"

	// Step 1: free.
	req1 := httptest.NewRequest("GET", url, nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("step 1 GET: status = %d\nbody: %s", rec1.Code, rec1.Body)
	}
	if !strings.Contains(rec1.Body.String(), "SUMMARY:Free") {
		t.Errorf("step 1 (free): expected SUMMARY:Free\n%s", rec1.Body)
	}

	// Step 2: create a booking.
	seedBooking(t, st)

	// Step 3: same GET — must now show booked.
	req2 := httptest.NewRequest("GET", url, nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("step 3 GET: status = %d\nbody: %s", rec2.Code, rec2.Body)
	}
	body := rec2.Body.String()
	if !strings.Contains(body, "SUMMARY:Anna Schmidt (Massage)") {
		t.Errorf("step 3 (booked): expected SUMMARY:Anna Schmidt (Massage)\n%s", body)
	}
	if !strings.Contains(body, "TRANSP:OPAQUE") {
		t.Errorf("step 3 (booked): expected TRANSP:OPAQUE\n%s", body)
	}
}
