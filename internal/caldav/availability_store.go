package caldav

// availability_store.go provides a CalendarStore implementation that merges
// the regular CalDAV calendars (backed by the base CalendarStore) with virtual
// per-staff availability calendars (backed by the DomainStore availability layer).
//
// Availability calendar IDs use the "avail-<userID>" prefix. The access rules
// are:
//
//   - Staff users: only their own "avail-<userID>" calendar is visible and
//     writable. Availability records stored there represent their availability windows.
//   - Administrator users: all staff availability calendars are visible and
//     writable (read-only access is intentionally not distinguished so admins
//     may also correct availability records on behalf of staff).

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	calstore "schedio/internal/store"

	rrulego "github.com/teambition/rrule-go"

	"k8s.io/klog/v2"
)

const availCalPrefix = "avail-"

// bookingEventPrefix is prepended to a Booking.ID to form the CalDAV event ID
// for booking-derived synthetic events in the default calendar.
const bookingEventPrefix = "booking-"

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

// bookingIDFromEventID extracts the Booking.ID from a booking-derived event ID.
// Returns "", false when eventID does not have the booking prefix.
func bookingIDFromEventID(eventID string) (string, bool) {
	if strings.HasPrefix(eventID, bookingEventPrefix) {
		return strings.TrimPrefix(eventID, bookingEventPrefix), true
	}
	return "", false
}

// bookingToEvent converts a Booking into a synthetic read-only CalDAV Event.
// serviceName is the human-readable service label (may be "" if unavailable).
// The event is TENTATIVE for reserved bookings and CONFIRMED once approved.
func bookingToEvent(calID string, b *calstore.Booking, contactName, serviceName string) *calstore.Event {
	// Build summary: "<ContactName> (<ServiceName>)", falling back gracefully
	// when either component is unavailable.
	var summary string
	switch {
	case contactName != "" && serviceName != "":
		summary = contactName + " (" + serviceName + ")"
	case contactName != "":
		summary = contactName
	case serviceName != "":
		summary = serviceName
	default:
		summary = "Booking"
	}
	status := calstore.StatusTentative
	if b.State == calstore.BookingStateConfirmed {
		status = calstore.StatusConfirmed
	}
	// Use UpdatedAt as a stable proxy for ETag when CalDAVETag is empty.
	etag := b.CalDAVETag
	if etag == "" {
		etag = fmt.Sprintf("%x", b.UpdatedAt.UnixNano())
	}
	return &calstore.Event{
		ID:         bookingEventPrefix + b.ID,
		CalendarID: calID,
		Summary:    summary,
		Start:      b.StartAt,
		End:        b.EndAt,
		Status:     status,
		Opacity:    calstore.OpacityOpaque,
		Created:    b.CreatedAt,
		Modified:   b.UpdatedAt,
		Sequence:   b.Sequence,
		ETag:       etag,
	}
}

// combinedCalendarStore implements calstore.CalendarStore by merging the
// regular CalDAV calendars from base with virtual availability calendars
// synthesised from the DomainStore's availability data.
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

// availabilityToEvent converts an Availability record into a synthetic Event for the CalDAV
// layer. The CalDAV UID is used as the event ID. The RRule and ExDates are
// preserved so that eventToObject can emit proper RRULE:/EXDATE: properties
// in the iCal output.
//
// The event is synthesised as free (SUMMARY: "Free", TRANSP: TRANSPARENT)
// by default. Callers that need occupancy-aware synthesis should use
// availabilityToEventWithOccupancy instead.
func availabilityToEvent(calID string, t *calstore.Availability) *calstore.Event {
	return &calstore.Event{
		ID:           t.CalDAVUID,
		CalendarID:   calID,
		Summary:      "Free",
		Start:        t.StartAt,
		End:          t.EndAt,
		Opacity:      calstore.OpacityTransparent,
		Status:       calstore.StatusConfirmed,
		Created:      t.CreatedAt,
		Modified:     t.UpdatedAt,
		ETag:         t.CalDAVETag,
		RRule:        t.RRule,
		RecurrenceID: t.RecurrenceID,
		ExDates:      append([]time.Time(nil), t.ExDates...),
	}
}

