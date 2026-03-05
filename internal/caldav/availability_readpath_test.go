package caldav

// availability_readpath_test.go verifies CDV-AVAIL-READ-1/2/3:
//
//   - A free availability record is synthesised with SUMMARY:"Free" and TRANSP:TRANSPARENT.
//   - A booked availability record is synthesised with SUMMARY:"<contact> (<service>)"
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

// availSlotStart / availSlotEnd define a fixed availability window used in all tests.
var (
	availSlotStart = time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	availSlotEnd   = time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
)

// newReadPathFixture builds a MemoryStore pre-seeded with:
//   - one staff user (ID "staff-1")
//   - one service (ID "svc-1", name "Massage")
//   - one availability record for staff-1 covering [slotStart, slotEnd)
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

	// Seed availability record for staff-1.
	if err := st.UpsertAvailability(ctx, &calstore.Availability{
		UserID:    "staff-1",
		CalDAVUID: "ts-free-1",
		StartAt:   availSlotStart,
		EndAt:     availSlotEnd,
	}); err != nil {
		t.Fatalf("UpsertAvailability: %v", err)
	}

	calID := availCalendarID("staff-1")
	return st, calID
}

// seedBooking inserts a booking that covers the availability window in the fixture.
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
		StartAt:   availSlotStart,
		EndAt:     availSlotEnd,
		State:     calstore.BookingStateReserved,
	}
	if err := st.CreateBooking(ctx, booking); err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}
	return booking.ID
}

// ── unit tests on combinedCalendarStore ───────────────────────────────────────

// TestAvailabilityReadPath_FreeEvent verifies CDV-AVAIL-READ-2: an availability record with no
// overlapping active booking is synthesised as free (Summary:"Free",
// Opacity:Transparent).
func TestAvailabilityReadPath_FreeEvent(t *testing.T) {
	st, calID := newReadPathFixture(t)
	ctx := context.Background()
	ownerID := "staff-1"

	combined := &combinedCalendarStore{base: st, domain: st}

	ts := &calstore.Availability{
		UserID:    ownerID,
		CalDAVUID: "ts-free-1",
		StartAt:   availSlotStart,
		EndAt:     availSlotEnd,
	}

	ev, err := combined.availabilityToEventWithOccupancy(ctx, calID, ownerID, ts)
	if err != nil {
		t.Fatalf("availabilityToEventWithOccupancy: %v", err)
	}
	if ev.Summary != "Free" {
		t.Errorf("Summary = %q, want %q", ev.Summary, "Free")
	}
	if ev.Opacity != calstore.OpacityTransparent {
		t.Errorf("Opacity = %v, want Transparent", ev.Opacity)
	}
}

// TestAvailabilityReadPath_BookedEvent verifies CDV-AVAIL-READ-3: an availability record with an
// overlapping active booking is synthesised with the customer+service summary
// and Opacity:Opaque.
func TestAvailabilityReadPath_BookedEvent(t *testing.T) {
	st, calID := newReadPathFixture(t)
	seedBooking(t, st)

	ctx := context.Background()
	ownerID := "staff-1"
	combined := &combinedCalendarStore{base: st, domain: st}

	ts := &calstore.Availability{
		UserID:    ownerID,
		CalDAVUID: "ts-free-1",
		StartAt:   availSlotStart,
		EndAt:     availSlotEnd,
	}

	ev, err := combined.availabilityToEventWithOccupancy(ctx, calID, ownerID, ts)
	if err != nil {
		t.Fatalf("availabilityToEventWithOccupancy: %v", err)
	}
	wantSummary := "Anna Schmidt (Massage)"
	if ev.Summary != wantSummary {
		t.Errorf("Summary = %q, want %q", ev.Summary, wantSummary)
	}
	if ev.Opacity != calstore.OpacityOpaque {
		t.Errorf("Opacity = %v, want Opaque", ev.Opacity)
	}
}

// TestAvailabilityReadPath_SeriesRootAlwaysFree verifies CDV-AVAIL-READ-1: a
// recurring series root is always synthesised as free regardless of whether
// a booking overlaps one of its occurrences.
func TestAvailabilityReadPath_SeriesRootAlwaysFree(t *testing.T) {
	st, calID := newReadPathFixture(t)
	seedBooking(t, st)

	ctx := context.Background()
	ownerID := "staff-1"
	combined := &combinedCalendarStore{base: st, domain: st}

	// Series root: has RRule, no RecurrenceID.
	ts := &calstore.Availability{
		UserID:    ownerID,
		CalDAVUID: "ts-free-1",
		StartAt:   availSlotStart,
		EndAt:     availSlotEnd,
		RRule:     "FREQ=WEEKLY;COUNT=4",
	}

	ev, err := combined.availabilityToEventWithOccupancy(ctx, calID, ownerID, ts)
	if err != nil {
		t.Fatalf("availabilityToEventWithOccupancy: %v", err)
	}
	if ev.Summary != "Free" {
		t.Errorf("series root Summary = %q, want %q (CDV-AVAIL-READ-1)", ev.Summary, "Free")
	}
	if ev.Opacity != calstore.OpacityTransparent {
		t.Errorf("series root Opacity = %v, want Transparent (CDV-AVAIL-READ-1)", ev.Opacity)
	}
}

