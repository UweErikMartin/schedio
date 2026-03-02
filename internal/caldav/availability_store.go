package caldav

// availability_store.go provides a CalendarStore implementation that merges
// the regular CalDAV calendars (backed by the base CalendarStore) with virtual
// per-staff availability calendars (backed by the DomainStore timeslot layer).
//
// Availability calendar IDs use the "avail-<userID>" prefix. The access rules
// are:
//
//   - Staff users: only their own "avail-<userID>" calendar is visible and
//     writable. Timeslots stored there represent their availability windows.
//   - Administrator users: all staff availability calendars are visible and
//     writable (read-only access is intentionally not distinguished so admins
//     may also correct timeslots on behalf of staff).

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	calstore "schedio/internal/store"

	"k8s.io/klog/v2"
)

const availCalPrefix = "avail-"

// availCalendarID returns the CalDAV calendar ID for a given staff user's
// availability calendar.
func availCalendarID(userID string) string {
	return availCalPrefix + userID
}

// availUserIDFromCalID extracts the staff user ID embedded in an availability
// calendar ID. Returns "", false when id does not start with the availability
// prefix.
func availUserIDFromCalID(id string) (string, bool) {
	if strings.HasPrefix(id, availCalPrefix) {
		return strings.TrimPrefix(id, availCalPrefix), true
	}
	return "", false
}

// combinedCalendarStore implements calstore.CalendarStore by merging the
// regular CalDAV calendars from base with virtual availability calendars
// synthesised from the DomainStore's timeslot data.
type combinedCalendarStore struct {
	base   calstore.CalendarStore
	domain calstore.DomainStore
}

// availCalendar builds the store.Calendar descriptor for a staff user's
// availability calendar.
func availCalendar(u *calstore.User) *calstore.Calendar {
	name := u.Name
	if name == "" {
		name = u.Email
	}
	return &calstore.Calendar{
		ID:          availCalendarID(u.ID),
		Name:        "Availability – " + name,
		Description: "Availability windows for " + u.Email,
		Color:       "#4CAF50",
		Timezone:    "UTC",
	}
}

// canAccessAvailCal reports whether the context principal may access the
// availability calendar owned by ownerUserID.
//
//   - Administrators have access to all staff availability calendars.
//   - Staff users have access only to their own.
//   - Unauthenticated principals have no access.
func (s *combinedCalendarStore) canAccessAvailCal(ctx context.Context, ownerUserID string) bool {
	u := principalFromContext(ctx)
	if u == nil {
		klog.V(2).Infof("caldav/avail: canAccessAvailCal ownerID=%q → false (no principal)", ownerUserID)
		return false
	}
	var result bool
	if u.Role == calstore.UserRoleAdministrator {
		result = true
	} else {
		result = u.ID == ownerUserID
	}
	klog.V(2).Infof("caldav/avail: canAccessAvailCal ownerID=%q principal=%q role=%v → %v", ownerUserID, u.Email, u.Role, result)
	return result
}

// viewableAvailCalendars returns the availability calendars that the context
// principal is allowed to see.
func (s *combinedCalendarStore) viewableAvailCalendars(ctx context.Context) ([]*calstore.Calendar, error) {
	u := principalFromContext(ctx)
	if u == nil {
		klog.V(1).Infof("caldav/avail: viewableAvailCalendars: no principal in context → 0 avail calendars")
		return nil, nil
	}
	klog.V(1).Infof("caldav/avail: viewableAvailCalendars: principal=%q role=%v id=%q", u.Email, u.Role, u.ID)
	if u.Role == calstore.UserRoleAdministrator {
		staff, err := s.domain.ListStaff(ctx)
		if err != nil {
			return nil, err
		}
		cals := make([]*calstore.Calendar, 0, len(staff))
		for _, su := range staff {
			cals = append(cals, availCalendar(su))
		}
		klog.V(1).Infof("caldav/avail: viewableAvailCalendars: admin → %d avail calendar(s) for %d staff", len(cals), len(staff))
		return cals, nil
	}
	// Staff: only their own availability calendar.
	klog.V(1).Infof("caldav/avail: viewableAvailCalendars: staff → 1 avail calendar (own)")
	return []*calstore.Calendar{availCalendar(u)}, nil
}

// timeslotToEvent converts a Timeslot into a synthetic Event for the CalDAV
// layer. The CalDAV UID is used as the event ID. The RRule is preserved so
// that eventToObject can emit a proper RRULE: property in the iCal output.
func timeslotToEvent(calID string, t *calstore.Timeslot) *calstore.Event {
	return &calstore.Event{
		ID:           t.CalDAVUID,
		CalendarID:   calID,
		Summary:      "Available",
		Start:        t.StartAt,
		End:          t.EndAt,
		Opacity:      calstore.OpacityOpaque,
		Status:       calstore.StatusConfirmed,
		Created:      t.CreatedAt,
		Modified:     t.UpdatedAt,
		ETag:         t.CalDAVETag,
		RRule:        t.RRule,
		RecurrenceID: t.RecurrenceID,
	}
}

// eventToTimeslot converts an Event received via CalDAV PUT into a Timeslot
// suitable for persistence via DomainStore.UpsertTimeslot.
func eventToTimeslot(ownerUserID string, e *calstore.Event) *calstore.Timeslot {
	return &calstore.Timeslot{
		UserID:       ownerUserID,
		CalDAVUID:    e.ID,
		StartAt:      e.Start,
		EndAt:        e.End,
		RRule:        e.RRule,
		RecurrenceID: e.RecurrenceID,
	}
}