// availabilityToEventWithOccupancy synthesises a CalDAV Event from an Availability record and
// enriches its SUMMARY and TRANSP with booking occupancy information
// (CDV-AVAIL-READ-1 / CDV-AVAIL-READ-2 / CDV-AVAIL-READ-3).
//
// Rules:
//   - Recurring series roots (RRule != "", RecurrenceID zero) are always
//     returned as free+transparent because individual occurrence occupancy
//     cannot be expressed on a single series-root VEVENT. Individual booked
//     occurrences appear as annotated override events.
//   - Single events and recurring overrides: DomainStore.ListActiveBookingsInWindow
//     is called for [ts.StartAt, ts.EndAt). If the result is non-empty the
//     event receives the booked representation
//     (SUMMARY: "<contact> (<service>)", TRANSP: OPAQUE); otherwise it
//     stays as the free representation (SUMMARY: "Free", TRANSP: TRANSPARENT).
//
// Store lookup errors during the occupancy check degrade gracefully: the free
// representation is returned so that a transient error does not break the
// entire listing.
func (s *combinedCalendarStore) availabilityToEventWithOccupancy(ctx context.Context, calID, ownerID string, ts *calstore.Availability) (*calstore.Event, error) {
	ev := availabilityToEvent(calID, ts)

	// Fetch the calendar URL once; used in the free branches below (CDV-AVAIL-READ-2).
	calendarURL := ""
	if settings, settErr := s.domain.GetSettings(ctx); settErr == nil {
		calendarURL = settings.CalendarURL
	}

	// CDV-AVAIL-READ-1: recurring series roots are always free.
	if ts.RRule != "" && ts.RecurrenceID.IsZero() {
		ev.URL = calendarURL
		return ev, nil
	}

	// CDV-AVAIL-READ-1: check occupancy for single events and overrides.
	bookings, err := s.domain.ListActiveBookingsInWindow(ctx, ownerID, ts.StartAt, ts.EndAt)
	if err != nil {
		// Degrade gracefully: return the free representation.
		klog.Warningf("caldav/avail: availabilityToEventWithOccupancy uid=%q occupancy check: %v", ts.CalDAVUID, err)
		ev.URL = calendarURL
		return ev, nil
	}
	if len(bookings) == 0 {
		// CDV-AVAIL-READ-2: free — include CalendarURL so staff can open the server directly.
		ev.URL = calendarURL
		return ev, nil
	}

	// CDV-AVAIL-READ-3: booked — use the first booking (sorted by start_at ascending).
	b := bookings[0]
	contactName := ""
	if b.ContactID != "" {
		if contact, cErr := s.domain.GetContact(ctx, b.ContactID); cErr == nil {
			parts := strings.TrimSpace(contact.FirstName + " " + contact.LastName)
			if parts != "" {
				contactName = parts
			}
		}
	}
	serviceName := ""
	if svc, svcErr := s.domain.GetService(ctx, b.ServiceID); svcErr == nil {
		serviceName = svc.Name
	}
	ev.Summary = availabilityBookedSummary(contactName, serviceName)
	ev.Opacity = calstore.OpacityOpaque
	return ev, nil
}

// availabilityBookedSummary builds the display title for a booked availability event
// (CDV-AVAIL-READ-3): "<contactName> (<serviceName>)", degrading gracefully when
// either component is unavailable.
func availabilityBookedSummary(contactName, serviceName string) string {
	switch {
	case contactName != "" && serviceName != "":
		return contactName + " (" + serviceName + ")"
	case contactName != "":
		return contactName
	case serviceName != "":
		return "(" + serviceName + ")"
	default:
		return "Booked"
	}
}

