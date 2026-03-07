// Package customer implements the public HTTP handlers for the customer-facing
// booking flow. All endpoints are unauthenticated (no session required).
//
// Route prefix: /api/v1/ (registered by the server router).
package customer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"schedio/internal/domain"
	"schedio/internal/email"
	"schedio/internal/store"
	"schedio/internal/token"
)

// ── Request / response shapes ─────────────────────────────────────────────────

type createSessionReq struct {
	ServiceID string `json:"service_id"`
}

// sessionDetailResp is the response body for GET /api/v1/sessions/{id}.
type sessionDetailResp struct {
	ID       string               `json:"id"`
	State    string               `json:"state"`
	Service  serviceResp          `json:"service"`
	Contact  contactResp          `json:"contact"`
	Bookings []sessionBookingResp `json:"bookings"`
}

// sessionBookingResp is a booking line inside sessionDetailResp.
type sessionBookingResp struct {
	ID    string `json:"id"`
	Start string `json:"start_at"`
	End   string `json:"end_at"`
	State string `json:"state"`
}

// sessionRescheduleReq is the request body for POST /api/v1/sessions/{id}/reschedule.
type sessionRescheduleReq struct {
	Slots    []string `json:"slots"`    // RFC 3339 UTC; one entry per active booking in start order
	Timezone string   `json:"timezone"` // IANA tz name reported by the browser
}

type sessionResp struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
	State     string `json:"state"`
}

type addBookingReq struct {
	// Start is the requested slot start in RFC 3339 / ISO 8601 UTC format.
	Start string `json:"start"`
}

type bookingLineResp struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
	Start     string `json:"start"`
	End       string `json:"end"`
}

type submitReq struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Timezone  string `json:"timezone"` // IANA tz name from Intl.DateTimeFormat().resolvedOptions().timeZone
}

type submitResp struct {
	SessionID  string            `json:"session_id"`
	Bookings   []bookingLineResp `json:"bookings"`
	EmailSent  bool              `json:"email_sent"`
	EmailError string            `json:"email_error,omitempty"`
}

// ── SessionHandler ────────────────────────────────────────────────────────────

// SessionHandler implements the public booking-session lifecycle endpoints:
//
//	POST   /api/v1/sessions
//	POST   /api/v1/sessions/{id}/bookings
//	POST   /api/v1/sessions/{id}/submit
type SessionHandler struct {
	st         store.DomainStore
	avail      *domain.AvailabilityService
	bookingSvc *domain.BookingService
	sender     *email.Sender // nil when SMTP is not configured; email step is skipped
	signer     *token.Signer
	adminEmail string
}

// NewSessionHandler constructs a SessionHandler.
func NewSessionHandler(st store.DomainStore, sender *email.Sender, signer *token.Signer, adminEmail string) *SessionHandler {
	return &SessionHandler{
		st:         st,
		avail:      domain.NewAvailabilityService(st),
		bookingSvc: domain.NewBookingService(st),
		sender:     sender,
		signer:     signer,
		adminEmail: adminEmail,
	}
}

// manageLink builds the signed customer management URL for the given booking.
// It uses CalendarURL from settings as the base (stripped of any trailing path
// so the link always points to the web UI root).
func (h *SessionHandler) manageLink(ctx context.Context, bookingID string) string {
	base := ""
	if st, err := h.st.GetSettings(ctx); err == nil {
		base = strings.TrimRight(st.CalendarURL, "/")
		// Strip any CalDAV path suffix – we want the web root, not /caldav/.
		if idx := strings.Index(base, "/caldav"); idx != -1 {
			base = base[:idx]
		}
	}
	tok := h.signer.Sign(bookingID)
	return fmt.Sprintf("%s/?id=%s&token=%s", base, bookingID, tok)
}

// Create handles POST /api/v1/sessions.
// It validates that the requested service exists and persists a new open session.
func (h *SessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createSessionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ServiceID == "" {
		http.Error(w, "invalid request body: service_id required", http.StatusBadRequest)
		return
	}

	if _, err := h.st.GetService(r.Context(), req.ServiceID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "service not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	session := &store.BookingSession{
		ID:        store.NewID(),
		ServiceID: req.ServiceID,
		State:     store.SessionStateOpen,
		CreatedAt: now,
	}
	if err := h.st.CreateSession(r.Context(), session); err != nil {
		klog.Errorf("sessions: CreateSession: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, sessionResp{
		ID:        session.ID,
		ServiceID: session.ServiceID,
		State:     string(session.State),
	})
}