// TestAvailabilityBookedSummary verifies the helper that formats the display title.
func TestAvailabilityBookedSummary(t *testing.T) {
	cases := []struct {
		contact, service, want string
	}{
		{"Anna Schmidt", "Massage", "Anna Schmidt (Massage)"},
		{"Anna Schmidt", "", "Anna Schmidt"},
		{"", "Massage", "(Massage)"},
		{"", "", "Booked"},
	}
	for _, tc := range cases {
		got := availabilityBookedSummary(tc.contact, tc.service)
		if got != tc.want {
			t.Errorf("availabilityBookedSummary(%q, %q) = %q, want %q", tc.contact, tc.service, got, tc.want)
		}
	}
}

// ── HTTP-level integration tests ──────────────────────────────────────────────

// TestAvailabilityReadPath_HTTP_FreeICS verifies that a GET on a free availability record
// .ics URL returns an iCal body with SUMMARY:Free and TRANSP:TRANSPARENT.
func TestAvailabilityReadPath_HTTP_FreeICS(t *testing.T) {
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

// TestAvailabilityReadPath_HTTP_BookedICS verifies that a GET on a booked availability record
// .ics URL returns an iCal body with SUMMARY:"Anna Schmidt (Massage)" and
// TRANSP:OPAQUE.
func TestAvailabilityReadPath_HTTP_BookedICS(t *testing.T) {
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

// TestAvailabilityReadPath_FreeThenBooked verifies the state transition: after a
// booking is created for an availability record, the iCal representation flips from free
// to booked on the very next GET (no cache invalidation needed because the
// store is shared in memory).
func TestAvailabilityReadPath_FreeThenBooked(t *testing.T) {
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

// ── Recurring series synthetic override tests ────────────────────────────────
//
// These tests verify CDV-AVAIL-READ-1: when a booking falls on an occurrence of
// a recurring availability series, ListEvents must return an additional
// synthetic VEVENT override with RECURRENCE-ID = booking.StartAt,
// SUMMARY = "<contact> (<service>)", and TRANSP:OPAQUE. The parent series root
// itself must remain free and transparent.

// newRecurringSeriesFixture builds a MemoryStore pre-seeded with:
//   - one staff user ("staff-1")
//   - one service ("svc-1", name "Massage")
//   - one recurring availability series for staff-1 with FREQ=WEEKLY;COUNT=4
//     starting at slotStart / slotEnd (the first occurrence == slotStart).
//
// The series root CalDAV UID is "series-uid-1".
func newRecurringSeriesFixture(t *testing.T) (*calstore.MemoryStore, string) {
	t.Helper()
	ctx := context.Background()
	st := calstore.NewMemoryStore()

	if err := st.SyncUsers(ctx, []*calstore.User{
		{ID: "staff-1", Email: "staff@test.local", Role: calstore.UserRoleStaff, Name: "Test Staff"},
	}); err != nil {
		t.Fatalf("SyncUsers: %v", err)
	}
	if err := st.CreateService(ctx, &calstore.Service{
		ID: "svc-1", Name: "Massage", DurationMinutes: 60,
	}); err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	if err := st.UpsertAvailability(ctx, &calstore.Availability{
		UserID:    "staff-1",
		CalDAVUID: "series-uid-1",
		StartAt:   availSlotStart,
		EndAt:     availSlotEnd,
		RRule:     "FREQ=WEEKLY;COUNT=4",
	}); err != nil {
		t.Fatalf("UpsertAvailability: %v", err)
	}

	calID := availCalendarID("staff-1")
	return st, calID
}

// seedBookingForSeries inserts a booking that covers the first occurrence of
// the recurring series in the fixture (== slotStart / slotEnd).
func seedBookingForSeries(t *testing.T, st *calstore.MemoryStore) {
	t.Helper()
	ctx := context.Background()
	contact, err := st.GetOrCreateContact(ctx, "customer@test.local", &calstore.Contact{
		FirstName: "Anna",
		LastName:  "Schmidt",
	})
	if err != nil {
		t.Fatalf("GetOrCreateContact: %v", err)
	}
	session := &calstore.BookingSession{
		ID:          "sess-2",
		ServiceID:   "svc-1",
		ContactID:   contact.ID,
		State:       calstore.SessionStateSubmitted,
		SubmittedAt: time.Now().UTC(),
	}
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := st.CreateBooking(ctx, &calstore.Booking{
		ID:        "booking-series-1",
		SessionID: session.ID,
		UserID:    "staff-1",
		ServiceID: "svc-1",
		ContactID: contact.ID,
		StartAt:   availSlotStart,
		EndAt:     availSlotEnd,
		State:     calstore.BookingStateReserved,
	}); err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}
}

// TestAvailabilityReadPath_RecurringSeriesRootAlwaysFreeInListEvents verifies
// that ListEvents always returns the series root as free (CDV-AVAIL-READ-1),
// even when a booking exists for one of its occurrences.
func TestAvailabilityReadPath_RecurringSeriesRootAlwaysFreeInListEvents(t *testing.T) {
	st, calID := newRecurringSeriesFixture(t)
	seedBookingForSeries(t, st)
	ctx := contextWithPrincipal(context.Background(), testAdminUser)
	combined := &combinedCalendarStore{base: st, domain: st}

	events, err := combined.ListEvents(ctx, calID, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}

	// The series root must appear exactly once as free.
	var root *calstore.Event
	for _, ev := range events {
		if ev.RRule != "" && ev.RecurrenceID.IsZero() && ev.ID == "series-uid-1" {
			root = ev
			break
		}
	}
	if root == nil {
		t.Fatalf("series root not found in ListEvents; got %d event(s)", len(events))
	}
	if root.Summary != "Free" {
		t.Errorf("series root Summary = %q, want \"Free\" (CDV-AVAIL-READ-1)", root.Summary)
	}
	if root.Opacity != calstore.OpacityTransparent {
		t.Errorf("series root Opacity = %v, want Transparent (CDV-AVAIL-READ-1)", root.Opacity)
	}
}

// TestAvailabilityReadPath_RecurringSeriesBookedOccurrenceGetsOverride verifies
// the main CDV-AVAIL-READ-1 gap: a booking for an occurrence of a recurring
// series must cause ListEvents to return the series root with a populated
// InlineVEVENTs slice containing:
//   - RECURRENCE-ID = booking.StartAt
//   - the same UID as the series root
//   - SUMMARY = "<contact> (<service>)"
//   - TRANSP:OPAQUE
//
// The override must NOT appear as a separate top-level calstore.Event; it must
// be embedded inside the series root's InlineVEVENTs so that the CalDAV
// adapter emits all VEVENTs inside a single VCALENDAR/.ics resource (RFC 4791).
func TestAvailabilityReadPath_RecurringSeriesBookedOccurrenceGetsOverride(t *testing.T) {
	st, calID := newRecurringSeriesFixture(t)
	seedBookingForSeries(t, st)
	ctx := contextWithPrincipal(context.Background(), testAdminUser)
	combined := &combinedCalendarStore{base: st, domain: st}

	events, err := combined.ListEvents(ctx, calID, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}

	// ListEvents must return exactly one top-level event (the series root).
	// The booked occurrence override must be embedded in InlineVEVENTs, NOT as
	// a separate top-level entry, to avoid duplicate hrefs in PROPFIND Depth:1.
	if len(events) != 1 {
		t.Fatalf("expected exactly 1 top-level event (series root), got %d", len(events))
	}

	root := events[0]
	if root.ID != "series-uid-1" {
		t.Errorf("top-level event ID = %q, want %q", root.ID, "series-uid-1")
	}
	if root.RRule == "" {
		t.Error("series root has no RRule")
	}

	// The series root itself must remain free/transparent (CDV-AVAIL-READ-1).
	if root.Summary != "Free" {
		t.Errorf("series root Summary = %q, want \"Free\"", root.Summary)
	}

	// The booked occurrence must be in InlineVEVENTs.
	if len(root.InlineVEVENTs) == 0 {
		t.Fatal("series root InlineVEVENTs is empty; expected one booked-occurrence override")
	}
	override := root.InlineVEVENTs[0]

	wantSummary := "Anna Schmidt (Massage)"
	if override.Summary != wantSummary {
		t.Errorf("override Summary = %q, want %q", override.Summary, wantSummary)
	}
	if override.Opacity != calstore.OpacityOpaque {
		t.Errorf("override Opacity = %v, want Opaque", override.Opacity)
	}
	if !override.RecurrenceID.Equal(availSlotStart) {
		t.Errorf("override RecurrenceID = %v, want %v", override.RecurrenceID, availSlotStart)
	}
	if !override.Start.Equal(availSlotStart) {
		t.Errorf("override Start = %v, want %v", override.Start, availSlotStart)
	}
	if !override.End.Equal(availSlotEnd) {
		t.Errorf("override End = %v, want %v", override.End, availSlotEnd)
	}
}

// TestAvailabilityReadPath_FreeRecurringSeriesNoExtraEvents verifies that a
// recurring series with no bookings produces exactly one event in ListEvents
// (the series root), with no spurious synthetic overrides.
func TestAvailabilityReadPath_FreeRecurringSeriesNoExtraEvents(t *testing.T) {
	st, calID := newRecurringSeriesFixture(t)
	// No bookings added.
	ctx := contextWithPrincipal(context.Background(), testAdminUser)
	combined := &combinedCalendarStore{base: st, domain: st}

	events, err := combined.ListEvents(ctx, calID, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("ListEvents with no bookings: got %d event(s), want 1", len(events))
	}
	if len(events) > 0 && events[0].RRule == "" {
		t.Errorf("expected the single event to be the series root (with RRule set)")
	}
}

// TestAvailabilityReadPath_SeriesRootETagChangesOnBooking verifies that the ETag
// of a series root event changes when a booking is created for one of its
// occurrences. This is essential for iOS Cache invalidation: iOS compares the
// ETag from PROPFIND Depth:1 against its cache; without a changed ETag it
// would not re-download the .ics body and would never see the inline override
// VEVENT.
func TestAvailabilityReadPath_SeriesRootETagChangesOnBooking(t *testing.T) {
	st, calID := newRecurringSeriesFixture(t)
	ctx := contextWithPrincipal(context.Background(), testAdminUser)
	combined := &combinedCalendarStore{base: st, domain: st}

	// ETag before booking.
	events, err := combined.ListEvents(ctx, calID, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ListEvents (before booking): %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	etagBefore := events[0].ETag

	// Create a booking for the first occurrence.
	seedBookingForSeries(t, st)

	// ETag after booking.
	events2, err2 := combined.ListEvents(ctx, calID, time.Time{}, time.Time{})
	if err2 != nil {
		t.Fatalf("ListEvents (after booking): %v", err2)
	}
	if len(events2) != 1 {
		t.Fatalf("expected 1 event after booking, got %d", len(events2))
	}
	etagAfter := events2[0].ETag

	if etagBefore == etagAfter {
		t.Errorf("ETag unchanged after booking: %q — iOS will not re-fetch the .ics and override VEVENT will not be seen", etagAfter)
	}
}

// TestAvailabilityReadPath_HTTP_RecurringSeriesBookedICS verifies that a GET on a
// booked recurring series .ics URL returns a VCALENDAR containing:
//   - the series root VEVENT with RRULE and TRANSP:TRANSPARENT
//   - an override VEVENT with RECURRENCE-ID, SUMMARY:"Anna Schmidt (Massage)", TRANSP:OPAQUE
//
// This is the iOS-visible fix: without the inline override VEVENT the
// availability calendar would show all occurrences as "Free".
func TestAvailabilityReadPath_HTTP_RecurringSeriesBookedICS(t *testing.T) {
	st, _ := newRecurringSeriesFixture(t)
	seedBookingForSeries(t, st)
	handler := newTestCaldavHandler(st, "")

	url := "http://example.com/caldav/user/calendars/avail-staff-1/series-uid-1.ics"
	req := httptest.NewRequest("GET", url, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET series .ics: status = %d, want 200\nbody: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()

	// Series root must be present and free (governs all non-overridden occurrences).
	if !strings.Contains(body, "RRULE:"+strings.TrimPrefix("FREQ=WEEKLY;COUNT=4", "RRULE:")) {
		// Check for the value portion only; iCal folding may split the line.
		if !strings.Contains(body, "FREQ=WEEKLY") {
			t.Errorf("series .ics missing RRULE with FREQ=WEEKLY\n%s", body)
		}
	}
	if !strings.Contains(body, "TRANSP:TRANSPARENT") {
		t.Errorf("series .ics missing TRANSP:TRANSPARENT (series root must be free)\n%s", body)
	}

	// Override VEVENT must be present.
	if !strings.Contains(body, "RECURRENCE-ID") {
		t.Errorf("series .ics missing RECURRENCE-ID override VEVENT\n%s", body)
	}
	if !strings.Contains(body, "Anna Schmidt (Massage)") {
		t.Errorf("series .ics missing SUMMARY:Anna Schmidt (Massage) in override VEVENT\n%s", body)
	}
	if !strings.Contains(body, "TRANSP:OPAQUE") {
		t.Errorf("series .ics missing TRANSP:OPAQUE in override VEVENT\n%s", body)
	}
}