// syntheticBookedOverrides generates additional synthetic recurring-override
// VEVENT records for recurring series occurrences that have active bookings.
//
// This implements CDV-AVAIL-READ-1: individual booked occurrences of a
// recurring series are embedded as RECURRENCE-ID override VEVENT components
// inside the same VCALENDAR as the series root (via Event.InlineVEVENTs).
// This is the correct RFC 4791 approach: a single .ics resource holds the
// series root VEVENT plus one override VEVENT per booked occurrence. iOS
// Calendar renders booked occurrences with the override's SUMMARY/TRANSP while
// all other occurrences stay governed by the series root ("Free"/TRANSPARENT).
//
// Returns uid → []overrideEvent so callers can attach the slices to the
// matching series root Event.InlineVEVENTs.
//
// Errors are handled by graceful degradation: on any store failure the method
// returns (nil, nil) and callers proceed without the synthetic records.
func (s *combinedCalendarStore) syntheticBookedOverrides(
	ctx context.Context, calID, ownerID string, tss []*calstore.Availability,
) (map[string][]*calstore.Event, error) {
	// Collect series roots (RRule present, no RecurrenceID).
	var seriesRoots []*calstore.Availability
	for _, ts := range tss {
		if ts.RRule != "" && ts.RecurrenceID.IsZero() {
			seriesRoots = append(seriesRoots, ts)
		}
	}
	if len(seriesRoots) == 0 {
		return nil, nil
	}

	// Fetch all active bookings for this user (no time bounds).
	// ListAllBookingsInWindow correctly handles zero start/end as "unbounded";
	// we then filter by ownerID to restrict to this staff member's bookings.
	allBookings, err := s.domain.ListAllBookingsInWindow(ctx, time.Time{}, time.Time{})
	if err != nil {
		klog.Warningf("caldav/avail: syntheticBookedOverrides ownerID=%q: %v", ownerID, err)
		return nil, nil
	}
	var bookings []*calstore.Booking
	for _, b := range allBookings {
		if b.UserID == ownerID {
			bookings = append(bookings, b)
		}
	}
	if len(bookings) == 0 {
		return nil, nil
	}

	// Collect (uid → set of recurrenceID unix-seconds) for explicit override
	// records already stored so we don't emit a conflicting synthetic override.
	existingOverrides := make(map[string]map[int64]bool)
	allAvail, err := s.domain.ListAvailability(ctx, ownerID, time.Time{}, time.Time{})
	if err != nil {
		klog.Warningf("caldav/avail: syntheticBookedOverrides ownerID=%q ListAvailability: %v", ownerID, err)
		// Proceed without deduplication against explicit overrides.
	} else {
		for _, a := range allAvail {
			if !a.RecurrenceID.IsZero() {
				if existingOverrides[a.CalDAVUID] == nil {
					existingOverrides[a.CalDAVUID] = make(map[int64]bool)
				}
				existingOverrides[a.CalDAVUID][a.RecurrenceID.UTC().Truncate(time.Second).Unix()] = true
			}
		}
	}

	// Build an RRULE expander for each series root.
	type seriesExpander struct {
		root *calstore.Availability
		rr   *rrulego.RRule
		dur  time.Duration
	}
	expanders := make([]seriesExpander, 0, len(seriesRoots))
	for _, root := range seriesRoots {
		opts, parseErr := rrulego.StrToROption(root.RRule)
		if parseErr != nil {
			klog.Warningf("caldav/avail: syntheticBookedOverrides uid=%q invalid RRule %q: %v", root.CalDAVUID, root.RRule, parseErr)
			continue
		}
		opts.Dtstart = root.StartAt
		rr, buildErr := rrulego.NewRRule(*opts)
		if buildErr != nil {
			klog.Warningf("caldav/avail: syntheticBookedOverrides uid=%q cannot build RRule: %v", root.CalDAVUID, buildErr)
			continue
		}
		expanders = append(expanders, seriesExpander{
			root: root,
			rr:   rr,
			dur:  root.EndAt.Sub(root.StartAt),
		})
	}

	// uid → []override events
	overridesByUID := make(map[string][]*calstore.Event)
	for _, b := range bookings {
		for _, exp := range expanders {
			// Check whether b.StartAt is exactly an occurrence of this series.
			occs := exp.rr.Between(b.StartAt, b.StartAt, true)
			if len(occs) == 0 {
				continue
			}
			occ := occs[0]
			if !occ.UTC().Truncate(time.Second).Equal(b.StartAt.UTC().Truncate(time.Second)) {
				continue
			}
			// Skip EXDATE-excluded occurrences.
			if availIsExcluded(occ, exp.root.ExDates) {
				continue
			}
			// Skip if an explicit owner-stored override already covers this occurrence.
			if m := existingOverrides[exp.root.CalDAVUID]; m != nil {
				if m[occ.UTC().Truncate(time.Second).Unix()] {
					continue
				}
			}
			// Resolve contact and service names for the summary.
			contactName := ""
			if b.ContactID != "" {
				if contact, cErr := s.domain.GetContact(ctx, b.ContactID); cErr == nil {
					contactName = strings.TrimSpace(contact.FirstName + " " + contact.LastName)
				}
			}
			serviceName := ""
			if svc, svcErr := s.domain.GetService(ctx, b.ServiceID); svcErr == nil {
				serviceName = svc.Name
			}
			// Build a synthetic RECURRENCE-ID override for this booked occurrence.
			// It will be emitted as an additional VEVENT component inside the
			// series root's VCALENDAR (Event.InlineVEVENTs), NOT as a separate
			// top-level CalDAV resource.
			overridesByUID[exp.root.CalDAVUID] = append(overridesByUID[exp.root.CalDAVUID], &calstore.Event{
				ID:           exp.root.CalDAVUID,
				CalendarID:   calID,
				Summary:      availabilityBookedSummary(contactName, serviceName),
				Start:        occ,
				End:          occ.Add(exp.dur),
				Opacity:      calstore.OpacityOpaque,
				Status:       calstore.StatusConfirmed,
				Modified:     b.UpdatedAt,
				ETag:         fmt.Sprintf("%x", b.UpdatedAt.UnixNano()),
				RecurrenceID: occ,
			})
			break // a booking can only match one series root
		}
	}
	return overridesByUID, nil
}