// AddBooking handles POST /api/v1/sessions/{id}/bookings.
// It resolves the staff user who holds the matching timeslot and creates the
// booking line. Returns 409 Conflict when the slot is already taken or not
// in the availability list.
func (h *SessionHandler) AddBooking(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}

	session, err := h.st.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if session.State != store.SessionStateOpen {
		http.Error(w, "session is not open", http.StatusConflict)
		return
	}

	var req addBookingReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Start == "" {
		http.Error(w, "invalid request body: start required", http.StatusBadRequest)
		return
	}
	startAt, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid start time: %v", err), http.StatusBadRequest)
		return
	}
	startAt = startAt.UTC()

	svc, err := h.st.GetService(r.Context(), session.ServiceID)
	if err != nil {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}
	endAt := startAt.Add(time.Duration(svc.DurationMinutes) * time.Minute)

	// Resolve the staff member who has a free timeslot at the requested time.
	slots, err := h.avail.ListAvailable(r.Context(), session.ServiceID, startAt)
	if err != nil {
		klog.Errorf("sessions: ListAvailable for booking: %v", err)
		http.Error(w, "could not resolve availability", http.StatusInternalServerError)
		return
	}
	var staffID string
	for _, s := range slots {
		if s.StartAt.Equal(startAt) {
			staffID = s.UserID
			break
		}
	}
	if staffID == "" {
		http.Error(w, "requested time slot is not available", http.StatusConflict)
		return
	}

	// contactID is empty until the session is submitted.
	b, err := h.bookingSvc.CreateBooking(r.Context(), sessionID, session.ServiceID, "", staffID, startAt, endAt)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			http.Error(w, "time slot already booked", http.StatusConflict)
			return
		}
		klog.Errorf("sessions: CreateBooking: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, bookingLineResp{
		ID:        b.ID,
		ServiceID: b.ServiceID,
		Start:     b.StartAt.Format(time.RFC3339),
		End:       b.EndAt.Format(time.RFC3339),
	})
}

