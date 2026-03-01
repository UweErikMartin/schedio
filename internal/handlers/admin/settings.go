// Package admin implements the authenticated HTTP handlers for the admin SPA.
//
// All endpoints require a valid staff session (RequireRole middleware from
// internal/auth). Route prefix: /admin/api/v1/ (registered by the server router).
package admin

import (
	"encoding/json"
	"net/http"

	"k8s.io/klog/v2"

	"schedio/internal/email"
	"schedio/internal/store"
)

// ── Settings handler ──────────────────────────────────────────────────────────

// SettingsHandler implements GET and PUT for /admin/api/v1/settings.
type SettingsHandler struct {
	st     store.DomainStore
	sender *email.Sender // may be nil when SMTP is disabled
}

// NewSettingsHandler creates a SettingsHandler.
func NewSettingsHandler(st store.DomainStore, sender *email.Sender) *SettingsHandler {
	return &SettingsHandler{st: st, sender: sender}
}

// settingsResp is the JSON shape returned by GET and PUT /admin/api/v1/settings.
type settingsResp struct {
	NoShowDeadlineHours  int    `json:"no_show_deadline_hours"`
	RetentionPeriodDays  int    `json:"retention_period_days"`
	ReminderLeadTimeDays int    `json:"reminder_lead_time_days"`
	Currency             string `json:"currency"`
	AppointmentLocation  string `json:"appointment_location"`
	SenderName           string `json:"sender_name"`
	TandCFilename        string `json:"tandc_filename,omitempty"`
}

// settingsInput is the JSON body accepted by PUT /admin/api/v1/settings.
// All fields are optional; unset fields leave the current value unchanged.
type settingsInput struct {
	NoShowDeadlineHours  *int    `json:"no_show_deadline_hours"`
	RetentionPeriodDays  *int    `json:"retention_period_days"`
	ReminderLeadTimeDays *int    `json:"reminder_lead_time_days"`
	Currency             *string `json:"currency"`
	AppointmentLocation  *string `json:"appointment_location"`
	SenderName           *string `json:"sender_name"`
}

func toResp(s *store.Settings) settingsResp {
	return settingsResp{
		NoShowDeadlineHours:  s.NoShowDeadlineHours,
		RetentionPeriodDays:  s.RetentionPeriodDays,
		ReminderLeadTimeDays: s.ReminderLeadTimeDays,
		Currency:             s.Currency,
		AppointmentLocation:  s.AppointmentLocation,
		SenderName:           s.SenderName,
		TandCFilename:        s.TandCFilename,
	}
}

// Get handles GET /admin/api/v1/settings.
func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	s, err := h.st.GetSettings(r.Context())
	if err != nil {
		klog.Errorf("admin/settings GET: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toResp(s))
}

// Put handles PUT /admin/api/v1/settings.
// Only fields present in the JSON body are updated; the rest keep their
// current database values.
func (h *SettingsHandler) Put(w http.ResponseWriter, r *http.Request) {
	var inp settingsInput
	if err := json.NewDecoder(r.Body).Decode(&inp); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cur, err := h.st.GetSettings(r.Context())
	if err != nil {
		klog.Errorf("admin/settings PUT: GetSettings: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if inp.NoShowDeadlineHours != nil {
		cur.NoShowDeadlineHours = *inp.NoShowDeadlineHours
	}
	if inp.RetentionPeriodDays != nil && *inp.RetentionPeriodDays > 0 {
		cur.RetentionPeriodDays = *inp.RetentionPeriodDays
	}
	if inp.ReminderLeadTimeDays != nil && *inp.ReminderLeadTimeDays > 0 {
		cur.ReminderLeadTimeDays = *inp.ReminderLeadTimeDays
	}
	if inp.Currency != nil && *inp.Currency != "" {
		cur.Currency = *inp.Currency
	}
	if inp.AppointmentLocation != nil {
		cur.AppointmentLocation = *inp.AppointmentLocation
	}
	if inp.SenderName != nil && *inp.SenderName != "" {
		cur.SenderName = *inp.SenderName
		// Propagate immediately to the email sender so the change takes effect
		// for all subsequent outgoing mails without requiring a server restart.
		if h.sender != nil {
			h.sender.SetFromName(cur.SenderName)
		}
	}

	if err := h.st.UpdateSettings(r.Context(), cur); err != nil {
		klog.Errorf("admin/settings PUT: UpdateSettings: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toResp(cur))
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		klog.Errorf("admin/settings writeJSON: %v", err)
	}
}