// attachInlineOverrides fetches synthetic booked-occurrence overrides and
// attaches them to their matching series-root Event via InlineVEVENTs.
// This must be called after the flat list of events has been built so that
// each series root Event can receive its associated override slice.
//
// Crucially, it also recomputes ev.ETag to incorporate the booking data.
// Without this, iOS Calendar would cache the series-root .ics body by the
// original ETag (based on the Availability record only) and never re-download
// it after a booking is created — even though CTag signals a collection change.
// With a booking-sensitive ETag the PROPFIND Depth:1 response shows a changed
// ETag for the series root, forcing iOS to re-fetch the .ics and see the
// inline override VEVENT.
func (s *combinedCalendarStore) attachInlineOverrides(
	ctx context.Context, calID, ownerID string, tss []*calstore.Availability, events []*calstore.Event,
) {
	overridesByUID, _ := s.syntheticBookedOverrides(ctx, calID, ownerID, tss)
	for _, ev := range events {
		if ev.RRule == "" || !ev.RecurrenceID.IsZero() {
			continue
		}
		ovs := overridesByUID[ev.ID] // nil when no bookings match

		// Derive a booking-sensitive ETag whether or not there are overrides.
		// This ensures the ETag shrinks back when a booking is cancelled.
		h := sha256.New()
		fmt.Fprintf(h, "root:%s\n", ev.ETag)
		for _, ov := range ovs {
			fmt.Fprintf(h, "override:%s:%d\n", ov.ID, ov.Modified.UnixNano())
		}
		ev.ETag = fmt.Sprintf("%x", h.Sum(nil))[:16]

		if len(ovs) > 0 {
			ev.InlineVEVENTs = ovs
		}
	}
}

