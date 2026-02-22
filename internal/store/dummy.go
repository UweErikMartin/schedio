package store

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// DummyStore is a read-only CalendarStore that generates random events for the
// current calendar month. It is intended for development and demonstration only.
//
// Events are generated once per month: the first call to ListEvents (or
// GetEvent) for a given month triggers generation; subsequent calls within
// the same month reuse the cached events.
type DummyStore struct {
	mu       sync.RWMutex
	calendar Calendar
	events   map[string]*Event // eventID → *Event, refreshed each month
	month    time.Time         // start-of-month of the cached events (UTC)
	ctag     string            // changes whenever events change (or month rolls)
}

// NewDummyStore creates a DummyStore pre-seeded with a single dummy calendar.
func NewDummyStore() *DummyStore {
	return &DummyStore{
		calendar: Calendar{
			ID:          "dummy",
			Name:        "Dummy Calendar",
			Description: "Auto-generated events for the current month",
			Color:       "#4A90D9",
			Timezone:    "UTC",
		},
		events: make(map[string]*Event),
		ctag:   newToken(),
	}
}

// ── CalendarStore implementation ─────────────────────────────────────────────

func (d *DummyStore) GetCalendar(_ context.Context, id string) (*Calendar, error) {
	if id != d.calendar.ID {
		return nil, ErrNotFound
	}
	cp := d.calendar
	return &cp, nil
}

func (d *DummyStore) ListCalendars(_ context.Context) ([]*Calendar, error) {
	cp := d.calendar
	return []*Calendar{&cp}, nil
}

