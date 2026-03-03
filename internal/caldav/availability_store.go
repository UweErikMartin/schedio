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
	"errors"
	"fmt"
	"strings"
	"time"

	rrulego "github.com/teambition/rrule-go"
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
// layer. The CalDAV UID is used as the event ID. The RRule and ExDates are
// preserved so that eventToObject can emit proper RRULE:/EXDATE: properties
// in the iCal output.
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
		ExDates:      append([]time.Time(nil), t.ExDates...),
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
		ExDates:      append([]time.Time(nil), e.ExDates...),
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
// persisted as Timeslots via DomainStore; others are delegated to the base store.
//
// The method implements CDV-TS-1: when the incoming event is an override
// (RecurrenceID != zero) it is upserted directly. For series roots and single
// events the method diffs the EXDATE sets against the previously stored
// record and takes corrective action:
//
//   - Newly added EXDATE: any orphaned override for that occurrence is deleted.
//   - Newly removed EXDATE (re-enabled occurrence): any orphaned override is
//     deleted so the restored occurrence takes effect immediately.
//
// The function does NOT perform a broad availability conflict check; that is
// delegated to the booking layer (CreateBooking rejects overlapping bookings).
func (s *combinedCalendarStore) PutEvent(ctx context.Context, event *calstore.Event) error {
	if ownerID, ok := availUserIDFromCalID(event.CalendarID); ok {
		if !s.canAccessAvailCal(ctx, ownerID) {
			return calstore.ErrForbidden
		}
		// CDV-TS-1: Override instances.
		if !event.RecurrenceID.IsZero() {
			klog.V(2).Infof("caldav/avail: PutEvent uid=%q recurrenceID=%v → upserting override", event.ID, event.RecurrenceID)
			// CDV-TS-1c: A CANCELLED override represents deletion of that
			// occurrence (iOS may send this form in addition to – or instead of –
			// adding an EXDATE to the series root VEVENT). Convert it to an EXDATE
			// on the series root so ListTimeslots correctly excludes the occurrence
			// regardless of which form the client used.
			if event.Status == calstore.StatusCancelled {
				klog.V(2).Infof("caldav/avail: PutEvent uid=%q recurrenceID=%v STATUS:CANCELLED → converting to EXDATE", event.ID, event.RecurrenceID)
				return s.addExDateToSeriesRoot(ctx, ownerID, event.ID, event.RecurrenceID)
			}
			return s.domain.UpsertTimeslot(ctx, eventToTimeslot(ownerID, event))
		}

		// CDV-TS-1: Series root / single event path.
		// Load the existing record to compute the EXDATE diff.
		existing, getErr := s.domain.GetTimeslot(ctx, ownerID, event.ID)
		if getErr != nil && !errors.Is(getErr, calstore.ErrNotFound) {
			return getErr
		}

		// Compute newly added and newly removed EXDATEs.
		var oldExDates []time.Time
		if existing != nil {
			oldExDates = existing.ExDates
		}
		added, removed := diffExDates(oldExDates, event.ExDates)

		// Newly added EXDATEs: delete any orphaned override for that occurrence.
		for _, ex := range added {
			klog.V(2).Infof("caldav/avail: PutEvent uid=%q EXDATE added=%v → removing orphaned override", event.ID, ex)
			if delErr := s.domain.DeleteTimeslotOverride(ctx, ownerID, event.ID, ex); delErr != nil && !errors.Is(delErr, calstore.ErrNotFound) {
				klog.Warningf("caldav/avail: PutEvent uid=%q EXDATE=%v DeleteTimeslotOverride: %v", event.ID, ex, delErr)
			}
		}

		// Newly removed EXDATEs (re-enabled occurrences): delete any orphaned
		// override so the series occurrence is restored without a stale override.
		for _, ex := range removed {
			klog.V(2).Infof("caldav/avail: PutEvent uid=%q EXDATE removed=%v → removing orphaned override", event.ID, ex)
			if delErr := s.domain.DeleteTimeslotOverride(ctx, ownerID, event.ID, ex); delErr != nil && !errors.Is(delErr, calstore.ErrNotFound) {
				klog.Warningf("caldav/avail: PutEvent uid=%q EXDATE=%v DeleteTimeslotOverride: %v", event.ID, ex, delErr)
			}
		}

		return s.domain.UpsertTimeslot(ctx, eventToTimeslot(ownerID, event))
	}
	return s.base.PutEvent(ctx, event)
}

