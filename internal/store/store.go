package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a requested calendar or event does not exist.
var ErrNotFound = errors.New("caldav: not found")

// ErrConflict is returned when a conditional PUT or DELETE fails because the
// server-side ETag no longer matches the client's If-Match value.
var ErrConflict = errors.New("caldav: conflict")

// ErrReadOnly is returned when a write operation is attempted against a
// read-only CalendarStore implementation.
var ErrReadOnly = errors.New("caldav: read-only")

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
