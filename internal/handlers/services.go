package handlers

import (
	"encoding/json"
	"net/http"

	"schedio/internal/config"
)

// publicService is the JSON representation of a service returned by the public
// API. It intentionally omits daily_limit, which is an internal operational
// field not relevant to customers.
type publicService struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Summary         string  `json:"summary"`
	Description     string  `json:"description"`
	Price           float64 `json:"price"`
	DurationMinutes int     `json:"duration_minutes"`
}

// NewServicesHandler returns an http.Handler that serves the public
// GET /api/v1/services endpoint. The response is derived from the services
// slice stored in args at startup; it is read-only and never changes at
// runtime.
func NewServicesHandler(args *config.Config) http.Handler {
	// Build the response payload once at construction time.
	payload := make([]publicService, len(args.Services))
	for i, s := range args.Services {
		payload[i] = publicService{
			ID:              s.ID,
			Name:            s.Name,
			Summary:         s.Summary,
			Description:     s.Description,
			Price:           s.Price,
			DurationMinutes: s.DurationMinutes,
		}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		// This can only happen if publicService contains non-marshalable types,
		// which it cannot by construction.
		panic("handlers: failed to pre-marshal services payload: " + err.Error())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dispatchHandlers(w, r, map[string]MethodHandler{
			http.MethodGet: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(data)
			},
		})
	})
}