// eventToAvailability converts an Event received via CalDAV PUT into an Availability record
// suitable for persistence via DomainStore.UpsertAvailability.
func eventToAvailability(ownerUserID string, e *calstore.Event) *calstore.Availability {
	return &calstore.Availability{
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
// the Availability record with the matching CalDAV UID; booking-derived events are looked
// up from the DomainStore; others are delegated.
func (s *combinedCalendarStore) GetEvent(ctx context.Context, calendarID, eventID string) (*calstore.Event, error) {
	if ownerID, ok := availUserIDFromCalID(calendarID); ok {
		if !s.canAccessAvailCal(ctx, ownerID) {
			return nil, calstore.ErrForbidden
		}
		ti, err := s.domain.GetAvailability(ctx, ownerID, eventID)
		if err != nil {
			return nil, err
		}
		ev, evErr := s.availabilityToEventWithOccupancy(ctx, calendarID, ownerID, ti)
		if evErr != nil {
			return nil, evErr
		}
		// CDV-AVAIL-READ-1: for series roots, embed booked-occurrence overrides as
		// InlineVEVENTs so that a GET of this .ics resource returns the full
		// VCALENDAR including RECURRENCE-ID override VEVENTs.
		if ev.RRule != "" && ev.RecurrenceID.IsZero() {
			s.attachInlineOverrides(ctx, calendarID, ownerID, []*calstore.Availability{ti}, []*calstore.Event{ev})
		}
		return ev, nil
	}
	// Non-avail: check whether this is a booking-derived event.
	if bookingID, ok := bookingIDFromEventID(eventID); ok {
		b, err := s.domain.GetBooking(ctx, bookingID)
		if err != nil {
			return nil, err
		}
		serviceName := ""
		if svc, svcErr := s.domain.GetService(ctx, b.ServiceID); svcErr == nil {
			serviceName = svc.Name
		}
		contactName := ""
		if b.ContactID != "" {
			if contact, cErr := s.domain.GetContact(ctx, b.ContactID); cErr == nil {
				parts := strings.TrimSpace(contact.FirstName + " " + contact.LastName)
				if parts != "" {
					contactName = parts
				}
			}
		}
		return bookingToEvent(calendarID, b, contactName, serviceName), nil
	}
	return s.base.GetEvent(ctx, calendarID, eventID)
}

// ListEvents implements CalendarStore. Availability events are produced by
// converting raw (unexpanded) Availability records into Events that carry the RRULE.
// Returning the RRULE VEVENT lets the CalDAV client (e.g. iOS Calendar)
// expand recurring events itself, which is the correct RFC 4791 behaviour and
// avoids sending thousands of individual occurrence VEVENTs over the wire.
func (s *combinedCalendarStore) ListEvents(ctx context.Context, calendarID string, _ /*start*/, _ /*end*/ time.Time) ([]*calstore.Event, error) {
	if ownerID, ok := availUserIDFromCalID(calendarID); ok {
		if !s.canAccessAvailCal(ctx, ownerID) {
			return nil, calstore.ErrForbidden
		}
		// Use raw (unexpanded) availability records so that each recurring series is
		// represented by a single VEVENT with RRULE rather than one VEVENT
		// per expanded occurrence.
		tss, err := s.domain.ListRawAvailability(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		events := make([]*calstore.Event, 0, len(tss))
		for _, ts := range tss {
			ev, evErr := s.availabilityToEventWithOccupancy(ctx, calendarID, ownerID, ts)
			if evErr != nil {
				return nil, evErr
			}
			events = append(events, ev)
		}
		// CDV-AVAIL-READ-1: embed booked-occurrence override VEVENTs as
		// InlineVEVENTs on their series root Events so they are emitted as
		// additional VEVENT components inside the same VCALENDAR (.ics resource).
		// This is the correct RFC 4791 approach: overrides share the UID with the
		// series root and are not separate .ics resources.
		s.attachInlineOverrides(ctx, calendarID, ownerID, tss, events)
		return events, nil
	}
	// Non-avail calendar: merge base CalDAV events with booking-derived events
	// so that reserved and confirmed bookings appear to CalDAV clients.
	baseEvents, err := s.base.ListEvents(ctx, calendarID, time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}
	bookingEvents, err := s.mergeBookingEvents(ctx, calendarID, time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}
	return append(baseEvents, bookingEvents...), nil
}

// mergeBookingEvents fetches all non-cancelled Bookings in [start, end] from
// the DomainStore and converts them to synthetic CalDAV Events in calendarID.
// Errors during service-name lookup are silently ignored so that a missing
// service does not prevent the calendar from being returned.
func (s *combinedCalendarStore) mergeBookingEvents(ctx context.Context, calendarID string, start, end time.Time) ([]*calstore.Event, error) {
	bookings, err := s.domain.ListAllBookingsInWindow(ctx, start, end)
	if err != nil {
		return nil, err
	}
	events := make([]*calstore.Event, 0, len(bookings))
	for _, b := range bookings {
		serviceName := ""
		if svc, svcErr := s.domain.GetService(ctx, b.ServiceID); svcErr == nil {
			serviceName = svc.Name
		}
		contactName := ""
		if b.ContactID != "" {
			if contact, cErr := s.domain.GetContact(ctx, b.ContactID); cErr == nil {
				parts := strings.TrimSpace(contact.FirstName + " " + contact.LastName)
				if parts != "" {
					contactName = parts
				}
			}
		}
		events = append(events, bookingToEvent(calendarID, b, contactName, serviceName))
	}
	return events, nil
}

// PutEvent implements CalendarStore. Writes to an availability calendar are
// persisted as Availability records via DomainStore; others are delegated to the base store.
//
// The method implements CDV-AVAIL-1: when the incoming event is an override
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
		// CDV-AVAIL-1: Override instances.
		if !event.RecurrenceID.IsZero() {
			klog.V(2).Infof("caldav/avail: PutEvent uid=%q recurrenceID=%v → upserting override", event.ID, event.RecurrenceID)
			// CDV-AVAIL-1c: A CANCELLED override represents deletion of that
			// occurrence (iOS may send this form in addition to – or instead of –
			// adding an EXDATE to the series root VEVENT). Convert it to an EXDATE
			// on the series root so ListAvailability correctly excludes the occurrence
			// regardless of which form the client used.
			if event.Status == calstore.StatusCancelled {
				klog.V(2).Infof("caldav/avail: PutEvent uid=%q recurrenceID=%v STATUS:CANCELLED → converting to EXDATE", event.ID, event.RecurrenceID)
				return s.addExDateToSeriesRoot(ctx, ownerID, event.ID, event.RecurrenceID)
			}
			return s.domain.UpsertAvailability(ctx, eventToAvailability(ownerID, event))
		}

		// CDV-AVAIL-1: Series root / single event path.
		// Load the existing record to compute the EXDATE diff.
		existing, getErr := s.domain.GetAvailability(ctx, ownerID, event.ID)
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
			if delErr := s.domain.DeleteAvailabilityOverride(ctx, ownerID, event.ID, ex); delErr != nil && !errors.Is(delErr, calstore.ErrNotFound) {
				klog.Warningf("caldav/avail: PutEvent uid=%q EXDATE=%v DeleteAvailabilityOverride: %v", event.ID, ex, delErr)
			}
		}

		// Newly removed EXDATEs (re-enabled occurrences): delete any orphaned
		// override so the series occurrence is restored without a stale override.
		for _, ex := range removed {
			klog.V(2).Infof("caldav/avail: PutEvent uid=%q EXDATE removed=%v → removing orphaned override", event.ID, ex)
			if delErr := s.domain.DeleteAvailabilityOverride(ctx, ownerID, event.ID, ex); delErr != nil && !errors.Is(delErr, calstore.ErrNotFound) {
				klog.Warningf("caldav/avail: PutEvent uid=%q EXDATE=%v DeleteAvailabilityOverride: %v", event.ID, ex, delErr)
			}
		}

		return s.domain.UpsertAvailability(ctx, eventToAvailability(ownerID, event))
	}
	// Booking-derived events are read-only; reject any write attempt.
	if _, ok := bookingIDFromEventID(event.ID); ok {
		return calstore.ErrForbidden
	}
	return s.base.PutEvent(ctx, event)
}

