package domain

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"schedio/internal/store"
)

// ── stub store ────────────────────────────────────────────────────────────────

// availStub is a minimal store.DomainStore backed by function fields so that
// each test can inject exactly the behaviour it needs. All methods that are not
// set default to returning a zero/nil result without error.
type availStub struct {
	getService         func(id string) (*store.Service, error)
	listStaff          func() ([]*store.Staff, error)
	listTimeslots      func(userID string, start, end time.Time) ([]*store.Timeslot, error)
	listBookingsForDay func(date time.Time) ([]*store.Booking, error)
	listActiveInWindow func(userID string, start, end time.Time) ([]*store.Booking, error)
}

func (s *availStub) GetService(_ context.Context, id string) (*store.Service, error) {
	if s.getService != nil {
		return s.getService(id)
	}
	return nil, store.ErrNotFound
}
func (s *availStub) ListStaff(_ context.Context) ([]*store.Staff, error) {
	if s.listStaff != nil {
		return s.listStaff()
	}
	return nil, nil
}
func (s *availStub) ListTimeslots(_ context.Context, userID string, start, end time.Time) ([]*store.Timeslot, error) {
	if s.listTimeslots != nil {
		return s.listTimeslots(userID, start, end)
	}
	return nil, nil
}
func (s *availStub) ListBookingsForDay(_ context.Context, date time.Time) ([]*store.Booking, error) {
	if s.listBookingsForDay != nil {
		return s.listBookingsForDay(date)
	}
	return nil, nil
}
func (s *availStub) ListActiveBookingsInWindow(_ context.Context, userID string, start, end time.Time) ([]*store.Booking, error) {
	if s.listActiveInWindow != nil {
		return s.listActiveInWindow(userID, start, end)
	}
	return nil, nil
}

