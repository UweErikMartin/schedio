package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a requested calendar or event does not exist.
var ErrNotFound = errors.New("caldav: not found")

// ErrConflict is returned when a conditional PUT or DELETE fails because the
// server-side ETag no longer matches the client's If-Match value, or when a
// booking overlaps with an existing one, or when a service that still has
// active bookings is deleted.
var ErrConflict = errors.New("caldav: conflict")

// ErrReadOnly is returned when a write operation is attempted against a
// read-only CalendarStore implementation.
var ErrReadOnly = errors.New("caldav: read-only")

// ErrForbidden is returned when the caller is authenticated but lacks the
// privilege required to perform the requested operation.
var ErrForbidden = errors.New("store: forbidden")

// Backend is the composite store interface that every persistence backend must
// implement. It combines CalendarStore (CalDAV-level primitives) with
// DomainStore (domain-level business data). The memory backend and the future
// PostgreSQL backend both satisfy Backend.
type Backend interface {
	CalendarStore
	DomainStore
}

// CalendarStore is the interface the CalDAV facade uses to read and write
// calendars and events. Implement this interface to back the facade with any
// persistence layer (database, external API, …).
type CalendarStore interface {
	// Calendar operations

	// GetCalendar returns the calendar with the given ID.
	GetCalendar(ctx context.Context, id string) (*Calendar, error)

	// ListCalendars returns all calendars visible to the current principal.
	ListCalendars(ctx context.Context) ([]*Calendar, error)

	// Event operations

	// GetEvent returns a single event. Both calendarID and eventID must match.
	GetEvent(ctx context.Context, calendarID, eventID string) (*Event, error)

	// ListEvents returns events whose time range overlaps [start, end].
	// A zero start or end means no lower / upper bound respectively.
	ListEvents(ctx context.Context, calendarID string, start, end time.Time) ([]*Event, error)

	// PutEvent creates or updates an event.
	// When Event.ETag is non-empty the store MUST perform an optimistic-lock
	// check and return ErrConflict if the stored ETag differs.
	// The store is responsible for updating Event.ETag, Event.Modified and
	// Event.Sequence before persisting.
	PutEvent(ctx context.Context, event *Event) error

	// DeleteEvent removes an event. etag is the client's If-Match value;
	// pass an empty string to skip the check.
	DeleteEvent(ctx context.Context, calendarID, eventID, etag string) error

	// Sync tokens

	// CTag returns an opaque string that changes whenever any event in the
	// calendar is created, modified or deleted. Clients use it for quick
	// change-detection (draft-desruisseaux-caldav-sched-04).
	CTag(ctx context.Context, calendarID string) (string, error)
}

