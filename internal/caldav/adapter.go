package caldav

// adapter.go bridges our CalendarStore interface to the emersion/go-webdav
// caldav.Backend interface.  The library then handles all CalDAV
// protocol details: XML parsing, multistatus responses, PROPFIND depth,
// REPORT filtering, ETag handling, etc.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	ical "github.com/emersion/go-ical"
	webdav "github.com/emersion/go-webdav"
	extcaldav "github.com/emersion/go-webdav/caldav"
	"k8s.io/klog/v2"

	calstore "schedio/internal/store"
)

// storeAdapter wraps our CalendarStore so it satisfies extcaldav.Backend.
type storeAdapter struct {
	store    calstore.CalendarStore
	rootPath string // "" or "/ui" etc., matches the application root-path flag
}

// calHomePath returns the URL prefix shared by all calendar collections,
// e.g. "" → "/caldav/user/calendars/", "/ui" → "/ui/caldav/user/calendars/".
//
// go-webdav determines resource types by counting path depth after stripping the
// handler Prefix ("/caldav").  Correct depths:
//   depth 0  /caldav/                          → server root
//   depth 1  /caldav/user/                     → user principal
//   depth 2  /caldav/user/calendars/           → calendar home set  ← ListCalendars
//   depth 3  /caldav/user/calendars/<id>/      → calendar           ← ListCalendarObjects
//   depth 4  /caldav/user/calendars/<id>/e.ics → calendar object
func (a *storeAdapter) calHomePath() string {
	return a.rootPath + "/caldav/user/calendars/"
}

// calIDFromPath extracts the calendar ID from an absolute path like
// /caldav/user/calendars/dummy/ → "dummy".
func (a *storeAdapter) calIDFromPath(path string) string {
	rel := strings.TrimPrefix(path, a.calHomePath())
	rel = strings.Trim(rel, "/")
	if i := strings.Index(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return rel
}

// eventIDFromPath extracts (calID, eventID) from a path like
// /caldav/user/calendars/dummy/abc.ics → ("dummy", "abc").
func (a *storeAdapter) eventIDFromPath(path string) (calID, eventID string) {
	rel := strings.TrimPrefix(path, a.calHomePath())
	rel = strings.Trim(rel, "/")
	parts := strings.SplitN(rel, "/", 2)
	calID = parts[0]
	if len(parts) == 2 {
		eventID = strings.TrimSuffix(parts[1], ".ics")
	}
	return
}

func (a *storeAdapter) pathForCalendar(calID string) string {
	return a.calHomePath() + calID + "/"
}

func (a *storeAdapter) pathForEvent(calID, eventID string) string {
	return a.calHomePath() + calID + "/" + eventID + ".ics"
}

// ── webdav.UserPrincipalBackend ───────────────────────────────────────────────

// CurrentUserPrincipal returns the principal URL (depth 1 within the CalDAV
// namespace so go-webdav handles it via its propFindUserPrincipal code path).
func (a *storeAdapter) CurrentUserPrincipal(_ context.Context) (string, error) {
	return a.rootPath + "/caldav/user/", nil
}

// ── caldav.Backend ────────────────────────────────────────────────────────────

// CalendarHomeSetPath returns the calendar home set path (depth 2 within the
// CalDAV namespace so go-webdav calls ListCalendars on PROPFIND here).
func (a *storeAdapter) CalendarHomeSetPath(_ context.Context) (string, error) {
	return a.rootPath + "/caldav/user/calendars/", nil
}

// CreateCalendar is not supported; calendars are pre-configured server-side.
func (a *storeAdapter) CreateCalendar(_ context.Context, _ *extcaldav.Calendar) error {
	return webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("calendar creation not supported"))
}

// ListCalendars returns all calendars from the store.
func (a *storeAdapter) ListCalendars(ctx context.Context) ([]extcaldav.Calendar, error) {
	cals, err := a.store.ListCalendars(ctx)
	if err != nil {
		return nil, err
	}
	klog.V(2).Infof("caldav: ListCalendars → %d calendar(s)", len(cals))
	result := make([]extcaldav.Calendar, 0, len(cals))
	for _, cal := range cals {
		klog.V(2).Infof("caldav: calendar id=%q name=%q path=%q", cal.ID, cal.Name, a.pathForCalendar(cal.ID))
		result = append(result, extcaldav.Calendar{
			Path:                  a.pathForCalendar(cal.ID),
			Name:                  cal.Name,
			Description:           cal.Description,
			SupportedComponentSet: []string{ical.CompEvent},
		})
	}
	return result, nil
}

