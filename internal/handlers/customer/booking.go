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

// ── Response shapes ───────────────────────────────────────────────────────────

type bookingResp struct {
	ID           string      `json:"id"`
	SessionID    string      `json:"session_id"`
	Service      serviceResp `json:"service"`
	Contact      contactResp `json:"contact"`
	StartAt      string      `json:"start_at"`
	EndAt        string      `json:"end_at"`
	State        string      `json:"state"`
	CancelReason string      `json:"cancel_reason,omitempty"`
	Location     string      `json:"location"`
}

type serviceResp struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	DurationMinutes int     `json:"duration_minutes"`
	Price           float64 `json:"price"`
}

type contactResp struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type rescheduleReq struct {
	NewSlot  string `json:"new_slot"`  // RFC 3339 UTC
	Timezone string `json:"timezone"` // IANA tz name from Intl.DateTimeFormat().resolvedOptions().timeZone
}

type newSessionResp struct {
	SessionID string `json:"session_id"`
	BookingID string `json:"booking_id"`
}

// ── BookingHandler ────────────────────────────────────────────────────────────

// BookingHandler implements the customer management-link endpoints:
//
//	GET    /api/v1/bookings/{id}                — view booking
//	POST   /api/v1/bookings/{id}/reschedule     — reschedule
//	DELETE /api/v1/bookings/{id}                — cancel
//	POST   /api/v1/bookings/{id}/new-session    — start new session
//
// Every endpoint requires a valid ?token= query parameter signed with the
// HMAC secret managed by internal/token.
type BookingHandler struct {
	st         store.DomainStore
	signer     *token.Signer
	avail      *domain.AvailabilityService
	bookingSvc *domain.BookingService
	sender     *email.Sender // nil when SMTP is not configured
}

// NewBookingHandler constructs a BookingHandler.
func NewBookingHandler(st store.DomainStore, signer *token.Signer, sender *email.Sender) *BookingHandler {
	return &BookingHandler{
		st:         st,
		signer:     signer,
		avail:      domain.NewAvailabilityService(st),
		bookingSvc: domain.NewBookingService(st),
		sender:     sender,
	}
}

// buildManageLink returns the signed customer management URL for the given booking,
// using CalendarURL from settings as the web root base.
func (h *BookingHandler) buildManageLink(ctx context.Context, bookingID string) string {
	base := ""
	if st, err := h.st.GetSettings(ctx); err == nil {
		base = strings.TrimRight(st.CalendarURL, "/")
		if idx := strings.Index(base, "/caldav"); idx != -1 {
			base = base[:idx]
		}
	}
	tok := h.signer.Sign(bookingID)
	return fmt.Sprintf("%s/?id=%s&token=%s", base, bookingID, tok)
}

// verifyToken extracts and validates the ?token= query parameter against the
// provided bookingID. Returns false and writes the appropriate HTTP error when
// verification fails.
func (h *BookingHandler) verifyToken(w http.ResponseWriter, r *http.Request, bookingID string) bool {
	tok := r.URL.Query().Get("token")
	if tok == "" {
		http.Error(w, "token required", http.StatusForbidden)
		return false
	}
	gotID, err := h.signer.Verify(tok)
	if err != nil || gotID != bookingID {
		http.Error(w, "invalid or tampered token", http.StatusForbidden)
		return false
	}
	return true
}

// buildResp assembles a bookingResp by fetching the related service, contact,
// and settings from the store.
func (h *BookingHandler) buildResp(ctx context.Context, b *store.Booking) (bookingResp, error) {
	svc, err := h.st.GetService(ctx, b.ServiceID)
	if err != nil {
		return bookingResp{}, fmt.Errorf("get service: %w", err)
	}
	contact, err := h.st.GetContact(ctx, b.ContactID)
	if err != nil {
		return bookingResp{}, fmt.Errorf("get contact: %w", err)
	}
	settings, err := h.st.GetSettings(ctx)
	if err != nil {
		return bookingResp{}, fmt.Errorf("get settings: %w", err)
	}
	return bookingResp{
		ID:        b.ID,
		SessionID: b.SessionID,
		Service: serviceResp{
			ID:              svc.ID,
			Name:            svc.Name,
			DurationMinutes: svc.DurationMinutes,
			Price:           svc.Price,
		},
		Contact: contactResp{
			FirstName: contact.FirstName,
			LastName:  contact.LastName,
			Email:     contact.Email,
			Phone:     contact.Phone,
		},
		StartAt:      b.StartAt.Format(time.RFC3339),
		EndAt:        b.EndAt.Format(time.RFC3339),
		State:        string(b.State),
		CancelReason: string(b.CancelReason),
		Location:     settings.AppointmentLocation,
	}, nil
}