func (d *DummyStore) GetEvent(ctx context.Context, calendarID, eventID string) (*Event, error) {
	if calendarID != d.calendar.ID {
		return nil, ErrNotFound
	}
	d.ensureEventsForCurrentMonth(ctx)

	d.mu.RLock()
	defer d.mu.RUnlock()
	e, ok := d.events[eventID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *e
	cp.Attendees = append([]Attendee(nil), e.Attendees...)
	return &cp, nil
}

func (d *DummyStore) ListEvents(ctx context.Context, calendarID string, start, end time.Time) ([]*Event, error) {
	if calendarID != d.calendar.ID {
		return nil, ErrNotFound
	}
	d.ensureEventsForCurrentMonth(ctx)

	d.mu.RLock()
	defer d.mu.RUnlock()
	var result []*Event
	for _, e := range d.events {
		if !start.IsZero() && e.End.Before(start) {
			continue
		}
		if !end.IsZero() && e.Start.After(end) {
			continue
		}
		cp := *e
		cp.Attendees = append([]Attendee(nil), e.Attendees...)
		result = append(result, &cp)
	}
	return result, nil
}

// PutEvent updates or creates an event in the current month snapshot.
func (d *DummyStore) PutEvent(ctx context.Context, event *Event) error {
	if event.CalendarID != d.calendar.ID {
		return ErrNotFound
	}

	d.ensureEventsForCurrentMonth(ctx)

	d.mu.Lock()
	defer d.mu.Unlock()

	existing, exists := d.events[event.ID]
	if exists && event.ETag != "" && event.ETag != existing.ETag {
		return ErrConflict
	}

	now := time.Now().UTC()
	if !exists {
		event.Created = now
		event.Sequence = 0
	} else {
		event.Created = existing.Created
		event.Sequence = existing.Sequence + 1
	}
	event.Modified = now
	event.ETag = newToken()

	cp := *event
	cp.Attendees = append([]Attendee(nil), event.Attendees...)
	d.events[event.ID] = &cp
	d.ctag = newToken()
	return nil
}

// DeleteEvent removes an event from the current month snapshot.
func (d *DummyStore) DeleteEvent(ctx context.Context, calendarID, eventID, etag string) error {
	if calendarID != d.calendar.ID {
		return ErrNotFound
	}

	d.ensureEventsForCurrentMonth(ctx)

	d.mu.Lock()
	defer d.mu.Unlock()

	existing, ok := d.events[eventID]
	if !ok {
		return ErrNotFound
	}
	if etag != "" && etag != existing.ETag {
		return ErrConflict
	}

	delete(d.events, eventID)
	d.ctag = newToken()
	return nil
}

func (d *DummyStore) CTag(_ context.Context, calendarID string) (string, error) {
	if calendarID != d.calendar.ID {
		return "", ErrNotFound
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.ctag == "" {
		return d.month.Format("200601"), nil
	}
	return d.ctag, nil
}

// ── Event generation ─────────────────────────────────────────────────────────

// ensureEventsForCurrentMonth regenerates events when the cached month differs
// from the current UTC month.
func (d *DummyStore) ensureEventsForCurrentMonth(_ context.Context) {
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	d.mu.RLock()
	upToDate := d.month.Equal(monthStart)
	d.mu.RUnlock()
	if upToDate {
		return
	}

	events := generateDummyEvents(d.calendar.ID, monthStart)

	d.mu.Lock()
	d.month = monthStart
	d.events = events
	d.ctag = newToken()
	d.mu.Unlock()
}

var (
	dummySummaries = []string{
		"Team standup",
		"Sprint planning",
		"Code review",
		"1:1 with manager",
		"Architecture discussion",
		"Release retrospective",
		"Product demo",
		"Customer call",
		"Design review",
		"Incident post-mortem",
		"Quarterly review",
		"Lunch with team",
		"Training session",
		"Budget meeting",
		"Hiring interview",
	}

	dummyLocations = []string{
		"Conference room A",
		"Conference room B",
		"Video call",
		"Office kitchen",
		"",
		"",
		"",
	}

	dummyAttendeeNames = []string{
		"Alice Weber", "Bob Müller", "Carol Schmidt", "Dave Fischer",
		"Eva Bauer", "Frank Zimmermann", "Grace Hoffmann", "Hans Klein",
	}

	dummyStatuses = []EventStatus{
		StatusConfirmed, StatusConfirmed, StatusConfirmed,
		StatusTentative, StatusCancelled,
	}

	dummyOpacities = []EventOpacity{
		OpacityOpaque, OpacityOpaque, OpacityOpaque,
		OpacityTransparent,
	}
)

// generateDummyEvents creates 8–15 random events spread across monthStart's
// calendar month. Each event is 30–90 minutes long.
func generateDummyEvents(calendarID string, monthStart time.Time) map[string]*Event {
	rng := rand.New(rand.NewSource(monthStart.Unix())) // deterministic per month

	daysInMonth := daysIn(monthStart.Year(), monthStart.Month())
	count := rng.Intn(8) + 8 // 8..15 events

	events := make(map[string]*Event, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("dummy-%s-%02d", monthStart.Format("200601"), i+1)

		day := rng.Intn(daysInMonth) + 1
		hour := rng.Intn(9) + 8            // 08:00–16:00
		minute := (rng.Intn(4)) * 15       // 0, 15, 30, 45
		durationMin := rng.Intn(5)*15 + 30 // 30, 45, 60, 75, 90 minutes

		start := time.Date(monthStart.Year(), monthStart.Month(), day, hour, minute, 0, 0, time.UTC)
		end := start.Add(time.Duration(durationMin) * time.Minute)

		summary := dummySummaries[rng.Intn(len(dummySummaries))]
		location := dummyLocations[rng.Intn(len(dummyLocations))]
		status := dummyStatuses[rng.Intn(len(dummyStatuses))]
		opacity := dummyOpacities[rng.Intn(len(dummyOpacities))]

		organizer := Attendee{
			Email:  "organizer@example.com",
			Name:   "Organizer",
			Status: PartStatAccepted,
		}

		numAttendees := rng.Intn(3) + 1 // 1–3 attendees
		attendees := make([]Attendee, 0, numAttendees)
		perm := rng.Perm(len(dummyAttendeeNames))
		for j := 0; j < numAttendees && j < len(perm); j++ {
			name := dummyAttendeeNames[perm[j]]
			statuses := []ParticipationStatus{PartStatAccepted, PartStatDeclined, PartStatNeedsAction, PartStatTentative}
			attendees = append(attendees, Attendee{
				Email:  fmt.Sprintf("%s@example.com", sanitizeName(name)),
				Name:   name,
				RSVP:   true,
				Status: statuses[rng.Intn(len(statuses))],
			})
		}

		etag := fmt.Sprintf("%s-%02d", monthStart.Format("200601"), i+1)

		events[id] = &Event{
			ID:          id,
			CalendarID:  calendarID,
			Summary:     summary,
			Description: fmt.Sprintf("Auto-generated event #%d for %s", i+1, monthStart.Format("January 2006")),
			Location:    location,
			Start:       start,
			End:         end,
			AllDay:      false,
			Opacity:     opacity,
			Status:      status,
			Organizer:   organizer,
			Attendees:   attendees,
			Created:     monthStart,
			Modified:    monthStart,
			Sequence:    0,
			ETag:        etag,
		}
	}
	return events
}

// daysIn returns the number of days in the given month/year.
func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// sanitizeName lowercases a display name and replaces spaces/umlauts with
// ASCII equivalents suitable for use in an e-mail local part.
func sanitizeName(name string) string {
	replacer := strings.NewReplacer(
		" ", ".",
		"ä", "ae", "ö", "oe", "ü", "ue",
		"Ä", "ae", "Ö", "oe", "Ü", "ue",
		"ß", "ss",
	)
	return strings.ToLower(replacer.Replace(name))
}
