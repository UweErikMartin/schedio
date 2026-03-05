package store

import (
	"context"
	"sort"
	"time"

	rrulego "github.com/teambition/rrule-go"
	"k8s.io/klog/v2"
)

// Compile-time assertion that *MemoryStore implements DomainStore.
var _ DomainStore = (*MemoryStore)(nil)

// ── Staff / Users ─────────────────────────────────────────────────────────────

// SyncUsers implements DomainStore.
// It replaces the stored user set with the provided list. For any email that
// already exists, the stored password hash is preserved when the caller
// supplies an empty PasswordHash.
func (s *MemoryStore) SyncUsers(_ context.Context, users []*User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Build lookup of existing password hashes by email.
	existing := make(map[string]string, len(s.users))
	for _, u := range s.users {
		existing[u.Email] = u.PasswordHash
	}
	s.users = make(map[string]*User, len(users))
	s.userEmails = make(map[string]string, len(users))
	now := time.Now().UTC()
	for _, u := range users {
		cp := *u
		if cp.PasswordHash == "" {
			cp.PasswordHash = existing[cp.Email]
		}
		if cp.CreatedAt.IsZero() {
			cp.CreatedAt = now
		}
		cp.UpdatedAt = now
		s.users[cp.ID] = &cp
		s.userEmails[cp.Email] = cp.ID
	}
	return nil
}

// ListStaff implements DomainStore.
func (s *MemoryStore) ListStaff(_ context.Context) ([]*Staff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Staff
	for _, u := range s.users {
		if u.Role == UserRoleStaff {
			cp := *u
			result = append(result, &cp)
		}
	}
	return result, nil
}

// GetStaff implements DomainStore.
func (s *MemoryStore) GetStaff(_ context.Context, id string) (*Staff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok || u.Role != UserRoleStaff {
		return nil, ErrNotFound
	}
	cp := *u
	return &cp, nil
}

// GetUserByEmail implements DomainStore.
func (s *MemoryStore) GetUserByEmail(_ context.Context, email string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.userEmails[email]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *s.users[id]
	return &cp, nil
}

// ── Services ──────────────────────────────────────────────────────────────────

