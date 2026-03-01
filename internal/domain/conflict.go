package domain

import (
	"context"
	"fmt"

	"schedio/internal/store"
)

// Conflict describes a booking that is affected by a timeslot change.
type Conflict struct {
	Booking  *store.Booking
	Timeslot *store.Timeslot
}

// ConflictDetector checks whether a timeslot modification or deletion affects
// any active bookings.
type ConflictDetector struct {
	store store.DomainStore
}

// NewConflictDetector constructs a ConflictDetector.
func NewConflictDetector(st store.DomainStore) *ConflictDetector {
	return &ConflictDetector{store: st}
}

// FindConflicts returns all active bookings that overlap the given timeslot's
// window. Used before a timeslot upsert or delete to detect bookings that
// would become orphaned.
func (cd *ConflictDetector) FindConflicts(ctx context.Context, ts *store.Timeslot) ([]Conflict, error) {
	bookings, err := cd.store.ListActiveBookingsInWindow(ctx, ts.UserID, ts.StartAt, ts.EndAt)
	if err != nil {
		return nil, fmt.Errorf("conflict: list bookings in window: %w", err)
	}
	conflicts := make([]Conflict, 0, len(bookings))
	for _, b := range bookings {
		cp := *ts
		conflicts = append(conflicts, Conflict{Booking: b, Timeslot: &cp})
	}
	return conflicts, nil
}