// Satisfy the rest of the DomainStore interface with no-ops.
func (s *availStub) SyncUsers(_ context.Context, _ []*store.User) error { return nil }
func (s *availStub) GetStaff(_ context.Context, _ string) (*store.Staff, error) {
	return nil, store.ErrNotFound
}
func (s *availStub) GetUserByEmail(_ context.Context, _ string) (*store.User, error) {
	return nil, store.ErrNotFound
}
func (s *availStub) ListServices(_ context.Context) ([]*store.Service, error) { return nil, nil }
func (s *availStub) CreateService(_ context.Context, _ *store.Service) error  { return nil }
func (s *availStub) UpdateService(_ context.Context, _ *store.Service) error  { return nil }
func (s *availStub) DeleteService(_ context.Context, _ string) error          { return nil }
func (s *availStub) GetTimeslot(_ context.Context, _, _ string) (*store.Timeslot, error) {
	return nil, store.ErrNotFound
}
func (s *availStub) UpsertTimeslot(_ context.Context, _ *store.Timeslot) error { return nil }
func (s *availStub) DeleteTimeslot(_ context.Context, _, _ string) error       { return nil }
func (s *availStub) GetOrCreateContact(_ context.Context, _ string, c *store.Contact) (*store.Contact, error) {
	return c, nil
}
func (s *availStub) GetContact(_ context.Context, _ string) (*store.Contact, error) {
	return nil, store.ErrNotFound
}
func (s *availStub) UpdateContactLastAppointment(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (s *availStub) CreateSession(_ context.Context, _ *store.BookingSession) error { return nil }
func (s *availStub) GetSession(_ context.Context, _ string) (*store.BookingSession, error) {
	return nil, store.ErrNotFound
}
func (s *availStub) UpdateSession(_ context.Context, _ *store.BookingSession) error { return nil }
func (s *availStub) ListPendingSessions(_ context.Context) ([]*store.BookingSession, error) {
	return nil, nil
}
func (s *availStub) CreateBooking(_ context.Context, _ *store.Booking) error { return nil }
func (s *availStub) GetBooking(_ context.Context, _ string) (*store.Booking, error) {
	return nil, store.ErrNotFound
}
func (s *availStub) UpdateBooking(_ context.Context, _ *store.Booking) error { return nil }
func (s *availStub) ListBookingsForSession(_ context.Context, _ string) ([]*store.Booking, error) {
	return nil, nil
}
func (s *availStub) ListBookingsForContact(_ context.Context, _ string) ([]*store.Booking, error) {
	return nil, nil
}
func (s *availStub) GetSettings(_ context.Context) (*store.Settings, error) {
	return &store.Settings{}, nil
}
func (s *availStub) UpdateSettings(_ context.Context, _ *store.Settings) error { return nil }
func (s *availStub) GetHMACSecret(_ context.Context) ([]byte, error)           { return nil, nil }
func (s *availStub) SetHMACSecret(_ context.Context, _ []byte) error           { return nil }
func (s *availStub) ListRetentionDue(_ context.Context, _ time.Duration) ([]*store.Contact, error) {
	return nil, nil
}
func (s *availStub) MarkRetentionNotified(_ context.Context, _ string) error { return nil }
func (s *availStub) ListConfirmationExpired(_ context.Context) ([]*store.Contact, error) {
	return nil, nil
}
func (s *availStub) AddToPendingDeletion(_ context.Context, _ string) error { return nil }
func (s *availStub) ListPendingDeletion(_ context.Context) ([]*store.Contact, error) {
	return nil, nil
}
func (s *availStub) DeleteContact(_ context.Context, _ string) error { return nil }
func (s *availStub) ListBillingDue(_ context.Context) ([]*store.Contact, error) {
	return nil, nil
}
func (s *availStub) MarkBillingGenerated(_ context.Context, _ string) error { return nil }

// ── helper ────────────────────────────────────────────────────────────────────

func hm(h, m int) time.Time {
	return time.Date(2026, 3, 2, h, m, 0, 0, time.UTC)
}

func slotTimes(slots []Slot) []string {
	out := make([]string, len(slots))
	for i, s := range slots {
		out[i] = s.StartAt.UTC().Format("15:04")
	}
	sort.Strings(out)
	return out
}

// ── unit tests: pure functions ─────────────────────────────────────────────────

func TestSnapUp(t *testing.T) {
	grid := 15 * time.Minute
	tests := []struct {
		in   time.Time
		want time.Time
	}{
		{hm(8, 0), hm(8, 0)},   // already on boundary
		{hm(8, 1), hm(8, 15)},  // 1 min past → snap to :15
		{hm(8, 14), hm(8, 15)}, // 14 min past → snap to :15
		{hm(8, 15), hm(8, 15)}, // exact boundary
		{hm(8, 16), hm(8, 30)}, // just past :15 → :30
		{hm(8, 59), hm(9, 0)},  // almost next hour
	}
	for _, tc := range tests {
		got := snapUp(tc.in, grid)
		if !got.Equal(tc.want) {
			t.Errorf("snapUp(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestTruncateToDay(t *testing.T) {
	tests := []struct {
		in   time.Time
		want time.Time
	}{
		{
			time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			time.Date(2026, 3, 2, 23, 59, 59, 999, time.UTC),
			time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			// Non-UTC location: truncation must use the UTC calendar date.
			time.Date(2026, 3, 2, 1, 0, 0, 0, time.FixedZone("UTC+2", 2*3600)),
			time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), // UTC date is March 1
		},
	}
	for _, tc := range tests {
		got := truncateToDay(tc.in)
		if !got.Equal(tc.want) {
			t.Errorf("truncateToDay(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestCountActiveBookings(t *testing.T) {
	svcA := "svc-a"
	svcB := "svc-b"
	bookings := []*store.Booking{
		{ServiceID: svcA, State: store.BookingStateReserved},
		{ServiceID: svcA, State: store.BookingStateConfirmed},
		{ServiceID: svcA, State: store.BookingStateCancelled}, // excluded
		{ServiceID: svcA, State: store.BookingStateNoShow},    // excluded
		{ServiceID: svcB, State: store.BookingStateConfirmed}, // wrong service
	}
	if got := countActiveBookings(bookings, svcA); got != 2 {
		t.Errorf("countActiveBookings = %d, want 2", got)
	}
	if got := countActiveBookings(bookings, svcB); got != 1 {
		t.Errorf("countActiveBookings for svcB = %d, want 1", got)
	}
	if got := countActiveBookings(nil, svcA); got != 0 {
		t.Errorf("countActiveBookings(nil) = %d, want 0", got)
	}
}

func TestEnumerateCandidates(t *testing.T) {
	tests := []struct {
		name      string
		start     time.Time
		end       time.Time
		duration  time.Duration
		wantTimes []string // "HH:mm"
	}{
		{
			name:      "exact fit — one slot",
			start:     hm(8, 0),
			end:       hm(9, 0),
			duration:  60 * time.Minute,
			wantTimes: []string{"08:00"},
		},
		{
			name:     "four-hour window 60min duration — 4 candidates on duration grid",
			start:    hm(8, 0),
			end:      hm(12, 0),
			duration: 60 * time.Minute,
			// 08:00, 09:00, 10:00, 11:00 (step = duration = 60 min)
			wantTimes: []string{"08:00", "09:00", "10:00", "11:00"},
		},
		{
			name:      "window too narrow — no slots",
			start:     hm(8, 0),
			end:       hm(8, 30),
			duration:  60 * time.Minute,
			wantTimes: nil,
		},
		{
			name:     "non-aligned start snapped up — 08:05 snaps to 09:00 on 60-min grid",
			start:    hm(8, 5),
			end:      hm(12, 0),
			duration: 60 * time.Minute,
			// snapUp(08:05, 60min) = 09:00 (next whole hour); then 09:00, 10:00, 11:00
			wantTimes: []string{"09:00", "10:00", "11:00"},
		},
		{
			name:     "30-minute duration — 8 slots at 30-min steps, last at 11:30",
			start:    hm(8, 0),
			end:      hm(12, 0),
			duration: 30 * time.Minute,
			// step = 30 min: 08:00, 08:30, 09:00, 09:30, 10:00, 10:30, 11:00, 11:30
			wantTimes: []string{"08:00", "08:30", "09:00", "09:30", "10:00", "10:30", "11:00", "11:30"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := enumerateCandidates(tc.start, tc.end, tc.duration)
			if len(got) != len(tc.wantTimes) {
				var gotS []string
				for _, g := range got {
					gotS = append(gotS, g.UTC().Format("15:04"))
				}
				t.Fatalf("got %d candidates %v, want %d %v",
					len(got), gotS, len(tc.wantTimes), tc.wantTimes)
			}
			for i, g := range got {
				if g.UTC().Format("15:04") != tc.wantTimes[i] {
					t.Errorf("candidate[%d] = %s, want %s",
						i, g.UTC().Format("15:04"), tc.wantTimes[i])
				}
			}
		})
	}
}

// ── integration tests: ListAvailable ─────────────────────────────────────────

const testSvcID = "svc-0001"
const testStaffID = "staff-0001"

func defaultService(durationMin, dailyLimit int) *store.Service {
	return &store.Service{
		ID:              testSvcID,
		Name:            "Test",
		DurationMinutes: durationMin,
		DailyLimit:      dailyLimit,
	}
}

func defaultStaff() []*store.Staff {
	return []*store.Staff{{ID: testStaffID, Role: store.UserRoleStaff}}
}

func timeslotFor(staffID string, startH, startM, endH, endM int) *store.Timeslot {
	return &store.Timeslot{
		ID:      "ts-1",
		UserID:  staffID,
		StartAt: time.Date(2026, 3, 2, startH, startM, 0, 0, time.UTC),
		EndAt:   time.Date(2026, 3, 2, endH, endM, 0, 0, time.UTC),
	}
}

var testDay = time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

func TestListAvailable_ServiceNotFound(t *testing.T) {
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return nil, store.ErrNotFound
		},
	})
	_, err := svc.ListAvailable(context.Background(), testSvcID, testDay)
	if err == nil {
		t.Fatal("expected error for missing service, got nil")
	}
}

func TestListAvailable_NoStaff(t *testing.T) {
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return defaultService(60, 0), nil
		},
		listStaff: func() ([]*store.Staff, error) { return nil, nil },
	})
	slots, err := svc.ListAvailable(context.Background(), testSvcID, testDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 0 {
		t.Errorf("expected 0 slots with no staff, got %d", len(slots))
	}
}

func TestListAvailable_NoTimeslots(t *testing.T) {
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return defaultService(60, 0), nil
		},
		listStaff: func() ([]*store.Staff, error) { return defaultStaff(), nil },
		listTimeslots: func(uid string, s, e time.Time) ([]*store.Timeslot, error) {
			return nil, nil
		},
	})
	slots, err := svc.ListAvailable(context.Background(), testSvcID, testDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 0 {
		t.Errorf("expected 0 slots, got %d", len(slots))
	}
}