// ListServices implements DomainStore.
func (s *MemoryStore) ListServices(_ context.Context) ([]*Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Service, 0, len(s.services))
	for _, svc := range s.services {
		cp := *svc
		result = append(result, &cp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// GetService implements DomainStore.
func (s *MemoryStore) GetService(_ context.Context, id string) (*Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	svc, ok := s.services[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *svc
	return &cp, nil
}

// CreateService implements DomainStore.
func (s *MemoryStore) CreateService(_ context.Context, svc *Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	cp := *svc
	cp.CreatedAt = now
	cp.UpdatedAt = now
	s.services[cp.ID] = &cp
	return nil
}

// UpdateService implements DomainStore.
func (s *MemoryStore) UpdateService(_ context.Context, svc *Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.services[svc.ID]
	if !ok {
		return ErrNotFound
	}
	cp := *svc
	cp.CreatedAt = existing.CreatedAt
	cp.UpdatedAt = time.Now().UTC()
	s.services[cp.ID] = &cp
	return nil
}

// DeleteService implements DomainStore.
// Returns ErrConflict when active (non-cancelled) bookings reference the service.
func (s *MemoryStore) DeleteService(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.services[id]; !ok {
		return ErrNotFound
	}
	for _, b := range s.bookings {
		if b.ServiceID == id && b.State != BookingStateCancelled && b.State != BookingStateNoShow {
			return ErrConflict
		}
	}
	delete(s.services, id)
	return nil
}

// ── Availability ─────────────────────────────────────────────────────────────────────────

// compositeAvailabilityKey returns the map key for an Availability record. Series roots and
// single events are keyed by CalDAVUID alone. Override records are keyed by
// CalDAVUID + ":" + recurrenceID in RFC3339 format so they can coexist with
// their series root in the same map without collision.
func compositeAvailabilityKey(uid string, recurrenceID time.Time) string {
	if recurrenceID.IsZero() {
		return uid
	}
	return uid + ":" + recurrenceID.UTC().Format(time.RFC3339)
}

// isExcluded reports whether occ matches any time in exDates (UTC comparison,
// truncated to second precision to tolerate minor formatting differences).
func isExcluded(occ time.Time, exDates []time.Time) bool {
	for _, ex := range exDates {
		if occ.UTC().Truncate(time.Second).Equal(ex.UTC().Truncate(time.Second)) {
			return true
		}
	}
	return false
}

// ListAvailability implements DomainStore.
// Returns availability records for userID whose window overlaps [start, end].
// Zero start/end means no bound.
// Recurring series roots are expanded into individual occurrences; EXDATE
// occurrences and occurrences whose date has a stored override record are
// skipped (the override record's own window is the authoritative availability
// window for that occurrence). Override records are themselves included at
// their actual StartAt/EndAt so that moved occurrences appear at the
// rescheduled time rather than the original series-root time.
func (s *MemoryStore) ListAvailability(_ context.Context, userID string, start, end time.Time) ([]*Availability, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build a per-series set of override dates so series-root expansion can
	// skip occurrences whose authoritative window lives in an override record.
	overrideDatesByUID := make(map[string]map[int64]bool)
	for _, t := range s.availability {
		if t.UserID != userID || t.RecurrenceID.IsZero() {
			continue
		}
		if overrideDatesByUID[t.CalDAVUID] == nil {
			overrideDatesByUID[t.CalDAVUID] = make(map[int64]bool)
		}
		overrideDatesByUID[t.CalDAVUID][t.RecurrenceID.UTC().Truncate(time.Second).Unix()] = true
	}

	var result []*Availability
	for _, t := range s.availability {
		if t.UserID != userID {
			continue
		}
		if !t.RecurrenceID.IsZero() {
			// Override record: include it at its actual window so that moved
			// occurrences appear at the correct rescheduled time.
			if !start.IsZero() && t.EndAt.Before(start) {
				continue
			}
			if !end.IsZero() && t.StartAt.After(end) {
				continue
			}
			cp := *t
			cp.ExDates = nil // overrides carry no EXDATEs of their own
			result = append(result, &cp)
			continue
		}
		if t.RRule == "" {
			// Non-recurring: simple overlap check against stored window.
			if !start.IsZero() && t.EndAt.Before(start) {
				continue
			}
			if !end.IsZero() && t.StartAt.After(end) {
				continue
			}
			cp := *t
			cp.ExDates = append([]time.Time(nil), t.ExDates...)
			result = append(result, &cp)
			continue
		}
		// Recurring: expand the RRULE and return one synthetic Availability entry
		// per occurrence that falls within [start, end), skipping EXDATEs and
		// occurrences that have a stored override record.
		opts, err := rrulego.StrToROption(t.RRule)
		if err != nil {
			klog.Warningf("store: availability record %q has invalid RRule %q: %v — skipped", t.CalDAVUID, t.RRule, err)
			continue
		}
		opts.Dtstart = t.StartAt
		rr, err := rrulego.NewRRule(*opts)
		if err != nil {
			klog.Warningf("store: availability record %q: cannot build RRule: %v — skipped", t.CalDAVUID, err)
			continue
		}
		duration := t.EndAt.Sub(t.StartAt)
		// Determine the effective query bounds.
		qStart := t.StartAt
		if !start.IsZero() {
			qStart = start
		}
		qEnd := time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
		if !end.IsZero() {
			qEnd = end
		}
		overrideDates := overrideDatesByUID[t.CalDAVUID]
		// Between(after, before, inc) with inc=true returns start <= occ <= end.
		for _, occ := range rr.Between(qStart, qEnd, true) {
			// Skip occurrences excluded by EXDATE.
			if isExcluded(occ, t.ExDates) {
				continue
			}
			// Skip occurrences whose window is governed by a stored override record.
			if overrideDates[occ.UTC().Truncate(time.Second).Unix()] {
				continue
			}
			cp := *t
			cp.ExDates = append([]time.Time(nil), t.ExDates...)
			cp.StartAt = occ
			cp.EndAt = occ.Add(duration)
			result = append(result, &cp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartAt.Before(result[j].StartAt)
	})
	return result, nil
}

// ListRawAvailability implements DomainStore.
// Returns the unexpanded raw availability records for userID sorted by StartAt.
// Recurring records are returned once (with RRule intact), not expanded.
// Override records (RecurrenceID != zero) are excluded because CalDAV clients
// receive overrides embedded inside the same .ics as the series root — they
// should not appear as separate calendar objects in directory listings.
func (s *MemoryStore) ListRawAvailability(_ context.Context, userID string) ([]*Availability, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Availability
	for _, t := range s.availability {
		if t.UserID != userID {
			continue
		}
		// Overrides share the CalDAVUID with the series root; they must not
		// appear as separate .ics entries in calendar directory listings.
		if !t.RecurrenceID.IsZero() {
			continue
		}
		cp := *t
		cp.ExDates = append([]time.Time(nil), t.ExDates...)
		result = append(result, &cp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartAt.Before(result[j].StartAt)
	})
	return result, nil
}

// GetAvailability implements DomainStore.
// Returns the series root (or single-event) record identified by its CalDAVUID.
// Overrides are stored under composite keys and are not returned by this method.
func (s *MemoryStore) GetAvailability(_ context.Context, userID, uid string) (*Availability, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.availability[compositeAvailabilityKey(uid, time.Time{})]
	if !ok || t.UserID != userID {
		return nil, ErrNotFound
	}
	cp := *t
	cp.ExDates = append([]time.Time(nil), t.ExDates...)
	return &cp, nil
}

// UpsertAvailability implements DomainStore.
// Creates the record when the composite key (CalDAVUID, RecurrenceID) is new;
// replaces the existing record otherwise. ExDates are preserved (deep-copied).
//
// Returns ErrConflict when the record's effective time windows (its own
// window for single events and overrides, or all non-EXDATE RRULE occurrences
// for series roots) overlap with any existing availability record of the same user.
// The record being replaced (same composite key) is excluded from the check.
func (s *MemoryStore) UpsertAvailability(_ context.Context, t *Availability) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := compositeAvailabilityKey(t.CalDAVUID, t.RecurrenceID)

	// Check for overlap against all existing availability records of the same user,
	// excluding the record we are about to replace.
	if err := s.checkAvailabilityOverlapLocked(t, key); err != nil {
		return err
	}

	now := time.Now().UTC()
	cp := *t
	if existing, ok := s.availability[key]; ok {
		cp.CreatedAt = existing.CreatedAt
	} else {
		cp.CreatedAt = now
	}
	cp.UpdatedAt = now
	cp.CalDAVETag = newToken()
	cp.ExDates = append([]time.Time(nil), t.ExDates...)
	s.availability[key] = &cp
	return nil
}

// overlapHorizon is the upper bound used when expanding infinite recurring
// series for the purpose of overlap detection (3 years from now).
var overlapHorizon = func() time.Time {
	return time.Now().UTC().AddDate(3, 0, 0)
}

// availabilityWindows returns the effective [start, end) windows for t, up to
// horizon. For a series root, RRULE occurrences are expanded with EXDATE and
// any currently stored override dates excluded so that the override record's
// own window is the authoritative window for that occurrence.
//
// extraExclude is an additional occurrence date to exclude from expansion
// (beyond stored overrides). It is used by checkAvailabilityOverlapLocked to
// exclude the incoming override's RecurrenceID from the existing series root's
// expansion: the override has not yet been stored — but once it is, the series
// root will no longer own that occurrence window.
//
// Must be called with s.mu held (read or write).
func (s *MemoryStore) availabilityWindowsLocked(t *Availability, horizon time.Time, extraExclude time.Time) [][2]time.Time {
	if !t.RecurrenceID.IsZero() || t.RRule == "" {
		// Single event or override: exactly one window.
		return [][2]time.Time{{t.StartAt, t.EndAt}}
	}
	// Series root: expand RRULE occurrences up to horizon, skipping EXDATEs
	// and dates that already have an override stored (those are accounted for
	// by the override record's own window).
	overrideDates := make(map[int64]bool) // Unix seconds → true
	for _, rec := range s.availability {
		if rec.CalDAVUID == t.CalDAVUID && rec.UserID == t.UserID && !rec.RecurrenceID.IsZero() {
			overrideDates[rec.RecurrenceID.UTC().Truncate(time.Second).Unix()] = true
		}
	}
	// Also exclude a pending incoming override that is not yet stored.
	if !extraExclude.IsZero() {
		overrideDates[extraExclude.UTC().Truncate(time.Second).Unix()] = true
	}

	opts, err := rrulego.StrToROption(t.RRule)
	if err != nil {
		return nil
	}
	opts.Dtstart = t.StartAt
	rr, err := rrulego.NewRRule(*opts)
	if err != nil {
		return nil
	}
	duration := t.EndAt.Sub(t.StartAt)
	var windows [][2]time.Time
	for _, occ := range rr.Between(t.StartAt, horizon, true) {
		if isExcluded(occ, t.ExDates) {
			continue
		}
		if overrideDates[occ.UTC().Truncate(time.Second).Unix()] {
			continue
		}
		windows = append(windows, [2]time.Time{occ, occ.Add(duration)})
	}
	return windows
}

// checkAvailabilityOverlapLocked returns ErrConflict when the incoming record t
// overlaps with any existing availability record for the same user, excluding the record
// at excludeKey (which is the record being replaced in an update).
// Must be called with s.mu held for writing.
func (s *MemoryStore) checkAvailabilityOverlapLocked(t *Availability, excludeKey string) error {
	horizon := overlapHorizon()
	newWindows := s.availabilityWindowsLocked(t, horizon, time.Time{})
	if len(newWindows) == 0 {
		return nil
	}
	for existKey, existing := range s.availability {
		if existing.UserID != t.UserID {
			continue
		}
		if existKey == excludeKey {
			continue
		}
		// When the incoming record is an override, exclude its RecurrenceID from
		// the existing series root's expansion. The override replaces that
		// occurrence — once stored, the series root no longer owns that window.
		var pendingOD time.Time
		if !t.RecurrenceID.IsZero() && existing.CalDAVUID == t.CalDAVUID && existing.RecurrenceID.IsZero() {
			pendingOD = t.RecurrenceID
		}
		for _, ew := range s.availabilityWindowsLocked(existing, horizon, pendingOD) {
			for _, nw := range newWindows {
				// Windows overlap when: existStart < newEnd AND newStart < existEnd
				if ew[0].Before(nw[1]) && nw[0].Before(ew[1]) {
					klog.V(2).Infof("store: availability overlap: uid=%q [%v,%v) conflicts with uid=%q [%v,%v)",
						t.CalDAVUID, nw[0], nw[1], existing.CalDAVUID, ew[0], ew[1])
					return ErrConflict
				}
			}
		}
	}
	return nil
}

// DeleteAvailability implements DomainStore.
// Removes the series root or single-event identified by CalDAVUID.
// Override records for the same CalDAVUID are NOT removed; call
// DeleteAvailabilityOverrides first when deleting a recurring series (CDV-AVAIL-2 Case B).
func (s *MemoryStore) DeleteAvailability(_ context.Context, userID, uid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeAvailabilityKey(uid, time.Time{})
	t, ok := s.availability[key]
	if !ok || t.UserID != userID {
		return ErrNotFound
	}
	delete(s.availability, key)
	return nil
}

// DeleteAvailabilityOverride implements DomainStore.
// Removes the single override record identified by (userID, CalDAVUID, recurrenceID).
func (s *MemoryStore) DeleteAvailabilityOverride(_ context.Context, userID, uid string, recurrenceID time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeAvailabilityKey(uid, recurrenceID)
	t, ok := s.availability[key]
	if !ok || t.UserID != userID {
		return ErrNotFound
	}
	delete(s.availability, key)
	return nil
}

// DeleteAvailabilityOverrides implements DomainStore.
// Removes all override records for the given series CalDAVUID (i.e. all
// Availability records with the same CalDAVUID whose RecurrenceID is non-zero).
// Returns nil when no overrides exist.
func (s *MemoryStore) DeleteAvailabilityOverrides(_ context.Context, userID, uid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, t := range s.availability {
		if t.CalDAVUID == uid && t.UserID == userID && !t.RecurrenceID.IsZero() {
			delete(s.availability, key)
		}
	}
	return nil
}

// ── Contacts ──────────────────────────────────────────────────────────────────

// GetOrCreateContact implements DomainStore.
func (s *MemoryStore) GetOrCreateContact(_ context.Context, email string, c *Contact) (*Contact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.emailIndex[email]; ok {
		existing := s.contacts[id]
		cp := *existing
		return &cp, nil
	}
	now := time.Now().UTC()
	nc := *c
	nc.ID = newUUID()
	nc.Email = email
	nc.CreatedAt = now
	nc.RetentionState = RetentionStateActive
	s.contacts[nc.ID] = &nc
	s.emailIndex[email] = nc.ID
	cp := nc
	return &cp, nil
}

// GetContact implements DomainStore.
func (s *MemoryStore) GetContact(_ context.Context, id string) (*Contact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.contacts[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *c
	return &cp, nil
}

// UpdateContactLastAppointment implements DomainStore.
func (s *MemoryStore) UpdateContactLastAppointment(_ context.Context, contactID string, appointmentEndAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contacts[contactID]
	if !ok {
		return ErrNotFound
	}
	if !appointmentEndAt.After(c.LastAppointmentEndAt) {
		return nil
	}
	c.LastAppointmentEndAt = appointmentEndAt
	c.BillingGenerated = false
	c.RetentionState = RetentionStateActive
	return nil
}

// ── Booking sessions ──────────────────────────────────────────────────────────

// CreateSession implements DomainStore.
func (s *MemoryStore) CreateSession(_ context.Context, bs *BookingSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *bs
	s.sessions[cp.ID] = &cp
	return nil
}

// GetSession implements DomainStore.
func (s *MemoryStore) GetSession(_ context.Context, id string) (*BookingSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bs, ok := s.sessions[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *bs
	return &cp, nil
}

// UpdateSession implements DomainStore.
func (s *MemoryStore) UpdateSession(_ context.Context, bs *BookingSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[bs.ID]; !ok {
		return ErrNotFound
	}
	cp := *bs
	s.sessions[cp.ID] = &cp
	return nil
}

// ListPendingSessions implements DomainStore.
// Returns sessions in state "submitted", oldest first.
func (s *MemoryStore) ListPendingSessions(_ context.Context) ([]*BookingSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*BookingSession
	for _, bs := range s.sessions {
		if bs.State == SessionStateSubmitted {
			cp := *bs
			result = append(result, &cp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].SubmittedAt.Before(result[j].SubmittedAt)
	})
	return result, nil
}

// ── Bookings ──────────────────────────────────────────────────────────────────

// CreateBooking implements DomainStore.
// It atomically checks for time-slot overlap and returns ErrConflict when one
// is found.
func (s *MemoryStore) CreateBooking(_ context.Context, b *Booking) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.bookings {
		if existing.UserID != b.UserID {
			continue
		}
		if existing.State == BookingStateCancelled || existing.State == BookingStateNoShow {
			continue
		}
		// Intervals overlap when: existing.StartAt < b.EndAt AND b.StartAt < existing.EndAt
		if existing.StartAt.Before(b.EndAt) && b.StartAt.Before(existing.EndAt) {
			return ErrConflict
		}
	}
	now := time.Now().UTC()
	cp := *b
	cp.CreatedAt = now
	cp.UpdatedAt = now
	s.bookings[cp.ID] = &cp
	return nil
}

// GetBooking implements DomainStore.
func (s *MemoryStore) GetBooking(_ context.Context, id string) (*Booking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.bookings[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *b
	return &cp, nil
}

// UpdateBooking implements DomainStore.
func (s *MemoryStore) UpdateBooking(_ context.Context, b *Booking) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bookings[b.ID]; !ok {
		return ErrNotFound
	}
	cp := *b
	cp.UpdatedAt = time.Now().UTC()
	s.bookings[cp.ID] = &cp
	return nil
}

// ListBookingsForSession implements DomainStore.
func (s *MemoryStore) ListBookingsForSession(_ context.Context, sessionID string) ([]*Booking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Booking
	for _, b := range s.bookings {
		if b.SessionID == sessionID {
			cp := *b
			result = append(result, &cp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartAt.Before(result[j].StartAt)
	})
	return result, nil
}

// ListBookingsForDay implements DomainStore.
// Returns all bookings whose StartAt falls on the given UTC calendar day.
func (s *MemoryStore) ListBookingsForDay(_ context.Context, date time.Time) ([]*Booking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	y, m, d := date.UTC().Date()
	var result []*Booking
	for _, b := range s.bookings {
		by, bm, bd := b.StartAt.UTC().Date()
		if by == y && bm == m && bd == d {
			cp := *b
			result = append(result, &cp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartAt.Before(result[j].StartAt)
	})
	return result, nil
}

// ListActiveBookingsInWindow implements DomainStore.
func (s *MemoryStore) ListActiveBookingsInWindow(_ context.Context, userID string, start, end time.Time) ([]*Booking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Booking
	for _, b := range s.bookings {
		if b.UserID != userID {
			continue
		}
		if b.State == BookingStateCancelled || b.State == BookingStateNoShow {
			continue
		}
		if b.StartAt.Before(end) && start.Before(b.EndAt) {
			cp := *b
			result = append(result, &cp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartAt.Before(result[j].StartAt)
	})
	return result, nil
}

// ListAllBookingsInWindow implements DomainStore.
// Returns all non-cancelled Bookings across all users whose time window
// overlaps [start, end]. Zero start/end means no bound.
func (s *MemoryStore) ListAllBookingsInWindow(_ context.Context, start, end time.Time) ([]*Booking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Booking
	for _, b := range s.bookings {
		if b.State == BookingStateCancelled || b.State == BookingStateNoShow {
			continue
		}
		if !start.IsZero() && !end.IsZero() {
			if !b.StartAt.Before(end) || !start.Before(b.EndAt) {
				continue
			}
		} else if !start.IsZero() && b.EndAt.Before(start) {
			continue
		} else if !end.IsZero() && b.StartAt.After(end) {
			continue
		}
		cp := *b
		result = append(result, &cp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartAt.Before(result[j].StartAt)
	})
	return result, nil
}

// ListBookingsForContact implements DomainStore.
func (s *MemoryStore) ListBookingsForContact(_ context.Context, contactID string) ([]*Booking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Booking
	for _, b := range s.bookings {
		if b.ContactID != contactID {
			continue
		}
		if b.State == BookingStateCancelled || b.State == BookingStateNoShow {
			continue
		}
		cp := *b
		result = append(result, &cp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartAt.Before(result[j].StartAt)
	})
	return result, nil
}

// ── Settings ──────────────────────────────────────────────────────────────────

// GetSettings implements DomainStore.
func (s *MemoryStore) GetSettings(_ context.Context) (*Settings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := s.settings
	return &cp, nil
}

// UpdateSettings implements DomainStore.
func (s *MemoryStore) UpdateSettings(_ context.Context, settings *Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = *settings
	// Sync the display name of the default calendar immediately so all
	// subsequent ListCalendars / GetCalendar calls reflect the change.
	if cal, ok := s.calendars["default"]; ok {
		if settings.DefaultCalendarName != "" {
			cal.Name = settings.DefaultCalendarName
		} else {
			cal.Name = "Booking-Calendar"
		}
	}
	return nil
}

// GetHMACSecret implements DomainStore.
func (s *MemoryStore) GetHMACSecret(_ context.Context) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.hmacSecret) == 0 {
		return nil, ErrNotFound
	}
	cp := make([]byte, len(s.hmacSecret))
	copy(cp, s.hmacSecret)
	return cp, nil
}

// SetHMACSecret implements DomainStore.
func (s *MemoryStore) SetHMACSecret(_ context.Context, secret []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(secret))
	copy(cp, secret)
	s.hmacSecret = cp
	return nil
}

// ── Data retention ────────────────────────────────────────────────────────────

// ListRetentionDue implements DomainStore.
// Returns contacts whose LastAppointmentEndAt + retentionPeriod has passed and
// whose RetentionState is "active".
func (s *MemoryStore) ListRetentionDue(_ context.Context, retentionPeriod time.Duration) ([]*Contact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now().UTC()
	var result []*Contact
	for _, c := range s.contacts {
		if c.RetentionState != RetentionStateActive {
			continue
		}
		if c.LastAppointmentEndAt.IsZero() {
			continue
		}
		if now.After(c.LastAppointmentEndAt.Add(retentionPeriod)) {
			cp := *c
			result = append(result, &cp)
		}
	}
	return result, nil
}

// MarkRetentionNotified implements DomainStore.
func (s *MemoryStore) MarkRetentionNotified(_ context.Context, contactID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contacts[contactID]
	if !ok {
		return ErrNotFound
	}
	c.RetentionState = RetentionStateNotified
	c.RetentionNotifiedAt = time.Now().UTC()
	return nil
}

// ListConfirmationExpired implements DomainStore.
// Returns contacts whose RetentionState is "notified" and whose
// RetentionNotifiedAt is more than 7 days ago.
func (s *MemoryStore) ListConfirmationExpired(_ context.Context) ([]*Contact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	deadline := time.Now().UTC().Add(-7 * 24 * time.Hour)
	var result []*Contact
	for _, c := range s.contacts {
		if c.RetentionState != RetentionStateNotified {
			continue
		}
		if c.RetentionNotifiedAt.Before(deadline) {
			cp := *c
			result = append(result, &cp)
		}
	}
	return result, nil
}

// AddToPendingDeletion implements DomainStore.
func (s *MemoryStore) AddToPendingDeletion(_ context.Context, contactID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contacts[contactID]
	if !ok {
		return ErrNotFound
	}
	c.RetentionState = RetentionStatePendingDeletion
	return nil
}

// ListPendingDeletion implements DomainStore.
func (s *MemoryStore) ListPendingDeletion(_ context.Context) ([]*Contact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Contact
	for _, c := range s.contacts {
		if c.RetentionState == RetentionStatePendingDeletion {
			cp := *c
			result = append(result, &cp)
		}
	}
	return result, nil
}

// DeleteContact implements DomainStore.
// Permanently removes the Contact and cascades to all its BookingSessions and
// Bookings.
func (s *MemoryStore) DeleteContact(_ context.Context, contactID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contacts[contactID]
	if !ok {
		return ErrNotFound
	}
	delete(s.emailIndex, c.Email)
	delete(s.contacts, contactID)
	for id, bs := range s.sessions {
		if bs.ContactID == contactID {
			delete(s.sessions, id)
		}
	}
	for id, b := range s.bookings {
		if b.ContactID == contactID {
			delete(s.bookings, id)
		}
	}
	return nil
}

// ── Billing ───────────────────────────────────────────────────────────────────

// ListBillingDue implements DomainStore.
// Returns contacts whose LastAppointmentEndAt is in the past and
// BillingGenerated is false.
func (s *MemoryStore) ListBillingDue(_ context.Context) ([]*Contact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now().UTC()
	var result []*Contact
	for _, c := range s.contacts {
		if c.BillingGenerated {
			continue
		}
		if c.LastAppointmentEndAt.IsZero() || !c.LastAppointmentEndAt.Before(now) {
			continue
		}
		cp := *c
		result = append(result, &cp)
	}
	return result, nil
}

// MarkBillingGenerated implements DomainStore.
func (s *MemoryStore) MarkBillingGenerated(_ context.Context, contactID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contacts[contactID]
	if !ok {
		return ErrNotFound
	}
	c.BillingGenerated = true
	return nil
}
