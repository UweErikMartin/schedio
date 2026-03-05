package store

// timeslot_overlap_test.go verifies that UpsertAvailability rejects any new or
// updated availability record whose effective time windows overlap with an existing
// availability record belonging to the same staff user.
//
// "Effective windows" are:
//   - Single event   → its own [StartAt, EndAt) interval.
//   - Recurring root → each non-EXDATE RRULE occurrence (minus dates that have
//     a stored override, which contributes its own window).
//   - Override       → its own [StartAt, EndAt) interval.

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

var ctx = context.Background()

const userA = "user-a"
const userB = "user-b"

// day returns a UTC time for hour h on the fixed reference date 2026-03-02.
func day(h, m int) time.Time {
	return time.Date(2026, 3, 2, h, m, 0, 0, time.UTC)
}

// singleSlot constructs a non-recurring Availability record.
func singleSlot(uid, userID string, start, end time.Time) *Availability {
	return &Availability{
		ID:        uid,
		UserID:    userID,
		CalDAVUID: uid,
		StartAt:   start,
		EndAt:     end,
	}
}

// recurringSlot constructs a recurring series root Availability record.
func recurringSlot(uid, userID string, start, end time.Time, rrule string, exDates ...time.Time) *Availability {
	return &Availability{
		ID:        uid,
		UserID:    userID,
		CalDAVUID: uid,
		StartAt:   start,
		EndAt:     end,
		RRule:     rrule,
		ExDates:   exDates,
	}
}

// overrideSlot constructs an override record for a recurring series.
func overrideSlot(uid, userID string, recurrenceID, start, end time.Time) *Availability {
	return &Availability{
		ID:           uid + "-override-" + recurrenceID.Format("20060102"),
		UserID:       userID,
		CalDAVUID:    uid,
		RecurrenceID: recurrenceID,
		StartAt:      start,
		EndAt:        end,
	}
}

// mustUpsert fails the test if UpsertAvailability returns an error.
func mustUpsert(t *testing.T, s *MemoryStore, slot *Availability) {
	t.Helper()
	if err := s.UpsertAvailability(ctx, slot); err != nil {
		t.Fatalf("UpsertAvailability(%q) unexpectedly failed: %v", slot.CalDAVUID, err)
	}
}

// wantConflict fails the test unless UpsertAvailability returns ErrConflict.
func wantConflict(t *testing.T, s *MemoryStore, slot *Availability) {
	t.Helper()
	err := s.UpsertAvailability(ctx, slot)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("UpsertAvailability(%q) got %v, want ErrConflict", slot.CalDAVUID, err)
	}
}

// ─── non-recurring vs non-recurring ──────────────────────────────────────────

// Two adjacent single events (touching boundaries are NOT overlapping).
func TestAvailabilityOverlap_NonRecurring_Adjacent_OK(t *testing.T) {
	s := NewMemoryStore()
	mustUpsert(t, s, singleSlot("a", userA, day(9, 0), day(10, 0)))
	// Starts exactly when "a" ends — no overlap.
	mustUpsert(t, s, singleSlot("b", userA, day(10, 0), day(11, 0)))
}

// Complete enclosure: "b" is fully inside "a".
func TestAvailabilityOverlap_NonRecurring_Enclose_Conflict(t *testing.T) {
	s := NewMemoryStore()
	mustUpsert(t, s, singleSlot("a", userA, day(9, 0), day(11, 0)))
	wantConflict(t, s, singleSlot("b", userA, day(9, 30), day(10, 30)))
}

// Partial overlap: "b" starts before "a" ends.
func TestAvailabilityOverlap_NonRecurring_Partial_Conflict(t *testing.T) {
	s := NewMemoryStore()
	mustUpsert(t, s, singleSlot("a", userA, day(9, 0), day(10, 0)))
	wantConflict(t, s, singleSlot("b", userA, day(9, 30), day(10, 30)))
}

// Identical window: inserting two events at exactly the same time.
func TestAvailabilityOverlap_NonRecurring_Identical_Conflict(t *testing.T) {
	s := NewMemoryStore()
	mustUpsert(t, s, singleSlot("a", userA, day(9, 0), day(10, 0)))
	wantConflict(t, s, singleSlot("b", userA, day(9, 0), day(10, 0)))
}