func TestListAvailable_BasicSlots(t *testing.T) {
	// 08:00–10:00 window, 60-min service, no bookings → 5 candidates
	// 08:00, 08:15, 08:30, 08:45, 09:00 (all fit within 10:00)
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return defaultService(60, 0), nil
		},
		listStaff: func() ([]*store.Staff, error) { return defaultStaff(), nil },
		listTimeslots: func(uid string, s, e time.Time) ([]*store.Timeslot, error) {
			return []*store.Timeslot{timeslotFor(uid, 8, 0, 10, 0)}, nil
		},
	})
	slots, err := svc.ListAvailable(context.Background(), testSvcID, testDay)
	if err != nil {
		t.Fatal(err)
	}
	// Candidates: 08:00..09:00 inclusive on 15-min grid where start+60 ≤ 10:00
	want := []string{"08:00", "09:00"}
	got := slotTimes(slots)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slot[%d]: got %s, want %s", i, got[i], want[i])
		}
	}
}

func TestListAvailable_BookedWindowExcluded(t *testing.T) {
	// Window 08:00–10:00, 60-min service.
	// Staff has a booking at 08:00–09:00 → any candidate overlapping that window is blocked.
	// Candidates 08:00, 08:15, 08:30, 08:45 all overlap with 08:00–09:00.
	// Candidate 09:00–10:00 does not overlap → only one slot returned.
	booking := &store.Booking{
		UserID:  testStaffID,
		StartAt: time.Date(2026, 3, 2, 8, 0, 0, 0, time.UTC),
		EndAt:   time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC),
		State:   store.BookingStateConfirmed,
	}
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return defaultService(60, 0), nil
		},
		listStaff: func() ([]*store.Staff, error) { return defaultStaff(), nil },
		listTimeslots: func(uid string, s, e time.Time) ([]*store.Timeslot, error) {
			return []*store.Timeslot{timeslotFor(uid, 8, 0, 10, 0)}, nil
		},
		listActiveInWindow: func(uid string, start, end time.Time) ([]*store.Booking, error) {
			// Return booking only when the candidate window overlaps [08:00, 09:00).
			if start.Before(booking.EndAt) && end.After(booking.StartAt) {
				return []*store.Booking{booking}, nil
			}
			return nil, nil
		},
	})
	slots, err := svc.ListAvailable(context.Background(), testSvcID, testDay)
	if err != nil {
		t.Fatal(err)
	}
	got := slotTimes(slots)
	want := []string{"09:00"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got[0] != want[0] {
		t.Errorf("expected slot at 09:00, got %s", got[0])
	}
}