// addExDateToSeriesRoot adds recurrenceID to the EXDATE list of the series
// root identified by (ownerID, uid). It is idempotent: if the date is already
// in the EXDATE list the call is a no-op. Any existing override record for
// that date is removed first, since the EXDATE supersedes it.
// Returns nil when the series root does not exist (already deleted).
func (s *combinedCalendarStore) addExDateToSeriesRoot(ctx context.Context, ownerID, uid string, recurrenceID time.Time) error {
	root, err := s.domain.GetTimeslot(ctx, ownerID, uid)
	if err != nil {
		if errors.Is(err, calstore.ErrNotFound) {
			return nil // series already gone — nothing to do
		}
		return err
	}
	// Remove any existing override for this date (superseded by EXDATE).
	if delErr := s.domain.DeleteTimeslotOverride(ctx, ownerID, uid, recurrenceID); delErr != nil && !errors.Is(delErr, calstore.ErrNotFound) {
		klog.Warningf("caldav/avail: addExDateToSeriesRoot uid=%q recurrenceID=%v DeleteTimeslotOverride: %v", uid, recurrenceID, delErr)
	}
	// Idempotency: skip if the date is already excluded.
	for _, ex := range root.ExDates {
		if ex.UTC().Truncate(time.Second).Equal(recurrenceID.UTC().Truncate(time.Second)) {
			return nil
		}
	}
	root.ExDates = append(root.ExDates, recurrenceID.UTC())
	return s.domain.UpsertTimeslot(ctx, root)
}

// availIsExcluded reports whether occ matches any time in exDates
// (UTC comparison, truncated to second precision).
func availIsExcluded(occ time.Time, exDates []time.Time) bool {
	for _, ex := range exDates {
		if occ.UTC().Truncate(time.Second).Equal(ex.UTC().Truncate(time.Second)) {
			return true
		}
	}
	return false
}

// diffExDates computes the difference between oldDates and newDates.
// added contains times present in newDates but not in oldDates.
// removed contains times present in oldDates but not in newDates.
// Comparison is truncated to second precision for tolerance.
func diffExDates(oldDates, newDates []time.Time) (added, removed []time.Time) {
	inOld := func(t time.Time) bool {
		for _, o := range oldDates {
			if o.UTC().Truncate(time.Second).Equal(t.UTC().Truncate(time.Second)) {
				return true
			}
		}
		return false
	}
	inNew := func(t time.Time) bool {
		for _, n := range newDates {
			if n.UTC().Truncate(time.Second).Equal(t.UTC().Truncate(time.Second)) {
				return true
			}
		}
		return false
	}
	for _, t := range newDates {
		if !inOld(t) {
			added = append(added, t)
		}
	}
	for _, t := range oldDates {
		if !inNew(t) {
			removed = append(removed, t)
		}
	}
	return
}

