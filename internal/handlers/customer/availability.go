// Package customer implements the public HTTP handlers for the customer-facing
// booking flow. All endpoints are unauthenticated (no session required).
//
// Route prefix: /api/v1/ (registered by the server router).
package customer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"schedio/internal/domain"
	"schedio/internal/store"
)

// availabilityResponse is the JSON array returned by GET /api/v1/availability.
// It is a flat, chronologically-sorted list of RFC 3339 UTC start-time strings.
// Empty periods return an empty array (never null).
//
//	["2026-03-02T08:00:00Z", "2026-03-02T09:00:00Z", …]
type availabilityResponse = []string

// AvailabilityHandler handles GET /api/v1/availability.
type AvailabilityHandler struct {
	svc *domain.AvailabilityService
}

// NewAvailabilityHandler constructs an AvailabilityHandler backed by the given
// DomainStore.
func NewAvailabilityHandler(st store.DomainStore) *AvailabilityHandler {
	return &AvailabilityHandler{svc: domain.NewAvailabilityService(st)}
}

// ServeHTTP implements http.Handler.
func (h *AvailabilityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()

	serviceID := q.Get("service_id")
	if serviceID == "" {
		http.Error(w, "missing required query parameter: service_id", http.StatusBadRequest)
		return
	}

	period := q.Get("period")
	if period == "" {
		http.Error(w, "missing required query parameter: period (YYYY-MM or YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	var rangeStart, rangeEnd time.Time

	switch len(period) {
	case 7: // YYYY-MM
		t, err := time.Parse("2006-01", period)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid period %q: expected YYYY-MM or YYYY-MM-DD", period), http.StatusBadRequest)
			return
		}
		rangeStart = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		rangeEnd = rangeStart.AddDate(0, 1, 0)
	case 10: // YYYY-MM-DD
		t, err := time.Parse("2006-01-02", period)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid period %q: expected YYYY-MM or YYYY-MM-DD", period), http.StatusBadRequest)
			return
		}
		rangeStart = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		rangeEnd = rangeStart.Add(24 * time.Hour)
	default:
		http.Error(w, fmt.Sprintf("invalid period %q: expected YYYY-MM or YYYY-MM-DD", period), http.StatusBadRequest)
		return
	}

	daySlots, err := h.svc.ListAvailableForDateRange(r.Context(), serviceID, rangeStart, rangeEnd)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "service not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Build the response: convert []domain.Slot → flat []string (RFC 3339 UTC),
	// deduplicate, and sort chronologically.
	seen := make(map[string]struct{})
	var slots []string
	for _, daySlotList := range daySlots {
		for _, s := range daySlotList {
			t := s.StartAt.UTC().Format(time.RFC3339)
			if _, dup := seen[t]; dup {
				continue
			}
			seen[t] = struct{}{}
			slots = append(slots, t)
		}
	}
	sort.Strings(slots)

	// Always return an array, never null.
	if slots == nil {
		slots = []string{}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(availabilityResponse(slots))
}