// Gap between two events: a third event fits in the gap.
func TestAvailabilityOverlap_NonRecurring_FitsInGap_OK(t *testing.T) {
	s := NewMemoryStore()
	mustUpsert(t, s, singleSlot("a", userA, day(8, 0), day(9, 0)))
	mustUpsert(t, s, singleSlot("b", userA, day(10, 0), day(11, 0)))
	// Fits in the gap between 09:00–10:00.
	mustUpsert(t, s, singleSlot("c", userA, day(9, 0), day(10, 0)))
}

// Overlap between userA's event and userB's event is always allowed.
func TestAvailabilityOverlap_DifferentUsers_NeverConflict(t *testing.T) {
	s := NewMemoryStore()
	mustUpsert(t, s, singleSlot("a", userA, day(9, 0), day(10, 0)))
	// Same time, same UID even — but different user, must not conflict.
	mustUpsert(t, s, singleSlot("a", userB, day(9, 0), day(10, 0)))
}

// ─── update of existing record ────────────────────────────────────────────────

// Updating an event (same UID) to a different non-conflicting window is OK.
func TestAvailabilityOverlap_Update_NonConflicting_OK(t *testing.T) {
	s := NewMemoryStore()
	mustUpsert(t, s, singleSlot("a", userA, day(9, 0), day(10, 0)))
	// Move to 11:00–12:00: no conflict.
	mustUpsert(t, s, singleSlot("a", userA, day(11, 0), day(12, 0)))
}

// Updating an event to a window that overlaps with a different event returns
// ErrConflict.
func TestAvailabilityOverlap_Update_IntoConflict(t *testing.T) {
	s := NewMemoryStore()
	mustUpsert(t, s, singleSlot("a", userA, day(9, 0), day(10, 0)))
	mustUpsert(t, s, singleSlot("b", userA, day(11, 0), day(12, 0)))
	// Move "a" to 11:30–12:30: overlaps "b".
	wantConflict(t, s, singleSlot("a", userA, day(11, 30), day(12, 30)))
}

// Upserting a record with exactly the same window (pure re-save) must succeed
// because a record does not conflict with itself.
func TestAvailabilityOverlap_Update_SameWindow_OK(t *testing.T) {
	s := NewMemoryStore()
	mustUpsert(t, s, singleSlot("a", userA, day(9, 0), day(10, 0)))
	// Re-save with unchanged window.
	mustUpsert(t, s, singleSlot("a", userA, day(9, 0), day(10, 0)))
}

// ─── non-recurring vs recurring series ───────────────────────────────────────

// A single event that lands exactly on a recurring occurrence conflicts.
func TestAvailabilityOverlap_Single_vs_RecurringOccurrence_Conflict(t *testing.T) {
	s := NewMemoryStore()
	// Weekly on Mondays 09:00–10:00 starting 2026-03-02 (a Monday).
	mustUpsert(t, s, recurringSlot("weekly", userA, day(9, 0), day(10, 0), "FREQ=WEEKLY;COUNT=10"))
	// 2026-03-09 (next Monday) 09:30–10:00 → overlaps week-2 occurrence.
	next := day(9, 30).AddDate(0, 0, 7)
	wantConflict(t, s, singleSlot("single", userA, next, next.Add(30*time.Minute)))
}

// A single event in the gap between two recurring occurrences is fine.
func TestAvailabilityOverlap_Single_BetweenRecurringOccurrences_OK(t *testing.T) {
	s := NewMemoryStore()
	// Weekly on Mondays 09:00–10:00.
	mustUpsert(t, s, recurringSlot("weekly", userA, day(9, 0), day(10, 0), "FREQ=WEEKLY;COUNT=10"))
	// 2026-03-04 (Wednesday) 09:00–10:00 → no Monday occurrence on that day.
	wed := time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC)
	mustUpsert(t, s, singleSlot("single", userA, wed, wed.Add(time.Hour)))
}