// ── calstore.CalendarStore implementation ─────────────────────────────────────

// GetCalendar implements CalendarStore. Availability calendars are resolved
// from the DomainStore; all other IDs are delegated to the base CalendarStore.
func (s *combinedCalendarStore) GetCalendar(ctx context.Context, id string) (*calstore.Calendar, error) {
	if ownerID, ok := availUserIDFromCalID(id); ok {
		if !s.canAccessAvailCal(ctx, ownerID) {
			return nil, calstore.ErrForbidden
		}
		su, err := s.domain.GetStaff(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		return availCalendar(su), nil
	}
	return s.base.GetCalendar(ctx, id)
}

// ListCalendars implements CalendarStore. The returned list is the union of
// base CalDAV calendars and the availability calendars visible to the context
// principal.
func (s *combinedCalendarStore) ListCalendars(ctx context.Context) ([]*calstore.Calendar, error) {
	base, err := s.base.ListCalendars(ctx)
	if err != nil {
		return nil, err
	}
	avail, err := s.viewableAvailCalendars(ctx)
	if err != nil {
		return nil, err
	}
	all := append(base, avail...)
	if klog.V(1).Enabled() {
		klog.Infof("caldav/avail: ListCalendars: base=%d avail=%d total=%d", len(base), len(avail), len(all))
		for _, c := range all {
			klog.V(2).Infof("  calendar id=%q name=%q", c.ID, c.Name)
		}
	}
	return all, nil
}

// GetEvent implements CalendarStore. Availability events are synthesised from
// the Timeslot with the matching CalDAV UID; others are delegated.
func (s *combinedCalendarStore) GetEvent(ctx context.Context, calendarID, eventID string) (*calstore.Event, error) {
	if ownerID, ok := availUserIDFromCalID(calendarID); ok {
		if !s.canAccessAvailCal(ctx, ownerID) {
			return nil, calstore.ErrForbidden
		}
		ts, err := s.domain.GetTimeslot(ctx, ownerID, eventID)
		if err != nil {
			return nil, err
		}
		return timeslotToEvent(calendarID, ts), nil
	}
	return s.base.GetEvent(ctx, calendarID, eventID)
}

// ListEvents implements CalendarStore. Availability events are produced by
// converting raw (unexpanded) Timeslots into Events that carry the RRULE.
// Returning the RRULE VEVENT lets the CalDAV client (e.g. iOS Calendar)
// expand recurring events itself, which is the correct RFC 4791 behaviour and
// avoids sending thousands of individual occurrence VEVENTs over the wire.
func (s *combinedCalendarStore) ListEvents(ctx context.Context, calendarID string, _ /*start*/, _ /*end*/ time.Time) ([]*calstore.Event, error) {
	if ownerID, ok := availUserIDFromCalID(calendarID); ok {
		if !s.canAccessAvailCal(ctx, ownerID) {
			return nil, calstore.ErrForbidden
		}
		// Use raw (unexpanded) timeslots so that each recurring series is
		// represented by a single VEVENT with RRULE rather than one VEVENT
		// per expanded occurrence.
		tss, err := s.domain.ListRawTimeslots(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		events := make([]*calstore.Event, 0, len(tss))
		for _, ts := range tss {
			events = append(events, timeslotToEvent(calendarID, ts))
		}
		return events, nil
	}
	return s.base.ListEvents(ctx, calendarID, time.Time{}, time.Time{})
}

// PutEvent implements CalendarStore. Writes to an availability calendar are
// persisted as Timeslots; others are delegated.
func (s *combinedCalendarStore) PutEvent(ctx context.Context, event *calstore.Event) error {
	if ownerID, ok := availUserIDFromCalID(event.CalendarID); ok {
		if !s.canAccessAvailCal(ctx, ownerID) {
			return calstore.ErrForbidden
		}
		return s.domain.UpsertTimeslot(ctx, eventToTimeslot(ownerID, event))
	}
	return s.base.PutEvent(ctx, event)
}

// DeleteEvent implements CalendarStore. Deletes from an availability calendar
// remove the matching Timeslot; others are delegated.
func (s *combinedCalendarStore) DeleteEvent(ctx context.Context, calendarID, eventID, etag string) error {
	if ownerID, ok := availUserIDFromCalID(calendarID); ok {
		if !s.canAccessAvailCal(ctx, ownerID) {
			return calstore.ErrForbidden
		}
		return s.domain.DeleteTimeslot(ctx, ownerID, eventID)
	}
	return s.base.DeleteEvent(ctx, calendarID, eventID, etag)
}

// CTag implements CalendarStore. For availability calendars the CTag is a
// truncated SHA-256 hash of all raw timeslot UIDs and ETags; others are delegated.
func (s *combinedCalendarStore) CTag(ctx context.Context, calendarID string) (string, error) {
	if ownerID, ok := availUserIDFromCalID(calendarID); ok {
		if !s.canAccessAvailCal(ctx, ownerID) {
			return "", calstore.ErrForbidden
		}
		tss, err := s.domain.ListRawTimeslots(ctx, ownerID)
		if err != nil {
			return "", err
		}
		h := sha256.New()
		for _, ts := range tss {
			fmt.Fprintf(h, "%s:%s\n", ts.CalDAVUID, ts.CalDAVETag)
		}
		return fmt.Sprintf("%x", h.Sum(nil))[:16], nil
	}
	return s.base.CTag(ctx, calendarID)
}
