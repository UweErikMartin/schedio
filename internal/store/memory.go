package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// MemoryStore is a thread-safe, in-memory CalendarStore.
// It is pre-seeded with a single default calendar and is intended for
// development and testing. Replace it with a persistent implementation
// in production.
type MemoryStore struct {
	mu        sync.RWMutex
	calendars map[string]*Calendar
	events    map[string]map[string]*Event // calendarID → eventID → *Event
	ctags     map[string]string            // calendarID → opaque ctag
}

// NewMemoryStore creates an initialised MemoryStore with one default calendar.
func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		calendars: make(map[string]*Calendar),
		events:    make(map[string]map[string]*Event),
		ctags:     make(map[string]string),
	}
	defaultCal := &Calendar{
		ID:          "default",
		Name:        "Default Calendar",
		Description: "schedio default calendar",
		Timezone:    "UTC",
	}
	s.calendars[defaultCal.ID] = defaultCal
	s.events[defaultCal.ID] = make(map[string]*Event)
	s.ctags[defaultCal.ID] = newToken()
	return s
}

// GetCalendar implements CalendarStore.
func (s *MemoryStore) GetCalendar(_ context.Context, id string) (*Calendar, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cal, ok := s.calendars[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *cal
	return &cp, nil
}

// ListCalendars implements CalendarStore.
func (s *MemoryStore) ListCalendars(_ context.Context) ([]*Calendar, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Calendar, 0, len(s.calendars))
	for _, cal := range s.calendars {
		cp := *cal
		result = append(result, &cp)
	}
	return result, nil
}

// GetEvent implements CalendarStore.
func (s *MemoryStore) GetEvent(_ context.Context, calendarID, eventID string) (*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	evts, ok := s.events[calendarID]
	if !ok {
		return nil, ErrNotFound
	}
	e, ok := evts[eventID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *e
	cp.Attendees = append([]Attendee(nil), e.Attendees...)
	return &cp, nil
}

// ListEvents implements CalendarStore.
// Events whose time range overlaps [start, end] are returned.
// Zero start/end means no lower/upper bound.
func (s *MemoryStore) ListEvents(_ context.Context, calendarID string, start, end time.Time) ([]*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	evts, ok := s.events[calendarID]
	if !ok {
		return nil, ErrNotFound
	}
	var result []*Event
	for _, e := range evts {
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

// PutEvent implements CalendarStore.
func (s *MemoryStore) PutEvent(_ context.Context, event *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	evts, ok := s.events[event.CalendarID]
	if !ok {
		return ErrNotFound
	}
	existing, exists := evts[event.ID]
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
	evts[event.ID] = &cp
	s.ctags[event.CalendarID] = newToken()
	return nil
}

// DeleteEvent implements CalendarStore.
func (s *MemoryStore) DeleteEvent(_ context.Context, calendarID, eventID, etag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	evts, ok := s.events[calendarID]
	if !ok {
		return ErrNotFound
	}
	existing, ok := evts[eventID]
	if !ok {
		return ErrNotFound
	}
	if etag != "" && etag != existing.ETag {
		return ErrConflict
	}
	delete(evts, eventID)
	s.ctags[calendarID] = newToken()
	return nil
}

// CTag implements CalendarStore.
func (s *MemoryStore) CTag(_ context.Context, calendarID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctag, ok := s.ctags[calendarID]
	if !ok {
		return "", ErrNotFound
	}
	return ctag, nil
}

// newToken returns a random hex string suitable for use as ETag or CTag.
func newToken() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp if crypto/rand is unavailable.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
