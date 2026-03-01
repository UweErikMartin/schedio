// Package domain implements pure business logic for the schedio booking
// system. It has no HTTP dependencies and no direct database access; all
// persistence is delegated to a store.DomainStore and store.CalendarStore
// passed in via constructor arguments.
//
// The three main concerns are:
//   - Availability calculation (this file)
//   - Booking lifecycle management (booking.go)
//   - Conflict detection for timeslot changes (conflict.go)
package domain

import (
	"context"
	"time"

	"schedio/internal/store"
)

// Slot is a single available start time returned by ListAvailable.
type Slot struct {
	StartAt time.Time
	EndAt   time.Time
	UserID  string // staff member who owns the timeslot
}

// AvailabilityService calculates available booking slots for a given service
// on a given date by consulting the staff timeslot calendar and excluding
// already-booked windows.
type AvailabilityService struct {
	store store.DomainStore
}

// NewAvailabilityService constructs an AvailabilityService.
func NewAvailabilityService(st store.DomainStore) *AvailabilityService {
	return &AvailabilityService{store: st}
}

// ListAvailable returns all free start times for serviceID on the given UTC
// calendar date. It considers the service's duration and daily limit.
//
// Algorithm:
//  1. Load the service to get DurationMinutes and DailyLimit.
//  2. List all staff members.
//  3. For each staff member list their timeslots that overlap the given day.
//  4. For each timeslot enumerate candidate slots (aligned to 15-minute grid).
//  5. For each candidate call ListActiveBookingsInWindow to subtract booked windows.
//  6. Apply daily-limit: count confirmed+reserved bookings for the day against limit.
func (svc *AvailabilityService) ListAvailable(ctx context.Context, serviceID string, date time.Time) ([]Slot, error) {
	service, err := svc.store.GetService(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	staff, err := svc.store.ListStaff(ctx)
	if err != nil {
		return nil, err
	}

	duration := time.Duration(service.DurationMinutes) * time.Minute
	dayStart := truncateToDay(date)
	dayEnd := dayStart.Add(24 * time.Hour)

	// Collect bookings for the day once for the daily-limit check.
	dayBookings, err := svc.store.ListBookingsForDay(ctx, dayStart)
	if err != nil {
		return nil, err
	}
	activeToday := countActiveBookings(dayBookings, serviceID)

	var slots []Slot
	for _, member := range staff {
		timeslots, err := svc.store.ListTimeslots(ctx, member.ID, dayStart, dayEnd)
		if err != nil {
			return nil, err
		}
		for _, ts := range timeslots {
			// Enumerate grid-aligned candidate start times within this timeslot.
			candidates := enumerateCandidates(ts.StartAt, ts.EndAt, duration)
			for _, start := range candidates {
				end := start.Add(duration)
				busy, err := svc.store.ListActiveBookingsInWindow(ctx, member.ID, start, end)
				if err != nil {
					return nil, err
				}
				if len(busy) > 0 {
					continue // window already booked
				}
				if service.DailyLimit > 0 && activeToday >= service.DailyLimit {
					continue // daily limit reached
				}
				slots = append(slots, Slot{StartAt: start, EndAt: end, UserID: member.ID})
			}
		}
	}
	return slots, nil
}

// ListAvailableForDateRange calls ListAvailable for every calendar day in
// [rangeStart, rangeEnd) and returns the results keyed by ISO-8601 date
// string ("YYYY-MM-DD"). Days with no available slots are omitted.
func (svc *AvailabilityService) ListAvailableForDateRange(ctx context.Context, serviceID string, rangeStart, rangeEnd time.Time) (map[string][]Slot, error) {
	result := make(map[string][]Slot)
	for day := truncateToDay(rangeStart); day.Before(rangeEnd); day = day.Add(24 * time.Hour) {
		slots, err := svc.ListAvailable(ctx, serviceID, day)
		if err != nil {
			return nil, err
		}
		if len(slots) > 0 {
			key := day.UTC().Format("2006-01-02")
			result[key] = slots
		}
	}
	return result, nil
}

// enumerateCandidates returns all duration-aligned start times within
// [windowStart, windowEnd) such that start + duration ≤ windowEnd.
// Consecutive slots are spaced exactly one service-duration apart so they
// never overlap; the first slot is snapped up to the nearest duration boundary.
func enumerateCandidates(windowStart, windowEnd time.Time, duration time.Duration) []time.Time {
	// Step equals the service duration — no overlapping appointments.
	first := snapUp(windowStart, duration)
	var out []time.Time
	for t := first; !t.Add(duration).After(windowEnd); t = t.Add(duration) {
		out = append(out, t)
	}
	return out
}

// snapUp rounds t up to the nearest multiple of grid.
func snapUp(t time.Time, grid time.Duration) time.Time {
	unix := t.UnixNano()
	g := int64(grid)
	rem := unix % g
	if rem == 0 {
		return t
	}
	return t.Add(time.Duration(g - rem))
}

// truncateToDay returns the start of the UTC calendar day containing t.
func truncateToDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// countActiveBookings counts non-cancelled bookings for serviceID.
func countActiveBookings(bookings []*store.Booking, serviceID string) int {
	n := 0
	for _, b := range bookings {
		if b.ServiceID != serviceID {
			continue
		}
		if b.State == store.BookingStateCancelled || b.State == store.BookingStateNoShow {
			continue
		}
		n++
	}
	return n
}