// Get handles GET /api/v1/bookings/{id}?token=
func (h *BookingHandler) Get(w http.ResponseWriter, r *http.Request) {
	bookingID := r.PathValue("id")
	if !h.verifyToken(w, r, bookingID) {
		return
	}

	b, err := h.st.GetBooking(r.Context(), bookingID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "booking not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp, err := h.buildResp(r.Context(), b)
	if err != nil {
		klog.Errorf("bookings.Get: buildResp %s: %v", bookingID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// Reschedule handles POST /api/v1/bookings/{id}/reschedule?token=
func (h *BookingHandler) Reschedule(w http.ResponseWriter, r *http.Request) {
	bookingID := r.PathValue("id")
	if !h.verifyToken(w, r, bookingID) {
		return
	}

	var req rescheduleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NewSlot == "" {
		http.Error(w, "invalid request body: new_slot required", http.StatusBadRequest)
		return
	}

	newStart, err := time.Parse(time.RFC3339, req.NewSlot)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid new_slot: %v", err), http.StatusBadRequest)
		return
	}
	newStart = newStart.UTC()

	b, err := h.st.GetBooking(r.Context(), bookingID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "booking not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if b.State != store.BookingStateReserved && b.State != store.BookingStateConfirmed {
		http.Error(w, "booking cannot be rescheduled in its current state", http.StatusConflict)
		return
	}

	svc, err := h.st.GetService(r.Context(), b.ServiceID)
	if err != nil {
		http.Error(w, "service not found", http.StatusInternalServerError)
		return
	}
	newEnd := newStart.Add(time.Duration(svc.DurationMinutes) * time.Minute)

	// Verify the requested slot is still available.
	slots, err := h.avail.ListAvailable(r.Context(), b.ServiceID, newStart)
	if err != nil {
		klog.Errorf("bookings.Reschedule ListAvailable %s: %v", bookingID, err)
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
		http.Error(w, "requested time slot is not available", http.StatusConflict)
		return
	}

	// Update the booking.
	b.StartAt = newStart
	b.EndAt = newEnd
	b.UserID = staffID
	b.Sequence++
	b.UpdatedAt = time.Now().UTC()
	if err := h.st.UpdateBooking(r.Context(), b); err != nil {
		if errors.Is(err, store.ErrConflict) {
			http.Error(w, "time slot already booked", http.StatusConflict)
			return
		}
		klog.Errorf("bookings.Reschedule UpdateBooking %s: %v", bookingID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Update the contact's last appointment clock.
	_ = h.st.UpdateContactLastAppointment(r.Context(), b.ContactID, newEnd)

	// Send change-summary email.
	if h.sender != nil {
		contact, cErr := h.st.GetContact(r.Context(), b.ContactID)
		if cErr == nil {
			// Use the timezone supplied in the request; fall back to what's stored on the
			// contact so re-sends (e.g. retry) still format correctly.
			tzName := req.Timezone
			if tzName == "" {
				tzName = contact.Timezone
			} else if tzName != contact.Timezone {
				// Keep the stored timezone up to date.
				contact.Timezone = tzName
				_ = h.st.UpdateContact(r.Context(), contact)
			}
			customerLoc := parseTimezone(tzName)
			manageLink := h.buildManageLink(r.Context(), b.ID)
			data := email.ChangeSummaryData{
				Contact:    contact,
				Booking:    inTZSingle(b, customerLoc),
				ManageLink: manageLink,
				SentAt:     time.Now().UTC(),
			}
			emailCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()
			if err := h.sender.SendChangeSummary(emailCtx, data); err != nil {
				klog.Errorf("bookings.Reschedule SendChangeSummary %s: %v", bookingID, err)
			}
		}
	}

	resp, err := h.buildResp(r.Context(), b)
	if err != nil {
		klog.Errorf("bookings.Reschedule buildResp %s: %v", bookingID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// Cancel handles DELETE /api/v1/bookings/{id}?token=
func (h *BookingHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	bookingID := r.PathValue("id")
	if !h.verifyToken(w, r, bookingID) {
		return
	}

	settings, err := h.st.GetSettings(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	b, err := h.bookingSvc.CancelBookingByCustomer(r.Context(), bookingID, settings.NoShowDeadlineHours)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "booking not found", http.StatusNotFound)
			return
		}
		// Domain error for wrong state.
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	// Send cancellation email.
	if h.sender != nil {
		contact, cErr := h.st.GetContact(r.Context(), b.ContactID)
		if cErr == nil {
			// The customer's timezone may come from the ?tz= query param (set by the
			// frontend) or fall back to whatever was stored on the contact at submit time.
			tzName := r.URL.Query().Get("tz")
			if tzName == "" {
				tzName = contact.Timezone
			}
			customerLoc := parseTimezone(tzName)
			data := email.CancellationData{
				Contact: contact,
				Booking: inTZSingle(b, customerLoc),
				SentAt:  time.Now().UTC(),
			}
			emailCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()
			if err := h.sender.SendCancellation(emailCtx, data); err != nil {
				klog.Errorf("bookings.Cancel SendCancellation %s: %v", bookingID, err)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"state": string(b.State)})
}

// NewSession handles POST /api/v1/bookings/{id}/new-session?token=
// It starts a new open booking session for the same service and contact.
func (h *BookingHandler) NewSession(w http.ResponseWriter, r *http.Request) {
	bookingID := r.PathValue("id")
	if !h.verifyToken(w, r, bookingID) {
		return
	}

	b, err := h.st.GetBooking(r.Context(), bookingID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "booking not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	sess := &store.BookingSession{
		ID:        store.NewID(),
		ServiceID: b.ServiceID,
		ContactID: b.ContactID,
		State:     store.SessionStateOpen,
		CreatedAt: now,
	}
	if err := h.st.CreateSession(r.Context(), sess); err != nil {
		klog.Errorf("bookings.NewSession CreateSession: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, newSessionResp{
		SessionID: sess.ID,
		BookingID: b.ID,
	})
}