func TestListAvailable_DailyLimitReached(t *testing.T) {
	// DailyLimit = 2, already 2 active bookings today → no slots offered.
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return defaultService(60, 2 /* limit */), nil
		},
		listStaff: func() ([]*store.Staff, error) { return defaultStaff(), nil },
		listTimeslots: func(uid string, s, e time.Time) ([]*store.Timeslot, error) {
			return []*store.Timeslot{timeslotFor(uid, 8, 0, 12, 0)}, nil
		},
		listBookingsForDay: func(date time.Time) ([]*store.Booking, error) {
			return []*store.Booking{
				{ServiceID: testSvcID, State: store.BookingStateReserved},
				{ServiceID: testSvcID, State: store.BookingStateConfirmed},
			}, nil
		},
	})
	slots, err := svc.ListAvailable(context.Background(), testSvcID, testDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 0 {
		t.Errorf("expected 0 slots after daily limit reached, got %d", len(slots))
	}
}

func TestListAvailable_DailyLimitNotYetReached(t *testing.T) {
	// DailyLimit = 3, only 2 active bookings — slots must still be returned.
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return defaultService(60, 3), nil
		},
		listStaff: func() ([]*store.Staff, error) { return defaultStaff(), nil },
		listTimeslots: func(uid string, s, e time.Time) ([]*store.Timeslot, error) {
			return []*store.Timeslot{timeslotFor(uid, 8, 0, 10, 0)}, nil
		},
		listBookingsForDay: func(date time.Time) ([]*store.Booking, error) {
			return []*store.Booking{
				{ServiceID: testSvcID, State: store.BookingStateReserved},
				{ServiceID: testSvcID, State: store.BookingStateConfirmed},
			}, nil
		},
	})
	slots, err := svc.ListAvailable(context.Background(), testSvcID, testDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) == 0 {
		t.Error("expected slots when daily limit not yet reached, got 0")
	}
}

func TestListAvailable_CancelledBookingDoesNotCountTowardLimit(t *testing.T) {
	// DailyLimit = 1; only cancelled bookings exist → slot must be returned.
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return defaultService(60, 1), nil
		},
		listStaff: func() ([]*store.Staff, error) { return defaultStaff(), nil },
		listTimeslots: func(uid string, s, e time.Time) ([]*store.Timeslot, error) {
			return []*store.Timeslot{timeslotFor(uid, 8, 0, 10, 0)}, nil
		},
		listBookingsForDay: func(date time.Time) ([]*store.Booking, error) {
			return []*store.Booking{
				{ServiceID: testSvcID, State: store.BookingStateCancelled},
				{ServiceID: testSvcID, State: store.BookingStateNoShow},
			}, nil
		},
	})
	slots, err := svc.ListAvailable(context.Background(), testSvcID, testDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) == 0 {
		t.Error("cancelled bookings should not count toward daily limit")
	}
}

func TestListAvailable_ZeroDailyLimit_Unlimited(t *testing.T) {
	// DailyLimit = 0 means unlimited; even with many bookings slots are returned.
	occupied := make([]*store.Booking, 100)
	for i := range occupied {
		occupied[i] = &store.Booking{ServiceID: testSvcID, State: store.BookingStateConfirmed}
	}
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return defaultService(60, 0 /* unlimited */), nil
		},
		listStaff: func() ([]*store.Staff, error) { return defaultStaff(), nil },
		listTimeslots: func(uid string, s, e time.Time) ([]*store.Timeslot, error) {
			return []*store.Timeslot{timeslotFor(uid, 8, 0, 10, 0)}, nil
		},
		listBookingsForDay: func(date time.Time) ([]*store.Booking, error) {
			return occupied, nil
		},
	})
	slots, err := svc.ListAvailable(context.Background(), testSvcID, testDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) == 0 {
		t.Error("expected slots when daily limit is unlimited (0), got 0")
	}
}

