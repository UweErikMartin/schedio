package domain

import (
	"context"
	"fmt"

	"schedio/internal/store"
)

// Conflict describes a booking that is affected by an availability change.
type Conflict struct {
	Booking      *store.Booking
	Availability *store.Availability
}

// ConflictDetector checks whether an availability modification or deletion affects
// any active bookings.
type ConflictDetector struct {
	store store.DomainStore
}

// NewConflictDetector constructs a ConflictDetector.
func NewConflictDetector(st store.DomainStore) *ConflictDetector {
	return &ConflictDetector{store: st}
}

// FindConflicts returns all active bookings that overlap the given availability record's
// window. Used before an availability upsert or delete to detect bookings that
// would become orphaned.
func (cd *ConflictDetector) FindConflicts(ctx context.Context, ts *store.Availability) ([]Conflict, error) {
	bookings, err := cd.store.ListActiveBookingsInWindow(ctx, ts.UserID, ts.StartAt, ts.EndAt)
	if err != nil {
		return nil, fmt.Errorf("conflict: list bookings in window: %w", err)
	}
	conflicts := make([]Conflict, 0, len(bookings))
	for _, b := range bookings {
		cp := *ts
		conflicts = append(conflicts, Conflict{Booking: b, Availability: &cp})
	}
	return conflicts, nil
}