// addExDateToSeriesRoot adds recurrenceID to the EXDATE list of the series
// root identified by (ownerID, uid). It is idempotent: if the date is already
// in the EXDATE list the call is a no-op. Any existing override record for
// that date is removed first, since the EXDATE supersedes it.
// Returns nil when the series root does not exist (already deleted).
func (s *combinedCalendarStore) addExDateToSeriesRoot(ctx context.Context, ownerID, uid string, recurrenceID time.Time) error {
	root, err := s.domain.GetAvailability(ctx, ownerID, uid)
	if err != nil {
		if errors.Is(err, calstore.ErrNotFound) {
			return nil // series already gone — nothing to do
		}
		return err
	}
	// Remove any existing override for this date (superseded by EXDATE).
	if delErr := s.domain.DeleteAvailabilityOverride(ctx, ownerID, uid, recurrenceID); delErr != nil && !errors.Is(delErr, calstore.ErrNotFound) {
		klog.Warningf("caldav/avail: addExDateToSeriesRoot uid=%q recurrenceID=%v DeleteAvailabilityOverride: %v", uid, recurrenceID, delErr)
	}
	// Idempotency: skip if the date is already excluded.
	for _, ex := range root.ExDates {
		if ex.UTC().Truncate(time.Second).Equal(recurrenceID.UTC().Truncate(time.Second)) {
			return nil
		}
	}
	root.ExDates = append(root.ExDates, recurrenceID.UTC())
	return s.domain.UpsertAvailability(ctx, root)
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
// are handled per CDV-AVAIL-2 based on whether the record is a single event
// (Case A) or a recurring series root (Case B).
//
//   - Case A (single event, RRule == ""): check for active bookings in the
//     event’s time window; reject with ErrConflict if any exist, then delete.
//   - Case B (series root, RRule != ""): delete all overrides first, then
//     check for active bookings across all non-EXDATE occurrences; reject if
//     any exist, then delete the series root.
//
// Note: CDV-AVAIL-2 Case C (deleting an individual override) is not triggered
// by a CalDAV DELETE because iOS sends a PUT that removes the override VEVENT
// and adds an EXDATE instead. If a DELETE is received for a path whose UID
// matches only an override record (unusual), it falls through to ErrNotFound.
func (s *combinedCalendarStore) DeleteEvent(ctx context.Context, calendarID, eventID, etag string) error {
	if ownerID, ok := availUserIDFromCalID(calendarID); ok {
		if !s.canAccessAvailCal(ctx, ownerID) {
			return calstore.ErrForbidden
		}

		ts, err := s.domain.GetAvailability(ctx, ownerID, eventID)
		if err != nil {
			return err
		}

		if ts.RRule == "" {
			// CDV-AVAIL-2 Case A: single event.
			if conflictErr := s.checkBookingConflict(ctx, ownerID, ts.StartAt, ts.EndAt); conflictErr != nil {
				return conflictErr
			}
			return s.domain.DeleteAvailability(ctx, ownerID, eventID)
		}

		// CDV-AVAIL-2 Case B: recurring series root.
		// Step 1: cascade-delete all override records for this UID.
		if overrideErr := s.domain.DeleteAvailabilityOverrides(ctx, ownerID, eventID); overrideErr != nil {
			klog.Warningf("caldav/avail: DeleteEvent uid=%q DeleteAvailabilityOverrides: %v", eventID, overrideErr)
		}
		// Step 2: check all non-EXDATE occurrences for active bookings.
		if conflictErr := s.checkSeriesBookingConflict(ctx, ownerID, ts); conflictErr != nil {
			return conflictErr
		}
		// Step 3: delete the series root record.
		return s.domain.DeleteAvailability(ctx, ownerID, eventID)
	}
	// Booking-derived events are read-only; reject any delete attempt.
	if _, ok := bookingIDFromEventID(eventID); ok {
		return calstore.ErrForbidden
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
func (s *combinedCalendarStore) checkSeriesBookingConflict(ctx context.Context, ownerID string, ts *calstore.Availability) error {
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
// truncated SHA-256 hash of all raw availability record UIDs and ETags; for base
// calendars the CTag is derived from the base store's token combined with a
// hash over the current booking snapshot so that CalDAV clients detect new or
// updated bookings.
func (s *combinedCalendarStore) CTag(ctx context.Context, calendarID string) (string, error) {
	if ownerID, ok := availUserIDFromCalID(calendarID); ok {
		if !s.canAccessAvailCal(ctx, ownerID) {
			return "", calstore.ErrForbidden
		}
		tss, err := s.domain.ListRawAvailability(ctx, ownerID)
		if err != nil {
			return "", err
		}
		h := sha256.New()
		for _, ts := range tss {
			fmt.Fprintf(h, "%s:%s\n", ts.CalDAVUID, ts.CalDAVETag)
		}
		// Include active bookings so that booking changes cause clients to
		// re-sync and pick up updated synthetic override VEVENTs.
		if bks, bErr := s.domain.ListAllBookingsInWindow(ctx, time.Time{}, time.Time{}); bErr == nil {
			for _, b := range bks {
				if b.UserID == ownerID {
					fmt.Fprintf(h, "b:%s:%d\n", b.ID, b.UpdatedAt.UnixNano())
				}
			}
		}
		return fmt.Sprintf("%x", h.Sum(nil))[:16], nil
	}
	// Non-avail calendar: combine base CTag with a hash over the booking
	// snapshot so that any booking change invalidates the client cache.
	baseCtag, err := s.base.CTag(ctx, calendarID)
	if err != nil {
		return "", err
	}
	bookings, err := s.domain.ListAllBookingsInWindow(ctx, time.Time{}, time.Time{})
	if err != nil {
		// Gracefully degrade: at least return the base ctag.
		klog.Warningf("caldav/avail: CTag %q: ListAllBookingsInWindow: %v", calendarID, err)
		return baseCtag, nil
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\n", baseCtag)
	for _, b := range bookings {
		fmt.Fprintf(h, "%s:%d:%d\n", b.ID, b.Sequence, b.UpdatedAt.UnixNano())
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16], nil
}