func TestListAvailable_MultipleStaff(t *testing.T) {
	staff := []*store.Staff{
		{ID: "staff-A", Role: store.UserRoleStaff},
		{ID: "staff-B", Role: store.UserRoleStaff},
	}
	// Both staff have the same 08:00–09:00 window → two sets of candidates.
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return defaultService(60, 0), nil
		},
		listStaff: func() ([]*store.Staff, error) { return staff, nil },
		listTimeslots: func(uid string, s, e time.Time) ([]*store.Timeslot, error) {
			return []*store.Timeslot{{
				ID:      "ts-" + uid,
				UserID:  uid,
				StartAt: time.Date(2026, 3, 2, 8, 0, 0, 0, time.UTC),
				EndAt:   time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC),
			}}, nil
		},
	})
	slots, err := svc.ListAvailable(context.Background(), testSvcID, testDay)
	if err != nil {
		t.Fatal(err)
	}
	// Each staff contributes 1 candidate (08:00 only for a 60-min slot in a 1-h window).
	if len(slots) != 2 {
		t.Errorf("expected 2 slots (one per staff), got %d", len(slots))
	}
	userIDs := map[string]bool{}
	for _, s := range slots {
		userIDs[s.UserID] = true
	}
	for _, m := range staff {
		if !userIDs[m.ID] {
			t.Errorf("missing slot for staff %s", m.ID)
		}
	}
}

func TestListAvailable_OtherServiceBookingDoesNotBlockWindow(t *testing.T) {
	// A booking for a *different* service should not block windows
	// (ListActiveBookingsInWindow is keyed by staff user, not service).
	// This test verifies the daily limit uses the correct serviceID filter.
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return defaultService(60, 1 /* limit */), nil
		},
		listStaff: func() ([]*store.Staff, error) { return defaultStaff(), nil },
		listTimeslots: func(uid string, s, e time.Time) ([]*store.Timeslot, error) {
			return []*store.Timeslot{timeslotFor(uid, 8, 0, 10, 0)}, nil
		},
		listBookingsForDay: func(date time.Time) ([]*store.Booking, error) {
			return []*store.Booking{
				{ServiceID: "other-service", State: store.BookingStateConfirmed},
			}, nil
		},
	})
	slots, err := svc.ListAvailable(context.Background(), testSvcID, testDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) == 0 {
		t.Error("booking for another service should not consume daily limit of current service")
	}
}

// ── integration tests: ListAvailableForDateRange ──────────────────────────────

func TestListAvailableForDateRange_MultipleDays(t *testing.T) {
	// Monday 2026-03-02 has a timeslot, Tuesday 2026-03-03 does not.
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return defaultService(60, 0), nil
		},
		listStaff: func() ([]*store.Staff, error) { return defaultStaff(), nil },
		listTimeslots: func(uid string, s, e time.Time) ([]*store.Timeslot, error) {
			// Only return a timeslot when queried for March 2.
			if s.Day() == 2 {
				return []*store.Timeslot{{
					ID:      "ts-1",
					UserID:  uid,
					StartAt: time.Date(2026, 3, 2, 8, 0, 0, 0, time.UTC),
					EndAt:   time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC),
				}}, nil
			}
			return nil, nil
		},
	})
	rangeStart := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	result, err := svc.ListAvailableForDateRange(context.Background(), testSvcID, rangeStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result["2026-03-02"]; !ok {
		t.Error("expected slots on 2026-03-02")
	}
	if _, ok := result["2026-03-03"]; ok {
		t.Error("expected no entry for 2026-03-03 (no timeslot)")
	}
}

func TestListAvailableForDateRange_EmptyRange(t *testing.T) {
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return defaultService(60, 0), nil
		},
		listStaff: func() ([]*store.Staff, error) { return defaultStaff(), nil },
	})
	t1 := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	result, err := svc.ListAvailableForDateRange(context.Background(), testSvcID, t1, t1)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("empty range should return empty map, got %v", result)
	}
}

func TestListAvailableForDateRange_ErrorPropagates(t *testing.T) {
	svc := NewAvailabilityService(&availStub{
		getService: func(id string) (*store.Service, error) {
			return nil, fmt.Errorf("db unavailable")
		},
	})
	rangeStart := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	_, err := svc.ListAvailableForDateRange(context.Background(), testSvcID, rangeStart, rangeEnd)
	if err == nil {
		t.Fatal("expected error to propagate from ListAvailable, got nil")
	}
}

// ── integration tests: config/availability.yaml fixture ─────────────────────
//
// These tests mirror the exact data from config/availability.yaml and
// config/users.yaml so that the fixture used in production is also exercised
// by the test suite.  IDs and RRules below are copied verbatim from those
// files; any deviation is a bug.

const (
	annaID   = "u0000000-0000-4000-8000-000000000001"
	svcShort = "d0000000-0000-4000-8000-000000000001" // Beratungsgespräch 30 min, no limit
	svcLong  = "d0000000-0000-4000-8000-000000000002" // Standardbehandlung 60 min, limit 4
)