// A single event that falls on an EXDATE gap of a recurring series is OK.
func TestAvailabilityOverlap_Single_OnExDateGap_OK(t *testing.T) {
	s := NewMemoryStore()
	// Weekly Mondays 09:00–10:00; skip 2026-03-09 via EXDATE.
	skipDate := day(9, 0).AddDate(0, 0, 7)
	mustUpsert(t, s, recurringSlot("weekly", userA, day(9, 0), day(10, 0),
		"FREQ=WEEKLY;COUNT=10", skipDate))
	// Single event on the skipped Monday should be allowed.
	mustUpsert(t, s, singleSlot("single", userA, skipDate, skipDate.Add(time.Hour)))
}

// A single event that overlaps with a recurring occurrence that later gets an
// EXDATE must be added AFTER the EXDATE is applied (update recurring to add
// EXDATE, then add single).
func TestAvailabilityOverlap_ExDate_Applied_Before_Single_OK(t *testing.T) {
	s := NewMemoryStore()
	// Weekly Mondays 09:00–10:00, no EXDATE yet.
	skipDate := day(9, 0).AddDate(0, 0, 7)
	mustUpsert(t, s, recurringSlot("weekly", userA, day(9, 0), day(10, 0), "FREQ=WEEKLY;COUNT=10"))
	// Adding single on second Monday should conflict.
	wantConflict(t, s, singleSlot("single", userA, skipDate, skipDate.Add(time.Hour)))
	// Now update the series to exclude that Monday.
	mustUpsert(t, s, recurringSlot("weekly", userA, day(9, 0), day(10, 0),
		"FREQ=WEEKLY;COUNT=10", skipDate))
	// Now the single event should succeed.
	mustUpsert(t, s, singleSlot("single", userA, skipDate, skipDate.Add(time.Hour)))
}

// ─── recurring vs recurring ───────────────────────────────────────────────────

// Two daily series with non-overlapping windows on the same days are fine.
func TestAvailabilityOverlap_Recurring_vs_Recurring_NoConflict(t *testing.T) {
	s := NewMemoryStore()
	// Daily 09:00–10:00 for 5 days.
	mustUpsert(t, s, recurringSlot("morning", userA, day(9, 0), day(10, 0), "FREQ=DAILY;COUNT=5"))
	// Daily 10:00–11:00 for 5 days — touches but does not overlap.
	mustUpsert(t, s, recurringSlot("afternoon", userA, day(10, 0), day(11, 0), "FREQ=DAILY;COUNT=5"))
}

// Two daily series whose occurrences overlap conflict.
func TestAvailabilityOverlap_Recurring_vs_Recurring_Conflict(t *testing.T) {
	s := NewMemoryStore()
	mustUpsert(t, s, recurringSlot("a", userA, day(9, 0), day(10, 0), "FREQ=DAILY;COUNT=5"))
	wantConflict(t, s, recurringSlot("b", userA, day(9, 30), day(10, 30), "FREQ=DAILY;COUNT=5"))
}

// Bi-weekly and weekly series that share a common occurrence conflict.
func TestAvailabilityOverlap_Recurring_BiWeekly_vs_Weekly_Conflict(t *testing.T) {
	s := NewMemoryStore()
	// Every Monday 09:00–10:00 for 4 weeks.
	mustUpsert(t, s, recurringSlot("weekly", userA, day(9, 0), day(10, 0), "FREQ=WEEKLY;COUNT=4"))
	// Every two weeks starting same Monday 09:30–10:00 — overlaps week-1.
	wantConflict(t, s, recurringSlot("biweekly", userA, day(9, 30), day(10, 0), "FREQ=WEEKLY;INTERVAL=2;COUNT=2"))
}

// ─── overrides ────────────────────────────────────────────────────────────────

// An override that moves an occurrence to a free slot is OK.
func TestAvailabilityOverlap_Override_MovedToFreeSlot_OK(t *testing.T) {
	s := NewMemoryStore()
	// Weekly Mondays 09:00–10:00.
	mustUpsert(t, s, recurringSlot("weekly", userA, day(9, 0), day(10, 0), "FREQ=WEEKLY;COUNT=4"))
	// Override week-2: move to Tuesday 11:00–12:00 (free slot).
	week2Monday := day(9, 0).AddDate(0, 0, 7)
	week2Tuesday := time.Date(2026, 3, 10, 11, 0, 0, 0, time.UTC)
	mustUpsert(t, s, overrideSlot("weekly", userA, week2Monday, week2Tuesday, week2Tuesday.Add(time.Hour)))
}