// GetCalendar returns a single calendar collection by path.
func (a *storeAdapter) GetCalendar(ctx context.Context, path string) (*extcaldav.Calendar, error) {
	calID := a.calIDFromPath(path)
	cal, err := a.store.GetCalendar(ctx, calID)
	if err != nil {
		if err == calstore.ErrNotFound {
			return nil, webdav.NewHTTPError(http.StatusNotFound, err)
		}
		return nil, err
	}
	return &extcaldav.Calendar{
		Path:                  a.pathForCalendar(cal.ID),
		Name:                  cal.Name,
		Description:           cal.Description,
		SupportedComponentSet: []string{ical.CompEvent},
	}, nil
}

// GetCalendarObject returns a single event resource by path.
func (a *storeAdapter) GetCalendarObject(ctx context.Context, path string, _ *extcaldav.CalendarCompRequest) (*extcaldav.CalendarObject, error) {
	calID, eventID := a.eventIDFromPath(path)
	if calID == "" || eventID == "" {
		return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("invalid path: %s", path))
	}
	e, err := a.store.GetEvent(ctx, calID, eventID)
	if err != nil {
		if err == calstore.ErrNotFound {
			return nil, webdav.NewHTTPError(http.StatusNotFound, err)
		}
		return nil, err
	}
	obj := a.eventToObject(e)
	return &obj, nil
}

// ListCalendarObjects returns all events in a calendar collection.
func (a *storeAdapter) ListCalendarObjects(ctx context.Context, path string, _ *extcaldav.CalendarCompRequest) ([]extcaldav.CalendarObject, error) {
	calID := a.calIDFromPath(path)
	if calID == "" {
		return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("invalid path: %s", path))
	}
	events, err := a.store.ListEvents(ctx, calID, time.Time{}, time.Time{})
	if err != nil {
		if err == calstore.ErrNotFound {
			return nil, webdav.NewHTTPError(http.StatusNotFound, err)
		}
		return nil, err
	}
	klog.V(2).Infof("caldav: ListCalendarObjects calID=%q path=%q → %d event(s)", calID, path, len(events))
	objects := make([]extcaldav.CalendarObject, 0, len(events))
	for _, e := range events {
		obj := a.eventToObject(e)
		if klog.V(3).Enabled() && len(objects) == 0 {
			// Log the first encoded event so iCal validity can be verified.
			if buf, err := encodeCalendar(obj.Data); err != nil {
				klog.V(3).Infof("caldav: iCal encode error for uid=%q: %v", e.ID, err)
			} else {
				klog.V(3).Infof("caldav: first event iCal:\n%s", buf)
			}
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

// QueryCalendarObjects executes a calendar-query report.
// It pre-filters by time range via the store, then applies any remaining
// filters with the library's Filter helper.
func (a *storeAdapter) QueryCalendarObjects(ctx context.Context, path string, query *extcaldav.CalendarQuery) ([]extcaldav.CalendarObject, error) {
	calID := a.calIDFromPath(path)
	if calID == "" {
		return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("invalid path: %s", path))
	}
	start, end := extractTimeRange(query)
	klog.V(2).Infof("caldav: QueryCalendarObjects calID=%q start=%v end=%v", calID, start, end)
	events, err := a.store.ListEvents(ctx, calID, start, end)
	if err != nil {
		if err == calstore.ErrNotFound {
			return nil, webdav.NewHTTPError(http.StatusNotFound, err)
		}
		return nil, err
	}
	klog.V(2).Infof("caldav: QueryCalendarObjects store returned %d event(s)", len(events))
	objects := make([]extcaldav.CalendarObject, 0, len(events))
	for _, e := range events {
		objects = append(objects, a.eventToObject(e))
	}
	filtered, err := extcaldav.Filter(query, objects)
	if err != nil {
		return nil, err
	}
	klog.V(2).Infof("caldav: QueryCalendarObjects after filter → %d event(s)", len(filtered))
	return filtered, nil
}

// PutCalendarObject creates or updates an event from an iCal payload.
func (a *storeAdapter) PutCalendarObject(ctx context.Context, path string, cal *ical.Calendar, opts *extcaldav.PutCalendarObjectOptions) (*extcaldav.CalendarObject, error) {
	calID, eventID := a.eventIDFromPath(path)
	if calID == "" || eventID == "" {
		return nil, webdav.NewHTTPError(http.StatusBadRequest, fmt.Errorf("invalid path: %s", path))
	}
	e, err := a.icalToEvent(calID, eventID, cal)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusBadRequest, err)
	}
	if opts != nil {
		// If-None-Match: * → the client wants a create, fail if event exists.
		if opts.IfNoneMatch.IsSet() {
			if _, getErr := a.store.GetEvent(ctx, calID, e.ID); getErr == nil {
				return nil, webdav.NewHTTPError(http.StatusPreconditionFailed,
					fmt.Errorf("calendar object already exists"))
			}
		}
		// If-Match: etag → the client sends its current version; store checks it.
		if opts.IfMatch.IsSet() && !opts.IfMatch.IsWildcard() {
			if etag, etagErr := opts.IfMatch.ETag(); etagErr == nil {
				e.ETag = etag
			}
		}
	}
	if err := a.store.PutEvent(ctx, e); err != nil {
		switch {
		case errors.Is(err, calstore.ErrConflict):
			return nil, webdav.NewHTTPError(http.StatusPreconditionFailed, err)
		case errors.Is(err, calstore.ErrNotFound):
			return nil, webdav.NewHTTPError(http.StatusNotFound, err)
		case errors.Is(err, calstore.ErrReadOnly):
			return nil, webdav.NewHTTPError(http.StatusForbidden, err)
		default:
			return nil, err
		}
	}
	// Re-fetch to return the server-assigned ETag.
	updated, err := a.store.GetEvent(ctx, calID, e.ID)
	if err != nil {
		return nil, err
	}
	obj := a.eventToObject(updated)
	return &obj, nil
}

