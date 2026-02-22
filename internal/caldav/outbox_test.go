package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	calstore "schedio/internal/store"
)

func TestOutboxOptionsAdvertisesPost(t *testing.T) {
	handler := NewHandler(calstore.NewMemoryStore(), "")
	req := httptest.NewRequest(http.MethodOptions, "http://example.com/caldav/user/outbox/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	allow := rec.Header().Get("Allow")
	if !strings.Contains(allow, "POST") {
		t.Fatalf("Allow header missing POST: %q", allow)
	}
	if !strings.Contains(allow, "PROPFIND") {
		t.Fatalf("Allow header missing PROPFIND: %q", allow)
	}
}

func TestOutboxPropfindIncludesSchedulingPrivileges(t *testing.T) {
	handler := NewHandler(calstore.NewMemoryStore(), "")
	req := httptest.NewRequest("PROPFIND", "http://example.com/caldav/user/outbox/", strings.NewReader(`<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"/>`))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "current-user-privilege-set") {
		t.Fatalf("missing current-user-privilege-set: %s", body)
	}
	if !strings.Contains(body, "schedule-send-freebusy") {
		t.Fatalf("missing schedule-send-freebusy privilege: %s", body)
	}
	if !strings.Contains(body, "supported-report-set") {
		t.Fatalf("missing supported-report-set: %s", body)
	}
}

func TestOutboxPostBusyRequestReturnsScheduleResponse(t *testing.T) {
	store := calstore.NewMemoryStore()
	event := &calstore.Event{
		ID:         "busy-1",
		CalendarID: "default",
		Summary:    "busy",
		Start:      time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC),
		End:        time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC),
		Opacity:    calstore.OpacityOpaque,
		Status:     calstore.StatusConfirmed,
	}
	if err := store.PutEvent(context.Background(), event); err != nil {
		t.Fatalf("PutEvent: %v", err)
	}

	handler := NewHandler(store, "")
	body := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"METHOD:REQUEST\r\n" +
		"PRODID:-//test//EN\r\n" +
		"BEGIN:VFREEBUSY\r\n" +
		"UID:fb-1\r\n" +
		"DTSTAMP:20260222T090000Z\r\n" +
		"ORGANIZER:mailto:organizer@example.com\r\n" +
		"ATTENDEE:mailto:user@example.com\r\n" +
		"DTSTART:20260222T090000Z\r\n" +
		"DTEND:20260222T110000Z\r\n" +
		"END:VFREEBUSY\r\n" +
		"END:VCALENDAR\r\n"

	req := httptest.NewRequest(http.MethodPost, "http://example.com/caldav/user/outbox/", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	resp := rec.Body.String()
	if !strings.Contains(resp, "<cal:schedule-response") {
		t.Fatalf("missing schedule-response root: %s", resp)
	}
	if !strings.Contains(resp, "mailto:user@example.com") {
		t.Fatalf("missing attendee recipient: %s", resp)
	}
	if !strings.Contains(resp, "2.0;Success") {
		t.Fatalf("missing success request-status: %s", resp)
	}
	if !strings.Contains(resp, "METHOD:REPLY") {
		t.Fatalf("missing METHOD:REPLY calendar-data: %s", resp)
	}
	if !strings.Contains(resp, "FREEBUSY;FBTYPE=BUSY:20260222T100000Z/20260222T110000Z") {
		t.Fatalf("missing clipped busy period: %s", resp)
	}
}
