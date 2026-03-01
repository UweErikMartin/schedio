package domain

import (
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"time"

	"schedio/internal/store"
)

// BookingService handles the booking lifecycle: creating, confirming,
// rejecting, cancelling, and marking bookings as no-show.
type BookingService struct {
	store store.DomainStore
}

// NewBookingService constructs a BookingService.
func NewBookingService(st store.DomainStore) *BookingService {
	return &BookingService{store: st}
}

// CreateBooking creates a new Booking under sessionID for the given time slot
// and staff user. It atomically checks for overlap via DomainStore.CreateBooking
// and returns store.ErrConflict when the slot is already taken.
func (svc *BookingService) CreateBooking(ctx context.Context, sessionID, serviceID, contactID, userID string, startAt, endAt time.Time) (*store.Booking, error) {
	session, err := svc.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("booking: get session: %w", err)
	}
	if session.State != store.SessionStateOpen {
		return nil, fmt.Errorf("booking: session %q is not open (state=%s)", sessionID, session.State)
	}

	b := &store.Booking{
		ID:        newID(),
		SessionID: sessionID,
		ServiceID: serviceID,
		ContactID: contactID,
		UserID:    userID,
		StartAt:   startAt.UTC(),
		EndAt:     endAt.UTC(),
		State:     store.BookingStateReserved,
	}
	if err := svc.store.CreateBooking(ctx, b); err != nil {
		return nil, err // ErrConflict propagates as-is
	}
	return b, nil
}

// ConfirmBooking transitions a Booking from Reserved to Confirmed. It
// increments Sequence and persists the update via DomainStore.UpdateBooking.
func (svc *BookingService) ConfirmBooking(ctx context.Context, bookingID string) (*store.Booking, error) {
	b, err := svc.store.GetBooking(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if b.State != store.BookingStateReserved {
		return nil, fmt.Errorf("booking: cannot confirm booking %q in state %s", bookingID, b.State)
	}
	b.State = store.BookingStateConfirmed
	b.Sequence++
	if err := svc.store.UpdateBooking(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// RejectBooking transitions a Booking from Reserved to Cancelled with reason "admin".
func (svc *BookingService) RejectBooking(ctx context.Context, bookingID string) (*store.Booking, error) {
	return svc.cancelBooking(ctx, bookingID, store.BookingStateReserved, store.CancelReasonAdmin)
}

// CancelBookingByCustomer cancels a Booking. If the booking's start time is
// within the no-show deadline the cancel reason is set to "noshow"; otherwise
// it is "customer".
func (svc *BookingService) CancelBookingByCustomer(ctx context.Context, bookingID string, noShowDeadlineHours int) (*store.Booking, error) {
	b, err := svc.store.GetBooking(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if b.State != store.BookingStateReserved && b.State != store.BookingStateConfirmed {
		return nil, fmt.Errorf("booking: cannot cancel booking %q in state %s", bookingID, b.State)
	}
	reason := store.CancelReasonCustomer
	deadline := time.Now().UTC().Add(time.Duration(noShowDeadlineHours) * time.Hour)
	if b.StartAt.Before(deadline) {
		reason = store.CancelReasonNoShow
	}
	b.State = store.BookingStateCancelled
	b.CancelReason = reason
	b.Sequence++
	if err := svc.store.UpdateBooking(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// MarkNoShow marks a booking as no-show (set manually by a staff member).
func (svc *BookingService) MarkNoShow(ctx context.Context, bookingID string) (*store.Booking, error) {
	b, err := svc.store.GetBooking(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if b.State != store.BookingStateConfirmed && b.State != store.BookingStateReserved {
		return nil, fmt.Errorf("booking: cannot mark no-show for booking %q in state %s", bookingID, b.State)
	}
	b.State = store.BookingStateNoShow
	b.CancelReason = store.CancelReasonNoShow
	b.Sequence++
	if err := svc.store.UpdateBooking(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// cancelBooking is the shared helper for cancellations originating from either
// fromState (Reserved or Confirmed).
func (svc *BookingService) cancelBooking(ctx context.Context, bookingID string, fromState store.BookingState, reason store.CancelReason) (*store.Booking, error) {
	b, err := svc.store.GetBooking(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if b.State != fromState {
		return nil, fmt.Errorf("booking: cannot cancel booking %q in state %s (expected %s)", bookingID, b.State, fromState)
	}
	b.State = store.BookingStateCancelled
	b.CancelReason = reason
	b.Sequence++
	if err := svc.store.UpdateBooking(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// newID returns a random UUID v4 string.
func newID() string {
	b := make([]byte, 16)
	if _, err := cryptorand.Read(b); err != nil {
		// Fallback (practically never reached): use timestamp.
		return fmt.Sprintf("%016x-0000-4000-8000-000000000000", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