// DeleteCalendarObject removes an event from the store.
func (a *storeAdapter) DeleteCalendarObject(ctx context.Context, path string) error {
	calID, eventID := a.eventIDFromPath(path)
	if calID == "" || eventID == "" {
		return webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("invalid path: %s", path))
	}
	// Pass empty etag: the library already validated If-Match via GetCalendarObject.
	if err := a.store.DeleteEvent(ctx, calID, eventID, ""); err != nil {
		if errors.Is(err, calstore.ErrNotFound) {
			return webdav.NewHTTPError(http.StatusNotFound, err)
		}
		if errors.Is(err, calstore.ErrReadOnly) {
			return webdav.NewHTTPError(http.StatusForbidden, err)
		}
		return err
	}
	return nil
}

// ── Conversion helpers ────────────────────────────────────────────────────────

// eventToObject converts an internal Event to a caldav.CalendarObject containing
// a proper ical.Calendar with a VEVENT component.
func (a *storeAdapter) eventToObject(e *calstore.Event) extcaldav.CalendarObject {
	cal := ical.NewCalendar()
	// NewCalendar already sets VERSION and PRODID via NewComponent; override them.
	cal.Props.SetText(ical.PropProductID, "-//schedio//schedio//EN")
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropCalendarScale, "GREGORIAN")

	vevent := ical.NewComponent(ical.CompEvent)

	// DTSTAMP is required by RFC 5545 §3.8.7.2 and enforced by the go-ical
	// encoder (exactlyOneProps).  Use the event's last-modification time so the
	// value is stable across requests; fall back to now only when unset.
	dtstamp := e.Modified
	if dtstamp.IsZero() {
		dtstamp = time.Now().UTC()
	}
	vevent.Props.SetDateTime(ical.PropDateTimeStamp, dtstamp)

	vevent.Props.SetText(ical.PropUID, e.ID)
	vevent.Props.SetText(ical.PropSummary, e.Summary)

	if e.AllDay {
		vevent.Props.SetDate(ical.PropDateTimeStart, e.Start)
		vevent.Props.SetDate(ical.PropDateTimeEnd, e.End)
	} else {
		vevent.Props.SetDateTime(ical.PropDateTimeStart, e.Start)
		vevent.Props.SetDateTime(ical.PropDateTimeEnd, e.End)
	}
	if e.Description != "" {
		vevent.Props.SetText(ical.PropDescription, e.Description)
	}
	if e.Location != "" {
		vevent.Props.SetText(ical.PropLocation, e.Location)
	}
	if string(e.Status) != "" {
		vevent.Props.SetText(ical.PropStatus, string(e.Status))
	}
	if string(e.Opacity) != "" {
		vevent.Props.SetText(ical.PropTransparency, string(e.Opacity))
	} else {
		vevent.Props.SetText(ical.PropTransparency, string(calstore.OpacityOpaque))
	}
	if !e.Created.IsZero() {
		vevent.Props.SetDateTime(ical.PropCreated, e.Created)
	}
	if !e.Modified.IsZero() {
		vevent.Props.SetDateTime(ical.PropLastModified, e.Modified)
	}

	seqProp := ical.NewProp(ical.PropSequence)
	seqProp.Value = strconv.Itoa(e.Sequence)
	vevent.Props.Set(seqProp)

	if e.Organizer.Email != "" {
		prop := ical.NewProp(ical.PropOrganizer)
		prop.Value = "mailto:" + e.Organizer.Email
		if e.Organizer.Name != "" {
			prop.Params.Set(ical.ParamCommonName, e.Organizer.Name)
		}
		vevent.Props.Set(prop)
	}
	for _, att := range e.Attendees {
		prop := ical.NewProp(ical.PropAttendee)
		prop.Value = "mailto:" + att.Email
		if att.Name != "" {
			prop.Params.Set(ical.ParamCommonName, att.Name)
		}
		if att.Status != "" {
			prop.Params.Set(ical.ParamParticipationStatus, string(att.Status))
		}
		if att.RSVP {
			prop.Params.Set(ical.ParamRSVP, "TRUE")
		}
		vevent.Props.Add(prop) // Add (not Set) to support multiple attendees
	}

	cal.Children = append(cal.Children, vevent)

	return extcaldav.CalendarObject{
		Path:    a.pathForEvent(e.CalendarID, e.ID),
		ModTime: e.Modified,
		ETag:    e.ETag,
		Data:    cal,
	}
}