// DeleteEvent implements CalendarStore. Deletes from an availability calendar
// are handled per CDV-TS-2 based on whether the timeslot is a single event
// (Case A) or a recurring series root (Case B).
//
//   - Case A (single event, RRule == ""): check for active bookings in the
//     event’s time window; reject with ErrConflict if any exist, then delete.
//   - Case B (series root, RRule != ""): delete all overrides first, then
//     check for active bookings across all non-EXDATE occurrences; reject if
//     any exist, then delete the series root.
//
// Note: CDV-TS-2 Case C (deleting an individual override) is not triggered
// by a CalDAV DELETE because iOS sends a PUT that removes the override VEVENT
// and adds an EXDATE instead. If a DELETE is received for a path whose UID
// matches only an override record (unusual), it falls through to ErrNotFound.
func (s *combinedCalendarStore) DeleteEvent(ctx context.Context, calendarID, eventID, etag string) error {
	if ownerID, ok := availUserIDFromCalID(calendarID); ok {
		if !s.canAccessAvailCal(ctx, ownerID) {
			return calstore.ErrForbidden
		}

		ts, err := s.domain.GetTimeslot(ctx, ownerID, eventID)
		if err != nil {
			return err
		}

		if ts.RRule == "" {
			// CDV-TS-2 Case A: single event.
			if conflictErr := s.checkBookingConflict(ctx, ownerID, ts.StartAt, ts.EndAt); conflictErr != nil {
				return conflictErr
			}
			return s.domain.DeleteTimeslot(ctx, ownerID, eventID)
		}

		// CDV-TS-2 Case B: recurring series root.
		// Step 1: cascade-delete all override records for this UID.
		if overrideErr := s.domain.DeleteTimeslotOverrides(ctx, ownerID, eventID); overrideErr != nil {
			klog.Warningf("caldav/avail: DeleteEvent uid=%q DeleteTimeslotOverrides: %v", eventID, overrideErr)
		}
		// Step 2: check all non-EXDATE occurrences for active bookings.
		if conflictErr := s.checkSeriesBookingConflict(ctx, ownerID, ts); conflictErr != nil {
			return conflictErr
		}
		// Step 3: delete the series root record.
		return s.domain.DeleteTimeslot(ctx, ownerID, eventID)
	}
	return s.base.DeleteEvent(ctx, calendarID, eventID, etag)
}

// checkBookingConflict returns ErrConflict when at least one active booking
// exists for ownerID in the window [start, end).
func (s *combinedCalendarStore) checkBookingConflict(ctx context.Context, ownerID string, start, end time.Time) error {
	bookings, err := s.domain.ListActiveBookingsInWindow(ctx, ownerID, start, end)
	if err != nil {
		return err
	}
	if len(bookings) > 0 {
		klog.V(1).Infof("caldav/avail: checkBookingConflict ownerID=%q [%v, %v) → %d booking(s) block deletion", ownerID, start, end, len(bookings))
		return calstore.ErrConflict
	}
	return nil
}

// checkSeriesBookingConflict checks each RRULE expansion of ts (skipping EXDATE
// occurrences) for active bookings. Returns ErrConflict on first hit.
func (s *combinedCalendarStore) checkSeriesBookingConflict(ctx context.Context, ownerID string, ts *calstore.Timeslot) error {
	opts, err := rrulego.StrToROption(ts.RRule)
	if err != nil {
		// If RRULE cannot be parsed, skip conflict check but log a warning.
		klog.Warningf("caldav/avail: checkSeriesBookingConflict uid=%q invalid RRule %q: %v", ts.CalDAVUID, ts.RRule, err)
		return nil
	}
	opts.Dtstart = ts.StartAt
	rr, err := rrulego.NewRRule(*opts)
	if err != nil {
		klog.Warningf("caldav/avail: checkSeriesBookingConflict uid=%q cannot build RRule: %v", ts.CalDAVUID, err)
		return nil
	}
	duration := ts.EndAt.Sub(ts.StartAt)
	// Expand up to a reasonable horizon (10 years).
	horizon := time.Now().UTC().AddDate(10, 0, 0)
	for _, occ := range rr.Between(ts.StartAt, horizon, true) {
		if availIsExcluded(occ, ts.ExDates) {
			continue
		}
		if conflictErr := s.checkBookingConflict(ctx, ownerID, occ, occ.Add(duration)); conflictErr != nil {
			return conflictErr
		}
	}
	return nil
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
