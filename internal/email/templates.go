package email

import (
	"context"
	"fmt"
	"time"

	"schedio/internal/store"
)

// ReservedData is the template data for the "reserved" email sent to the
// customer when a session is successfully submitted.
type ReservedData struct {
	Contact    *store.Contact
	Session    *store.BookingSession
	Bookings   []*store.Booking
	ManageLink string // signed management URL
	SentAt     time.Time
}

// SessionResultData is the template data for the "session-result" email sent
// when a staff member completes the review of an entire session.
type SessionResultData struct {
	Contact  *store.Contact
	Session  *store.BookingSession
	Bookings []*store.Booking // mix of confirmed and rejected
	SentAt   time.Time
}

// ChangeSummaryData is the template data for the "change-summary" email sent
// when the customer reschedules a booking.
type ChangeSummaryData struct {
	Contact    *store.Contact
	Booking    *store.Booking
	ManageLink string
	SentAt     time.Time
}

// CancellationData is the template data for the "cancellation" email.
type CancellationData struct {
	Contact *store.Contact
	Booking *store.Booking
	SentAt  time.Time
}

// AdminNotifyData is the template data for the admin-notify email sent when a
// customer submits a session.
type AdminNotifyData struct {
	Session   *store.BookingSession
	Contact   *store.Contact
	Bookings  []*store.Booking
	ReviewURL string
	SentAt    time.Time
}

// ConflictData is the template data for the admin-conflict email sent when an
// availability change affects active bookings.
type ConflictData struct {
	Availability *store.Availability
	Conflicts    []*store.Booking
	SentAt       time.Time
}

// RetentionNotifyData is the template data for the retention-notify email.
type RetentionNotifyData struct {
	Contact          *store.Contact
	ConfirmDeleteURL string // signed deletion confirmation URL
	ExpiresAt        time.Time
	SentAt           time.Time
}

// BillingInvoiceData is the template data for the billing-invoice email.
type BillingInvoiceData struct {
	Contact     *store.Contact
	Bookings    []*store.Booking
	InvoicePath string // path to the invoice file on disk
	SentAt      time.Time
}

// SendReserved sends the "reserved" email to the customer.
func (s *Sender) SendReserved(ctx context.Context, data ReservedData) error {
	subj, body, err := s.render("reserved", data)
	if err != nil {
		return err
	}
	return s.send(ctx, []string{data.Contact.Email}, subj, body)
}

// SendSessionResult sends the "session-result" email to the customer.
func (s *Sender) SendSessionResult(ctx context.Context, data SessionResultData) error {
	subj, body, err := s.render("session-result", data)
	if err != nil {
		return err
	}
	return s.send(ctx, []string{data.Contact.Email}, subj, body)
}

// SendChangeSummary sends the "change-summary" email to the customer.
func (s *Sender) SendChangeSummary(ctx context.Context, data ChangeSummaryData) error {
	subj, body, err := s.render("change-summary", data)
	if err != nil {
		return err
	}
	return s.send(ctx, []string{data.Contact.Email}, subj, body)
}

// SendCancellation sends the "cancellation" email to the customer.
func (s *Sender) SendCancellation(ctx context.Context, data CancellationData) error {
	subj, body, err := s.render("cancellation", data)
	if err != nil {
		return err
	}
	return s.send(ctx, []string{data.Contact.Email}, subj, body)
}

// SendAdminNotify sends the "admin-notify" email to the administrator.
func (s *Sender) SendAdminNotify(ctx context.Context, adminEmail string, data AdminNotifyData) error {
	subj, body, err := s.render("admin-notify", data)
	if err != nil {
		return err
	}
	return s.send(ctx, []string{adminEmail}, subj, body)
}

// SendAdminConflict sends the "admin-conflict" email to the administrator.
func (s *Sender) SendAdminConflict(ctx context.Context, adminEmail string, data ConflictData) error {
	subj, body, err := s.render("admin-conflict", data)
	if err != nil {
		return err
	}
	return s.send(ctx, []string{adminEmail}, subj, body)
}

// SendRetentionNotify sends the "retention-notify" email to all Staff users.
func (s *Sender) SendRetentionNotify(ctx context.Context, staffEmails []string, data RetentionNotifyData) error {
	subj, body, err := s.render("retention-notify", data)
	if err != nil {
		return err
	}
	if len(staffEmails) == 0 {
		return fmt.Errorf("email: no staff emails provided for retention-notify")
	}
	return s.send(ctx, staffEmails, subj, body)
}

// SendBillingInvoice sends the "billing-invoice" email to all Staff users.
func (s *Sender) SendBillingInvoice(ctx context.Context, staffEmails []string, data BillingInvoiceData) error {
	subj, body, err := s.render("billing-invoice", data)
	if err != nil {
		return err
	}
	if len(staffEmails) == 0 {
		return fmt.Errorf("email: no staff emails provided for billing-invoice")
	}
	return s.send(ctx, staffEmails, subj, body)
}