// DomainStore covers all domain-level persistence required by the business
// logic and REST handlers. The PostgreSQL backend implements both CalendarStore
// and DomainStore; the memory backend also implements both (domain operations
// are backed by in-process maps protected by a sync.RWMutex).
type DomainStore interface {
	// ── Staff / Users ─────────────────────────────────────────────────────

	// SyncUsers replaces the stored user set with the provided list. It is
	// called at startup to synchronise the USERS_CONFIG_FILE into the store.
	// For any user whose email already exists, the stored password hash is
	// preserved when the incoming PasswordHash field is empty.
	SyncUsers(ctx context.Context, users []*User) error

	// ListStaff returns all users with role "staff".
	ListStaff(ctx context.Context) ([]*Staff, error)

	// GetStaff returns the staff user with the given ID. Returns ErrNotFound
	// when the ID does not exist or belongs to a non-staff user.
	GetStaff(ctx context.Context, id string) (*Staff, error)

	// GetUserByEmail returns the user with the given email address regardless
	// of role. Returns ErrNotFound when no matching user exists.
	GetUserByEmail(ctx context.Context, email string) (*User, error)

	// ── Services ──────────────────────────────────────────────────────────

	// ListServices returns all services ordered by name.
	ListServices(ctx context.Context) ([]*Service, error)

	// GetService returns the service with the given ID.
	GetService(ctx context.Context, id string) (*Service, error)

	// CreateService persists a new Service. The caller is responsible for
	// setting a unique ID (UUID) before calling.
	CreateService(ctx context.Context, s *Service) error

	// UpdateService replaces the stored Service with the provided value.
	UpdateService(ctx context.Context, s *Service) error

	// DeleteService removes a service. Returns ErrConflict when there are
	// active (non-cancelled) bookings that reference the service.
	DeleteService(ctx context.Context, id string) error

	// ── Timeslots (availability windows) ──────────────────────────────────

	// ListTimeslots returns all Timeslots for the given staff user whose time
	// window overlaps [start, end]. Zero start/end means no bound.
	// Recurring timeslots are expanded into individual occurrences within the
	// requested range – use this for availability / booking decisions.
	ListTimeslots(ctx context.Context, userID string, start, end time.Time) ([]*Timeslot, error)

	// ListRawTimeslots returns the raw, unexpanded Timeslot records for
	// userID – one entry per CalDAVUID with the RRule field intact.
	// Unlike ListTimeslots, recurring timeslots are NOT expanded.
	// This is intended for the CalDAV layer, which sends RRULE VEVENTs to
	// calendar clients and lets the client expand them.
	ListRawTimeslots(ctx context.Context, userID string) ([]*Timeslot, error)

	// GetTimeslot returns the Timeslot identified by its CalDAV UID. Returns
	// ErrNotFound when the UID does not exist or belongs to a different user.
	GetTimeslot(ctx context.Context, userID, uid string) (*Timeslot, error)

	// UpsertTimeslot creates or replaces the Timeslot identified by its
	// CalDAVUID. If a record with the same CalDAVUID exists, it is updated.
	UpsertTimeslot(ctx context.Context, t *Timeslot) error

	// DeleteTimeslot removes the Timeslot identified by its CalDAV UID.
	// For a series root (RecurrenceID == zero) this removes only the root
	// record; call DeleteTimeslotOverrides separately to cascade to overrides.
	// Returns ErrNotFound when the UID does not exist or belongs to a
	// different user.
	DeleteTimeslot(ctx context.Context, userID, uid string) error

	// DeleteTimeslotOverride removes the single override record identified by
	// (userID, uid, recurrenceID). Returns ErrNotFound when no matching
	// override exists.
	DeleteTimeslotOverride(ctx context.Context, userID, uid string, recurrenceID time.Time) error

	// DeleteTimeslotOverrides removes all override records whose CalDAVUID
	// equals uid and whose RecurrenceID is non-zero. This is the cascade
	// step used when deleting a recurring series root (CDV-TS-2 Case B).
	// Returns nil (not ErrNotFound) when no overrides exist.
	DeleteTimeslotOverrides(ctx context.Context, userID, uid string) error

	// ── Contacts ─────────────────────────────────────────────────────────

	// GetOrCreateContact looks up a Contact by email. If none exists, a new
	// Contact is created from the provided template (all fields except ID and
	// timestamps are taken from c). The returned Contact is always non-nil on
	// success.
	GetOrCreateContact(ctx context.Context, email string, c *Contact) (*Contact, error)

	// GetContact returns the Contact with the given ID.
	GetContact(ctx context.Context, id string) (*Contact, error)

	// UpdateContactLastAppointment advances LastAppointmentEndAt to
	// appointmentEndAt when it is later than the currently stored value, and
	// resets BillingGenerated to false and RetentionState to "active" in that
	// case. It is a no-op when appointmentEndAt is not later than the stored
	// value.
	UpdateContactLastAppointment(ctx context.Context, contactID string, appointmentEndAt time.Time) error

	// ── Booking sessions ──────────────────────────────────────────────────

	// CreateSession persists a new BookingSession. The caller is responsible
	// for setting a unique ID (UUID) before calling.
	CreateSession(ctx context.Context, s *BookingSession) error

	// GetSession returns the BookingSession with the given ID.
	GetSession(ctx context.Context, id string) (*BookingSession, error)

	// UpdateSession replaces the stored BookingSession with the provided value.
	UpdateSession(ctx context.Context, s *BookingSession) error

	// ListPendingSessions returns all sessions in state "submitted" (reserved),
	// ordered by SubmittedAt ascending (oldest first).
	ListPendingSessions(ctx context.Context) ([]*BookingSession, error)

	// ── Bookings ──────────────────────────────────────────────────────────

	// CreateBooking atomically checks for time-overlap and inserts the booking.
	// Returns ErrConflict when the start–end window is already occupied by an
	// active booking.
	CreateBooking(ctx context.Context, b *Booking) error

	// GetBooking returns the Booking with the given ID.
	GetBooking(ctx context.Context, id string) (*Booking, error)

	// UpdateBooking replaces the stored Booking with the provided value.
	UpdateBooking(ctx context.Context, b *Booking) error

	// ListBookingsForSession returns all Bookings belonging to sessionID,
	// ordered by StartAt ascending.
	ListBookingsForSession(ctx context.Context, sessionID string) ([]*Booking, error)

	// ListBookingsForDay returns all Bookings whose StartAt falls on the
	// calendar day of date (UTC), ordered by StartAt ascending.
	ListBookingsForDay(ctx context.Context, date time.Time) ([]*Booking, error)

	// ListActiveBookingsInWindow returns all non-cancelled Bookings for the
	// given staff user whose time window overlaps [start, end].
	ListActiveBookingsInWindow(ctx context.Context, userID string, start, end time.Time) ([]*Booking, error)

	// ListAllBookingsInWindow returns all non-cancelled Bookings across all
	// staff users whose time window overlaps [start, end].
	// Zero start/end means no bound.
	// This is used by the CalDAV layer to synthesise events in the default
	// calendar so that confirmed and reserved bookings appear to CalDAV clients.
	ListAllBookingsInWindow(ctx context.Context, start, end time.Time) ([]*Booking, error)

	// ListBookingsForContact returns all non-cancelled Bookings for contactID,
	// ordered by StartAt ascending. Used for billing.
	ListBookingsForContact(ctx context.Context, contactID string) ([]*Booking, error)

	// ── Settings ──────────────────────────────────────────────────────────

	// GetSettings returns the single Settings row.
	GetSettings(ctx context.Context) (*Settings, error)

	// UpdateSettings replaces the stored Settings.
	UpdateSettings(ctx context.Context, s *Settings) error

	// GetHMACSecret returns the raw HMAC secret bytes.
	GetHMACSecret(ctx context.Context) ([]byte, error)

	// SetHMACSecret replaces the stored HMAC secret.
	SetHMACSecret(ctx context.Context, secret []byte) error

	// ── Data retention ────────────────────────────────────────────────────

	// ListRetentionDue returns Contacts whose LastAppointmentEndAt plus the
	// retention period has passed and whose RetentionState is "active".
	ListRetentionDue(ctx context.Context, retentionPeriod time.Duration) ([]*Contact, error)

	// MarkRetentionNotified sets RetentionState to "notified" and records the
	// notification timestamp for the given Contact.
	MarkRetentionNotified(ctx context.Context, contactID string) error

	// ListConfirmationExpired returns Contacts whose RetentionState is
	// "notified" and whose RetentionNotifiedAt is more than 7 days ago.
	ListConfirmationExpired(ctx context.Context) ([]*Contact, error)

	// AddToPendingDeletion sets RetentionState to "pending_deletion" for the
	// given Contact.
	AddToPendingDeletion(ctx context.Context, contactID string) error

	// ListPendingDeletion returns all Contacts with RetentionState
	// "pending_deletion".
	ListPendingDeletion(ctx context.Context) ([]*Contact, error)

	// DeleteContact permanently removes the Contact and all its associated
	// BookingSession and Booking rows.
	DeleteContact(ctx context.Context, contactID string) error

	// ── Billing ───────────────────────────────────────────────────────────

	// ListBillingDue returns Contacts whose LastAppointmentEndAt is in the past
	// and BillingGenerated is false.
	ListBillingDue(ctx context.Context) ([]*Contact, error)

	// MarkBillingGenerated sets BillingGenerated to true for the given Contact.
	MarkBillingGenerated(ctx context.Context, contactID string) error
}