// An override moved into an occupied slot conflicts.
func TestAvailabilityOverlap_Override_MovedIntoOccupied_Conflict(t *testing.T) {
	s := NewMemoryStore()
	// Weekly Mondays 09:00–10:00 for 4 weeks (weeks 1–4: Mar 2, 9, 16, 23).
	mustUpsert(t, s, recurringSlot("weekly", userA, day(9, 0), day(10, 0), "FREQ=WEEKLY;COUNT=4"))
	// A separate blocker on Tuesday week-3 09:00–10:00 (not a Monday series day).
	week3Tuesday := time.Date(2026, 3, 17, 9, 0, 0, 0, time.UTC)
	mustUpsert(t, s, singleSlot("blocker", userA, week3Tuesday, week3Tuesday.Add(time.Hour)))
	// Override week-2: try to move to Tuesday week-3 09:00 → conflicts with blocker.
	week2Monday := day(9, 0).AddDate(0, 0, 7)
	wantConflict(t, s, overrideSlot("weekly", userA, week2Monday, week3Tuesday, week3Tuesday.Add(time.Hour)))
}

// The override's RecurrenceID slot would otherwise be counted from the series
// root expansion; it must NOT conflict with itself when stored alongside the root.
func TestAvailabilityOverlap_Override_DoesNotConflictWithItsOwnSeriesSlot(t *testing.T) {
	s := NewMemoryStore()
	// Weekly Mondays 09:00–10:00 for 4 weeks.
	mustUpsert(t, s, recurringSlot("weekly", userA, day(9, 0), day(10, 0), "FREQ=WEEKLY;COUNT=4"))
	// Override for week-1 itself: same window, simulating a metadata-only change.
	week1 := day(9, 0)
	mustUpsert(t, s, overrideSlot("weekly", userA, week1, week1, week1.Add(time.Hour)))
}

// Updating an existing override to a non-conflicting window is OK (no
// self-conflict with the previous version of the override).
func TestAvailabilityOverlap_Override_Update_OK(t *testing.T) {
	s := NewMemoryStore()
	mustUpsert(t, s, recurringSlot("weekly", userA, day(9, 0), day(10, 0), "FREQ=WEEKLY;COUNT=4"))
	week2Monday := day(9, 0).AddDate(0, 0, 7)
	// Initial override: move to Tuesday.
	week2Tuesday := time.Date(2026, 3, 10, 11, 0, 0, 0, time.UTC)
	mustUpsert(t, s, overrideSlot("weekly", userA, week2Monday, week2Tuesday, week2Tuesday.Add(time.Hour)))
	// Re-save the same override with a slightly different time — no conflict.
	week2TuesdayShifted := time.Date(2026, 3, 10, 11, 30, 0, 0, time.UTC)
	mustUpsert(t, s, overrideSlot("weekly", userA, week2Monday, week2TuesdayShifted, week2TuesdayShifted.Add(time.Hour)))
}

// ─── series root expansion excludes dates with stored overrides ───────────────

// A single event on week-2 Monday is normally blocked by the series. After
// storing an override that moves week-2 away, the slot is freed and the single
// event can be inserted.
func TestAvailabilityOverlap_Override_FreesRecurrenceSlot(t *testing.T) {
	s := NewMemoryStore()
	mustUpsert(t, s, recurringSlot("weekly", userA, day(9, 0), day(10, 0), "FREQ=WEEKLY;COUNT=4"))

	week2Monday := day(9, 0).AddDate(0, 0, 7)
	// Confirm the slot is occupied.
	wantConflict(t, s, singleSlot("single", userA, week2Monday, week2Monday.Add(time.Hour)))

	// Store override that moves week-2 to Tuesday: slot on Monday is freed.
	week2Tuesday := time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC)
	mustUpsert(t, s, overrideSlot("weekly", userA, week2Monday, week2Tuesday, week2Tuesday.Add(time.Hour)))

	// Now the original Monday slot for week-2 is free.
	mustUpsert(t, s, singleSlot("single", userA, week2Monday, week2Monday.Add(time.Hour)))
}