// icalToEvent converts an *ical.Calendar received via PUT into an internal Event.
func (a *storeAdapter) icalToEvent(calID, fallbackEventID string, cal *ical.Calendar) (*calstore.Event, error) {
	events := cal.Events()
	if len(events) == 0 {
		return nil, fmt.Errorf("no VEVENT component in calendar data")
	}
	iev := events[0]

	e := &calstore.Event{
		CalendarID: calID,
		Status:     calstore.StatusConfirmed,
	}

	uid, _ := iev.Props.Text(ical.PropUID)
	if uid != "" {
		e.ID = uid
	} else {
		e.ID = fallbackEventID
	}

	e.Summary, _ = iev.Props.Text(ical.PropSummary)
	e.Description, _ = iev.Props.Text(ical.PropDescription)
	e.Location, _ = iev.Props.Text(ical.PropLocation)
	e.Opacity = calstore.OpacityOpaque
if transp, _ := iev.Props.Text(ical.PropTransparency); transp != "" {
		e.Opacity = calstore.EventOpacity(transp)
	} else if opacity, _ := iev.Props.Text("OPACITY"); opacity != "" {
		e.Opacity = calstore.EventOpacity(opacity)
	}

	// Detect all-day events: DTSTART uses VALUE=DATE instead of DATE-TIME.
	if dtstart := iev.Props.Get(ical.PropDateTimeStart); dtstart != nil {
		if dtstart.Params.Get(ical.ParamValue) == string(ical.ValueDate) {
			e.AllDay = true
		}
	}

	var err error
	e.Start, err = iev.DateTimeStart(time.UTC)
	if err != nil {
		e.Start = time.Now().UTC()
	}
	e.End, err = iev.DateTimeEnd(time.UTC)
	if err != nil {
		e.End = e.Start.Add(time.Hour)
	}

	if status, _ := iev.Props.Text(ical.PropStatus); status != "" {
		e.Status = calstore.EventStatus(status)
	}
	if seq := iev.Props.Get(ical.PropSequence); seq != nil {
		e.Sequence, _ = seq.Int()
	}

	return e, nil
}

// extractTimeRange finds the time range from a CalendarQuery's VEVENT CompFilter
// so the store can pre-filter events before the library applies full filtering.
func extractTimeRange(query *extcaldav.CalendarQuery) (start, end time.Time) {
	if query == nil {
		return
	}
	for _, comp := range query.CompFilter.Comps {
		if comp.Name == ical.CompEvent {
			return comp.Start, comp.End
		}
	}
	return
}

// encodeCalendar serialises an *ical.Calendar to a string; used for debug logging.
func encodeCalendar(cal *ical.Calendar) (string, error) {
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return "", err
	}
	return buf.String(), nil
}
