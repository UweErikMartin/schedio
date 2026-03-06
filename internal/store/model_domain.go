package store

import "time"

// ── Domain model ──────────────────────────────────────────────────────────────
//
// This file contains all domain-level types. CalDAV-layer types
// (Calendar, Event, Attendee, …) live in model.go.

// UserRole distinguishes the two roles a User may hold in the system.
type UserRole string

const (
	// UserRoleStaff identifies a user who manages availability windows and reviews bookings.
	UserRoleStaff UserRole = "staff"
	// UserRoleAdministrator identifies a user who manages services, settings,
	// and other administrators.
	UserRoleAdministrator UserRole = "administrator"
)

// User represents an admin or staff account. Staff users (Role == UserRoleStaff)
// are the foreign-key target for Availability and Booking records.
type User struct {
	ID                string
	Email             string // unique; also the CalDAV principal name for staff
	PasswordHash      string // bcrypt hash (cost ≥ 12)
	Role              UserRole
	AppleOAuthEnabled bool
	AppleSubject      string // Apple id_token `sub` claim; empty when AppleOAuthEnabled is false
	Name              string // display name
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Staff is a type alias for User used when the DomainStore guarantees that the
// returned user has Role == UserRoleStaff.
type Staff = User

// Service is a bookable service offered by the business. It is managed by
// administrators and referenced by BookingSessions and Bookings.
type Service struct {
	ID              string
	Name            string
	Summary         string  // short one-line label for the selection list
	Description     string  // full-length detail text
	Price           float64 // in the configured currency (see Settings.Currency)
	DurationMinutes int
	DailyLimit      int // 0 = unlimited
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Availability is an availability window managed by a staff user via CalDAV.
// A recurring event is represented by a single Availability with a non-empty RRule.
// Individual overrides of a recurring series carry a non-zero RecurrenceID.
// The composite key (UserID, CalDAVUID, RecurrenceID) uniquely identifies each
// record; series roots have a zero RecurrenceID.
type Availability struct {
	ID           string
	UserID       string    // FK → User (must be role = "staff")
	CalDAVUID    string    // iCal UID; globally unique across all availability records
	CalDAVETag   string    // opaque version token for conditional requests
	StartAt      time.Time // always stored in UTC
	EndAt        time.Time // always stored in UTC
	RRule        string    // iCal RRULE value; empty for single events
	RecurrenceID time.Time // non-zero for overrides of a recurring series
	// ExDates lists occurrence start-times (UTC) excluded from the series
	// (iCal EXDATE). Non-empty only on the series root; zero on overrides.
	ExDates   []time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Contact holds a customer's personal details. A single Contact may be
// referenced by multiple BookingSessions and Bookings across different visits.
type Contact struct {
	ID                   string
	FirstName            string
	LastName             string
	Email                string // unique; used as customer identity key
	Phone                string
	CreatedAt            time.Time
	LastAppointmentEndAt time.Time // zero when no completed appointment exists
	RetentionState       RetentionState
	RetentionNotifiedAt  time.Time // zero when no notification has been sent
	BillingGenerated     bool
}

// RetentionState models the GDPR data-retention lifecycle of a Contact.
type RetentionState string

const (
	// RetentionStateActive is the default state; contact has recent or future appointments.
	RetentionStateActive RetentionState = "active"
	// RetentionStateNotified means a retention-notification e-mail was sent; awaiting confirmation.
	RetentionStateNotified RetentionState = "notified"
	// RetentionStatePendingDeletion means the confirmation window has passed;
	// a staff member must manually confirm permanent deletion.
	RetentionStatePendingDeletion RetentionState = "pending_deletion"
)

// BookingSession groups all individual Bookings created during one customer
// interaction. It is always tied to exactly one Service and one Contact.
type BookingSession struct {
	ID          string
	ServiceID   string
	ContactID   string
	State       SessionState
	CreatedAt   time.Time
	SubmittedAt time.Time // zero until state transitions to "submitted"
}

// SessionState represents the lifecycle of a BookingSession.
type SessionState string

const (
	// SessionStateOpen is the initial state; the customer is selecting time slots.
	SessionStateOpen SessionState = "open"
	// SessionStateSubmitted means the customer has confirmed the session; awaiting staff review.
	SessionStateSubmitted SessionState = "submitted"
	// SessionStateClosed means the session has been reviewed and all bookings are in a terminal state.
	SessionStateClosed SessionState = "closed"
)

// Booking is one individual appointment within a BookingSession.
type Booking struct {
	ID           string
	SessionID    string
	ServiceID    string
	ContactID    string
	UserID       string // staff user who owns the booked availability window
	StartAt      time.Time
	EndAt        time.Time
	State        BookingState
	CancelReason CancelReason // non-empty only when State == BookingStateCancelled
	Sequence     int          // incremented on every update; maps to iCal SEQUENCE
	CalDAVUID    string       // iCal UID; populated once the CalDAV event is created
	CalDAVETag   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// BookingState models the booking state machine described in architecture §6.2.
type BookingState string

const (
	// BookingStateReserved is the initial state after a session is submitted (TENTATIVE in CalDAV).
	BookingStateReserved BookingState = "reserved"
	// BookingStateConfirmed means the booking was approved by a staff member (CONFIRMED in CalDAV).
	BookingStateConfirmed BookingState = "confirmed"
	// BookingStateCancelled means the booking was cancelled; see CancelReason for sub-state.
	BookingStateCancelled BookingState = "cancelled"
	// BookingStateNoShow means the customer did not appear; functionally equivalent to cancelled.
	BookingStateNoShow BookingState = "noshow"
)

// CancelReason distinguishes the three CANCELLED sub-states.
type CancelReason string

const (
	// CancelReasonCustomer means the customer cancelled within the allowed window.
	CancelReasonCustomer CancelReason = "customer"
	// CancelReasonAdmin means a staff member rejected or cancelled the booking.
	CancelReasonAdmin CancelReason = "admin"
	// CancelReasonNoShow means the customer cancelled after the deadline, or
	// was manually marked as no-show by a staff member.
	CancelReasonNoShow CancelReason = "noshow"
)

// Settings holds deployment-wide configuration. There is exactly one row
// (id = 1) in the persistent store.
type Settings struct {
	NoShowDeadlineHours  int    // hours before start after which cancellation = no-show
	RetentionPeriodDays  int    // days after last appointment before deletion workflow begins
	ReminderLeadTimeDays int    // calendar days before a confirmed appointment at which the reminder e-mail is sent
	Currency             string // ISO 4217, e.g. "EUR"
	AppointmentLocation  string // ICS LOCATION field
	TandCFilename        string // filename within DATA_DIR
	SenderName           string // display name used in the From: header of customer e-mails
	DefaultCalendarName  string // display name of the Booking-Calendar in CalDAV clients; falls back to "Booking-Calendar" when empty
	CalendarURL          string // hostname or URL of the CalDAV server advertised to CalDAV clients
}