// fixtureStore builds a MemoryStore seeded with the same data as the YAML
// config files.
func fixtureStore(t *testing.T) *store.MemoryStore {
	t.Helper()
	ctx := context.Background()
	ms := store.NewMemoryStore()

	// Staff user – Anna Müller
	if err := ms.SyncUsers(ctx, []*store.User{{
		ID:   annaID,
		Role: store.UserRoleStaff,
		Name: "Anna Müller",
	}}); err != nil {
		t.Fatal(err)
	}

	// Two default services
	for _, s := range []store.Service{
		{ID: svcShort, Name: "Beratungsgespräch", DurationMinutes: 30, DailyLimit: 0},
		{ID: svcLong, Name: "Standardbehandlung", DurationMinutes: 60, DailyLimit: 4},
	} {
		cp := s
		if err := ms.CreateService(ctx, &cp); err != nil {
			t.Fatal(err)
		}
	}

	// Recurring morning block: Mon–Fri 08:00–12:00 UTC, 260 occurrences (52 weeks)
	if err := ms.UpsertTimeslot(ctx, &store.Timeslot{
		ID:        "ts-anna-morning-2026",
		CalDAVUID: "ts-anna-morning-2026",
		UserID:    annaID,
		StartAt:   time.Date(2026, 3, 2, 8, 0, 0, 0, time.UTC),
		EndAt:     time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC),
		RRule:     "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR;COUNT=260",
	}); err != nil {
		t.Fatal(err)
	}

	// Recurring afternoon block: Mon–Fri 13:00–17:00 UTC, 260 occurrences (52 weeks)
	if err := ms.UpsertTimeslot(ctx, &store.Timeslot{
		ID:        "ts-anna-afternoon-2026",
		CalDAVUID: "ts-anna-afternoon-2026",
		UserID:    annaID,
		StartAt:   time.Date(2026, 3, 2, 13, 0, 0, 0, time.UTC),
		EndAt:     time.Date(2026, 3, 2, 17, 0, 0, 0, time.UTC),
		RRule:     "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR;COUNT=260",
	}); err != nil {
		t.Fatal(err)
	}

	// One-off Saturday morning: 2026-03-07 09:00–12:00 UTC
	if err := ms.UpsertTimeslot(ctx, &store.Timeslot{
		ID:        "ts-anna-saturday-2026-03-07",
		CalDAVUID: "ts-anna-saturday-2026-03-07",
		UserID:    annaID,
		StartAt:   time.Date(2026, 3, 7, 9, 0, 0, 0, time.UTC),
		EndAt:     time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	return ms
}

// TestFixture_SeedDate_ShortService checks the seed date (Mon 2026-03-02) for
// the 30-min service (no daily limit).
//
// Expected slots = morning (08:00–11:30, 15 slots) + afternoon (13:00–16:30, 15
// slots) = 30 slots.
func TestFixture_SeedDate_ShortService(t *testing.T) {
	svc := NewAvailabilityService(fixtureStore(t))
	slots, err := svc.ListAvailable(context.Background(), svcShort,
		time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	// morning: 08:00..11:30 (step 30min) → 8 starts
	// afternoon: 13:00..16:30 → 8 starts  → total 16
	const wantCount = 16
	if len(slots) != wantCount {
		t.Errorf("seed date 30-min: got %d slots, want %d\nslots: %v",
			len(slots), wantCount, slotTimes(slots))
	}
	// Spot-check first and last morning slot and first afternoon slot.
	times := slotTimes(slots)
	assertContains(t, times, "08:00", "first morning slot")
	assertContains(t, times, "11:30", "last morning slot")
	assertContains(t, times, "13:00", "first afternoon slot")
	assertContains(t, times, "16:30", "last afternoon slot")
	assertNotContains(t, times, "12:00", "gap between morning and afternoon")
	assertNotContains(t, times, "11:45", "11:45 start would end at 12:15, outside morning window")
}

// TestFixture_SeedDate_LongService checks the seed date for the 60-min service
// (daily limit 4, 0 existing bookings).
//
// Expected: morning 08:00–11:00 (4 slots) + afternoon 13:00–16:00 (4 slots)
// = 8 slots (daily limit not yet hit).
func TestFixture_SeedDate_LongService(t *testing.T) {
	svc := NewAvailabilityService(fixtureStore(t))
	slots, err := svc.ListAvailable(context.Background(), svcLong,
		time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	const wantCount = 8
	if len(slots) != wantCount {
		t.Errorf("seed date 60-min: got %d slots, want %d\nslots: %v",
			len(slots), wantCount, slotTimes(slots))
	}
	times := slotTimes(slots)
	assertContains(t, times, "08:00", "first morning slot")
	assertContains(t, times, "11:00", "last morning slot")
	assertNotContains(t, times, "11:15", "11:15+60=12:15 exceeds morning window end")
	assertContains(t, times, "13:00", "first afternoon slot")
	assertContains(t, times, "16:00", "last afternoon slot")
	assertNotContains(t, times, "16:15", "16:15+60=17:15 exceeds afternoon window end")
}

// TestFixture_NextMonday verifies the second Monday occurrence (2026-03-09)
// produces the same slot pattern as the seed date.
func TestFixture_NextMonday(t *testing.T) {
	svc := NewAvailabilityService(fixtureStore(t))
	slots, err := svc.ListAvailable(context.Background(), svcShort,
		time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 16 {
		t.Errorf("next Monday 30-min: got %d slots, want 16\nslots: %v",
			len(slots), slotTimes(slots))
	}
}

// TestFixture_Tuesday verifies a Tuesday (2026-03-03) also expands correctly.
func TestFixture_Tuesday(t *testing.T) {
	svc := NewAvailabilityService(fixtureStore(t))
	slots, err := svc.ListAvailable(context.Background(), svcShort,
		time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 16 {
		t.Errorf("Tuesday 30-min: got %d slots, want 16\nslots: %v",
			len(slots), slotTimes(slots))
	}
}

// TestFixture_Saturday_OneOff checks the one-off Saturday timeslot 09:00–12:00.
//
// 30-min service: 09:00..11:30 → 11 slots.
// 60-min service: 09:00..11:00 → 9 slots.
func TestFixture_Saturday_OneOff(t *testing.T) {
	saturday := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	svc := NewAvailabilityService(fixtureStore(t))

	t.Run("30min", func(t *testing.T) {
		slots, err := svc.ListAvailable(context.Background(), svcShort, saturday)
		if err != nil {
			t.Fatal(err)
		}
		const want = 6 // 09:00, 09:30, 10:00, 10:30, 11:00, 11:30
		if len(slots) != want {
			t.Errorf("Saturday 30-min: got %d slots, want %d\nslots: %v",
				len(slots), want, slotTimes(slots))
		}
		times := slotTimes(slots)
		assertContains(t, times, "09:00", "first slot")
		assertContains(t, times, "11:30", "last slot")
		assertNotContains(t, times, "08:45", "before one-off window")
		assertNotContains(t, times, "11:45", "11:45+30=12:15 past window end")
	})

	t.Run("60min", func(t *testing.T) {
		slots, err := svc.ListAvailable(context.Background(), svcLong, saturday)
		if err != nil {
			t.Fatal(err)
		}
		const want = 3 // 09:00, 10:00, 11:00
		if len(slots) != want {
			t.Errorf("Saturday 60-min: got %d slots, want %d\nslots: %v",
				len(slots), want, slotTimes(slots))
		}
		times := slotTimes(slots)
		assertContains(t, times, "09:00", "first slot")
		assertContains(t, times, "11:00", "last slot")
		assertNotContains(t, times, "11:15", "11:15+60=12:15 past window end")
	})
}

// TestFixture_Sunday_NoSlots verifies that Sunday produces no slots because
// the recurring RRule only covers Mon–Fri and there is no one-off override.
func TestFixture_Sunday_NoSlots(t *testing.T) {
	svc := NewAvailabilityService(fixtureStore(t))
	// 2026-03-08 is a Sunday.
	slots, err := svc.ListAvailable(context.Background(), svcShort,
		time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 0 {
		t.Errorf("Sunday: expected 0 slots, got %d: %v", len(slots), slotTimes(slots))
	}
}

// TestFixture_DailyLimitBlocksAllSlots verifies that once 4 active bookings
// exist for the 60-min service on a given day, zero slots are returned.
func TestFixture_DailyLimitBlocksAllSlots(t *testing.T) {
	ms := fixtureStore(t)
	// Insert 4 confirmed bookings for svcLong on 2026-03-02.
	for i := range 4 {
		b := &store.Booking{
			ID:        fmt.Sprintf("bk-%d", i),
			ServiceID: svcLong,
			UserID:    annaID,
			StartAt:   time.Date(2026, 3, 2, 8+i, 0, 0, 0, time.UTC),
			EndAt:     time.Date(2026, 3, 2, 9+i, 0, 0, 0, time.UTC),
			State:     store.BookingStateConfirmed,
		}
		if err := ms.CreateBooking(context.Background(), b); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewAvailabilityService(ms)
	slots, err := svc.ListAvailable(context.Background(), svcLong,
		time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 0 {
		t.Errorf("daily limit reached: expected 0 slots, got %d", len(slots))
	}
}

// TestFixture_MonthRange checks ListAvailableForDateRange over the first week
// of March 2026 (Mon–Sun), expecting slots on Mon–Fri + Saturday, none on Sunday.
func TestFixture_MonthRange(t *testing.T) {
	svc := NewAvailabilityService(fixtureStore(t))
	// Mon 2026-03-02 .. Sun 2026-03-08 inclusive (rangeEnd is exclusive: 2026-03-09)
	rangeStart := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)

	result, err := svc.ListAvailableForDateRange(context.Background(), svcShort, rangeStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}

	wantDays := []string{"2026-03-02", "2026-03-03", "2026-03-04", "2026-03-05", "2026-03-06", "2026-03-07"}
	for _, day := range wantDays {
		if _, ok := result[day]; !ok {
			t.Errorf("expected slots for %s, got none", day)
		}
	}
	// Sunday must be absent.
	if _, ok := result["2026-03-08"]; ok {
		t.Errorf("expected no slots for 2026-03-08 (Sunday), but got some")
	}
	// Slot counts per day.
	for _, weekday := range wantDays[:5] { // Mon–Fri: 16 slots each
		if got := len(result[weekday]); got != 16 {
			t.Errorf("%s (weekday) 30-min: got %d slots, want 16", weekday, got)
		}
	}
	if got := len(result["2026-03-07"]); got != 6 { // Saturday one-off
		t.Errorf("2026-03-07 (Saturday one-off) 30-min: got %d slots, want 6", got)
	}
}

// TestFixture_MidYear confirms that slots are still produced roughly 6 months
// after the seed date (2026-10-05, a Monday), well within the 52-week window.
func TestFixture_MidYear(t *testing.T) {
	svc := NewAvailabilityService(fixtureStore(t))
	// 2026-10-05 is a Monday, ~7 months after the seed date.
	slots, err := svc.ListAvailable(context.Background(), svcShort,
		time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 16 {
		t.Errorf("mid-year Monday 30-min: got %d slots, want 16\nslots: %v",
			len(slots), slotTimes(slots))
	}
}

// TestFixture_AfterRuleEnd verifies that no slots are produced after the 260th
// occurrence of the recurring timeslots has passed.
//
// With FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR;COUNT=260 starting 2026-03-02, the
// 260th occurrence is Friday 2027-02-26.  Monday 2027-03-01 must return no slots
// from the recurring rules (no one-off exists either).
func TestFixture_AfterRuleEnd(t *testing.T) {
	svc := NewAvailabilityService(fixtureStore(t))
	// 2027-03-01 is a Monday after the last occurrence (2027-02-26).
	afterEnd := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	slots, err := svc.ListAvailable(context.Background(), svcShort, afterEnd)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 0 {
		t.Errorf("after rule end: expected 0 slots, got %d: %v", len(slots), slotTimes(slots))
	}
}

// helpers used by fixture tests
func assertContains(t *testing.T, haystack []string, needle, label string) {
	t.Helper()
	for _, s := range haystack {
		if s == needle {
			return
		}
	}
	t.Errorf("expected slot at %s (%s), not found in %v", needle, label, haystack)
}

func assertNotContains(t *testing.T, haystack []string, needle, label string) {
	t.Helper()
	for _, s := range haystack {
		if s == needle {
			t.Errorf("unexpected slot at %s (%s) found in %v", needle, label, haystack)
			return
		}
	}
}

// ── integration tests: MemoryStore + RRule expansion ─────────────────────────

// TestListAvailable_RecurringTimeslot verifies that a Timeslot with an RRule
// produces slots on subsequent occurrences, not only on the seed date.
//
// The seed date is 2026-03-02 (Monday). The RRule repeats every weekday.
// The test queries 2026-03-09 (the following Monday) and expects to find slots.
func TestListAvailable_RecurringTimeslot(t *testing.T) {
	ctx := context.Background()
	ms := store.NewMemoryStore()

	// Seed one staff user.
	if err := ms.SyncUsers(ctx, []*store.User{{
		ID:   testStaffID,
		Role: store.UserRoleStaff,
	}}); err != nil {
		t.Fatal(err)
	}
	// Seed the service.
	if err := ms.CreateService(ctx, &store.Service{
		ID:              testSvcID,
		Name:            "Test",
		DurationMinutes: 60,
	}); err != nil {
		t.Fatal(err)
	}
	// Seed the recurring timeslot (seed date = 2026-03-02, weekly Mon–Fri).
	if err := ms.UpsertTimeslot(ctx, &store.Timeslot{
		ID:        "ts-recur-1",
		CalDAVUID: "ts-recur-1",
		UserID:    testStaffID,
		StartAt:   time.Date(2026, 3, 2, 8, 0, 0, 0, time.UTC),
		EndAt:     time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC),
		RRule:     "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR;COUNT=10",
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewAvailabilityService(ms)

	// Query the seed date — must work.
	seedDay := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	slots, err := svc.ListAvailable(ctx, testSvcID, seedDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) == 0 {
		t.Error("expected slots on seed date 2026-03-02, got none")
	}

	// Query the following Monday (second occurrence) — this is where the bug
	// manifests: without RRule expansion, no slots are returned.
	nextMonday := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	slots, err = svc.ListAvailable(ctx, testSvcID, nextMonday)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) == 0 {
		t.Error("expected slots on 2026-03-09 (second Monday occurrence of recurring timeslot), got none — RRule not expanded")
	}

	// A Saturday must NOT produce slots (not in BYDAY=MO,TU,WE,TH,FR).
	saturday := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	slots, err = svc.ListAvailable(ctx, testSvcID, saturday)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 0 {
		t.Errorf("expected 0 slots on Saturday 2026-03-07 (outside RRule BYDAY), got %d", len(slots))
	}
}
