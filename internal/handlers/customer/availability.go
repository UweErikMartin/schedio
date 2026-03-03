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

// availabilityResponse is the JSON envelope returned by GET /api/v1/availability.
//
//	{
//	  "months": {
//	    "YYYY-MM": {
//	      "YYYY-MM-DD": ["2026-03-02T08:00:00Z", …]
//	    }
//	  }
//	}
//
// Days with no available slots are omitted. Times are RFC 3339 UTC timestamps;
// the client converts them to the browser's local timezone.
type availabilityResponse struct {
	Months map[string]map[string][]string `json:"months"`
}

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
	var monthKey string // "YYYY-MM"

	switch len(period) {
	case 7: // YYYY-MM
		t, err := time.Parse("2006-01", period)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid period %q: expected YYYY-MM or YYYY-MM-DD", period), http.StatusBadRequest)
			return
		}
		rangeStart = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		rangeEnd = rangeStart.AddDate(0, 1, 0)
		monthKey = rangeStart.Format("2006-01")
	case 10: // YYYY-MM-DD
		t, err := time.Parse("2006-01-02", period)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid period %q: expected YYYY-MM or YYYY-MM-DD", period), http.StatusBadRequest)
			return
		}
		rangeStart = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		rangeEnd = rangeStart.Add(24 * time.Hour)
		monthKey = rangeStart.Format("2006-01")
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

	// Build the response: convert []domain.Slot → []string (RFC 3339 UTC), dedup,
	// and sort within each day.
	dayMap := make(map[string][]string, len(daySlots))
	for dateKey, slots := range daySlots {
		seen := make(map[string]struct{}, len(slots))
		var times []string
		for _, s := range slots {
			t := s.StartAt.UTC().Format(time.RFC3339)
			if _, dup := seen[t]; dup {
				continue
			}
			seen[t] = struct{}{}
			times = append(times, t)
		}
		sort.Strings(times)
		dayMap[dateKey] = times
	}

	resp := availabilityResponse{
		Months: map[string]map[string][]string{
			monthKey: dayMap,
		},
	}

	// When there are no slots at all, ensure the inner map is an empty object,
	// not JSON null.
	if resp.Months[monthKey] == nil {
		resp.Months[monthKey] = map[string][]string{}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