// Submit handles POST /api/v1/sessions/{id}/submit.
// It attaches contact information to all open booking lines, transitions the
// session to "submitted", and sends the reserved + admin-notify emails.
func (h *SessionHandler) Submit(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}

	var req submitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.FirstName == "" || req.LastName == "" || req.Email == "" {
		http.Error(w, "first_name, last_name and email are required", http.StatusBadRequest)
		return
	}

	session, err := h.st.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if session.State != store.SessionStateOpen {
		http.Error(w, "session already submitted", http.StatusConflict)
		return
	}

	bookings, err := h.st.ListBookingsForSession(r.Context(), sessionID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if len(bookings) == 0 {
		http.Error(w, "session has no booking lines", http.StatusBadRequest)
		return
	}

	// Upsert the contact (look up by email; create if new).
	contact, err := h.st.GetOrCreateContact(r.Context(), req.Email, &store.Contact{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Phone:     req.Phone,
		Timezone:  req.Timezone,
	})
	if err != nil {
		klog.Errorf("sessions: GetOrCreateContact: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	// Always update the stored timezone to the latest value supplied by the browser.
	if req.Timezone != "" && req.Timezone != contact.Timezone {
		contact.Timezone = req.Timezone
		if err := h.st.UpdateContact(r.Context(), contact); err != nil {
			klog.V(2).Infof("sessions: UpdateContact timezone: %v", err)
		}
	}

	// Attach the contact to every booking line and advance the retention clock.
	for _, b := range bookings {
		b.ContactID = contact.ID
		if err := h.st.UpdateBooking(r.Context(), b); err != nil {
			klog.Errorf("sessions: UpdateBooking contactID: %v", err)
		}
		_ = h.st.UpdateContactLastAppointment(r.Context(), contact.ID, b.EndAt)
	}

	// Transition the session to "submitted".
	now := time.Now().UTC()
	session.ContactID = contact.ID
	session.State = store.SessionStateSubmitted
	session.SubmittedAt = now
	if err := h.st.UpdateSession(r.Context(), session); err != nil {
		klog.Errorf("sessions: UpdateSession submit: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Send confirmation emails synchronously so the caller receives immediate
	// feedback on delivery. The booking is already persisted at this point;
	// an SMTP failure is reported in the response body but does not roll back
	// the session transition.
	var emailSent bool
	var emailErr string
	if h.sender != nil {
		// Determine the customer's timezone for display in customer-facing emails,
		// and use the server's local timezone for the admin-notify email.
		customerLoc := parseTimezone(contact.Timezone)

		// Build per-booking manage links and the single session manage link.
		manageLinks := make(map[string]string, len(bookings))
		for _, b := range bookings {
			manageLinks[b.ID] = h.manageLink(r.Context(), b.ID)
		}
		firstLink := ""
		if len(bookings) > 0 {
			firstLink = manageLinks[bookings[0].ID]
		}
		reservedData := email.ReservedData{
			Contact:           contact,
			Session:           session,
			Bookings:          inTZ(bookings, customerLoc),
			ManageLink:        firstLink,
			ManageLinks:       manageLinks,
			SessionManageLink: h.sessionManageLink(r.Context(), sessionID),
			SentAt:            now,
		}
		adminData := email.AdminNotifyData{
			Session:  session,
			Contact:  contact,
			Bookings: inTZ(bookings, time.Local),
			SentAt:   now,
		}
		emailCtx, emailCancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer emailCancel()
		if err := h.sender.SendReserved(emailCtx, reservedData); err != nil {
			klog.Errorf("sessions: SendReserved to %s: %v", contact.Email, err)
			emailErr = fmt.Sprintf("Bestätigungs-E-Mail konnte nicht gesendet werden: %v", err)
		} else {
			emailSent = true
			if h.adminEmail != "" {
				if err := h.sender.SendAdminNotify(emailCtx, h.adminEmail, adminData); err != nil {
					klog.Errorf("sessions: SendAdminNotify to %s: %v", h.adminEmail, err)
				}
			}
		}
	}

	lines := make([]bookingLineResp, len(bookings))
	for i, b := range bookings {
		lines[i] = bookingLineResp{
			ID:        b.ID,
			ServiceID: b.ServiceID,
			Start:     b.StartAt.Format(time.RFC3339),
			End:       b.EndAt.Format(time.RFC3339),
		}
	}
	writeJSON(w, http.StatusOK, submitResp{
		SessionID:  sessionID,
		Bookings:   lines,
		EmailSent:  emailSent,
		EmailError: emailErr,
	})
}

// writeJSON is a local helper that encodes v as JSON and writes it to w with
// the given status code and Content-Type header.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		klog.Errorf("writeJSON: %v", err)
	}
}

// sessionManageLink builds the signed customer management URL for a whole
// session, using CalendarURL from settings as the web root base.
func (h *SessionHandler) sessionManageLink(ctx context.Context, sessionID string) string {
	base := ""
	if st, err := h.st.GetSettings(ctx); err == nil {
		base = strings.TrimRight(st.CalendarURL, "/")
		if idx := strings.Index(base, "/caldav"); idx != -1 {
			base = base[:idx]
		}
	}
	tok := h.signer.Sign(sessionID)
	return fmt.Sprintf("%s/?session_id=%s&session_token=%s", base, sessionID, tok)
}

// verifySessionToken extracts and validates the ?session_token= query
// parameter against the provided sessionID.
func (h *SessionHandler) verifySessionToken(w http.ResponseWriter, r *http.Request, sessionID string) bool {
	tok := r.URL.Query().Get("session_token")
	if tok == "" {
		http.Error(w, "session_token required", http.StatusForbidden)
		return false
	}
	gotID, err := h.signer.Verify(tok)
	if err != nil || gotID != sessionID {
		http.Error(w, "invalid or tampered session token", http.StatusForbidden)
		return false
	}
	return true
}

// Get handles GET /api/v1/sessions/{id}?session_token=
// Returns session details including all booking lines, service, and contact.
func (h *SessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if !h.verifySessionToken(w, r, sessionID) {
		return
	}

	session, err := h.st.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	svc, err := h.st.GetService(r.Context(), session.ServiceID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var contact contactResp
	if session.ContactID != "" {
		if c, cErr := h.st.GetContact(r.Context(), session.ContactID); cErr == nil {
			contact = contactResp{
				FirstName: c.FirstName,
				LastName:  c.LastName,
				Email:     c.Email,
				Phone:     c.Phone,
			}
		}
	}

	bookings, err := h.st.ListBookingsForSession(r.Context(), sessionID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	lines := make([]sessionBookingResp, len(bookings))
	for i, b := range bookings {
		lines[i] = sessionBookingResp{
			ID:    b.ID,
			Start: b.StartAt.Format(time.RFC3339),
			End:   b.EndAt.Format(time.RFC3339),
			State: string(b.State),
		}
	}

	writeJSON(w, http.StatusOK, sessionDetailResp{
		ID:    sessionID,
		State: string(session.State),
		Service: serviceResp{
			ID:              svc.ID,
			Name:            svc.Name,
			DurationMinutes: svc.DurationMinutes,
			Price:           svc.Price,
		},
		Contact:  contact,
		Bookings: lines,
	})
}

// Reschedule handles POST /api/v1/sessions/{id}/reschedule?session_token=
// Body: {"slots":["RFC3339",...],"timezone":"Europe/Berlin"}
// The slots slice must contain exactly one entry per non-cancelled booking
// in the session, ordered by StartAt ascending.
func (h *SessionHandler) Reschedule(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if !h.verifySessionToken(w, r, sessionID) {
		return
	}

	var req sessionRescheduleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Slots) == 0 {
		http.Error(w, "invalid request body: slots required", http.StatusBadRequest)
		return
	}

	session, err := h.st.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	svc, err := h.st.GetService(r.Context(), session.ServiceID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	bookings, err := h.st.ListBookingsForSession(r.Context(), sessionID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	var active []*store.Booking
	for _, b := range bookings {
		if b.State != store.BookingStateCancelled {
			active = append(active, b)
		}
	}
	if len(req.Slots) != len(active) {
		http.Error(w, fmt.Sprintf("slot count (%d) must match active booking count (%d)", len(req.Slots), len(active)), http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	for i, b := range active {
		newStart, parseErr := time.Parse(time.RFC3339, req.Slots[i])
		if parseErr != nil {
			http.Error(w, fmt.Sprintf("invalid slot %q: %v", req.Slots[i], parseErr), http.StatusBadRequest)
			return
		}
		newStart = newStart.UTC()
		newEnd := newStart.Add(time.Duration(svc.DurationMinutes) * time.Minute)

		slots, availErr := h.avail.ListAvailable(r.Context(), session.ServiceID, newStart)
		if availErr != nil {
			klog.Errorf("sessions.Reschedule ListAvailable booking %s: %v", b.ID, availErr)
			http.Error(w, "could not verify availability", http.StatusInternalServerError)
			return
		}
		var staffID string
		for _, s := range slots {
			if s.StartAt.Equal(newStart) {
				staffID = s.UserID
				break
			}
		}
		if staffID == "" {
			http.Error(w, fmt.Sprintf("slot %s is not available", req.Slots[i]), http.StatusConflict)
			return
		}

		b.StartAt = newStart
		b.EndAt = newEnd
		b.UserID = staffID
		b.Sequence++
		b.UpdatedAt = now
		if updateErr := h.st.UpdateBooking(r.Context(), b); updateErr != nil {
			if errors.Is(updateErr, store.ErrConflict) {
				http.Error(w, fmt.Sprintf("slot %s is already booked", req.Slots[i]), http.StatusConflict)
				return
			}
			klog.Errorf("sessions.Reschedule UpdateBooking %s: %v", b.ID, updateErr)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		_ = h.st.UpdateContactLastAppointment(r.Context(), b.ContactID, newEnd)
	}

	// Send one change-summary email per rescheduled booking.
	if h.sender != nil && session.ContactID != "" {
		contact, cErr := h.st.GetContact(r.Context(), session.ContactID)
		if cErr == nil {
			tzName := req.Timezone
			if tzName == "" {
				tzName = contact.Timezone
			} else if tzName != contact.Timezone {
				contact.Timezone = tzName
				_ = h.st.UpdateContact(r.Context(), contact)
			}
			customerLoc := parseTimezone(tzName)
			sessionLink := h.sessionManageLink(r.Context(), sessionID)
			for _, b := range active {
				data := email.ChangeSummaryData{
					Contact:    contact,
					Booking:    inTZSingle(b, customerLoc),
					ManageLink: sessionLink,
					SentAt:     now,
				}
				emailCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
				defer cancel()
				if sendErr := h.sender.SendChangeSummary(emailCtx, data); sendErr != nil {
					klog.Errorf("sessions.Reschedule SendChangeSummary booking %s: %v", b.ID, sendErr)
				}
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// Cancel handles DELETE /api/v1/sessions/{id}?session_token=
// Cancels all non-cancelled bookings in the session and sends one
// cancellation e-mail.
func (h *SessionHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if !h.verifySessionToken(w, r, sessionID) {
		return
	}

	session, err := h.st.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	settings, err := h.st.GetSettings(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	bookings, err := h.st.ListBookingsForSession(r.Context(), sessionID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	var cancelled []*store.Booking
	for _, b := range bookings {
		if b.State == store.BookingStateCancelled {
			continue
		}
		cb, cancelErr := h.bookingSvc.CancelBookingByCustomer(r.Context(), b.ID, settings.NoShowDeadlineHours)
		if cancelErr != nil {
			klog.Errorf("sessions.Cancel CancelBookingByCustomer %s: %v", b.ID, cancelErr)
			continue
		}
		cancelled = append(cancelled, cb)
	}

	// Send one cancellation e-mail representing the whole session.
	if h.sender != nil && session.ContactID != "" && len(cancelled) > 0 {
		contact, cErr := h.st.GetContact(r.Context(), session.ContactID)
		if cErr == nil {
			tzName := r.URL.Query().Get("tz")
			if tzName == "" {
				tzName = contact.Timezone
			}
			customerLoc := parseTimezone(tzName)
			data := email.CancellationData{
				Contact: contact,
				Booking: inTZSingle(cancelled[0], customerLoc),
				SentAt:  now,
			}
			emailCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()
			if sendErr := h.sender.SendCancellation(emailCtx, data); sendErr != nil {
				klog.Errorf("sessions.Cancel SendCancellation: %v", sendErr)
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
