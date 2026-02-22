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

func TestFreeBusyQueryReportReturnsVFreeBusy(t *testing.T) {
	store := calstore.NewMemoryStore()

	opaqueEvent := &calstore.Event{
		ID:         "opaque-1",
		CalendarID: "default",
		Summary:    "busy",
		Start:      time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC),
		End:        time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC),
		Opacity:    calstore.OpacityOpaque,
		Status:     calstore.StatusConfirmed,
	}
	if err := store.PutEvent(context.Background(), opaqueEvent); err != nil {
		t.Fatalf("PutEvent opaque: %v", err)
	}

	tentativeEvent := &calstore.Event{
		ID:         "tentative-1",
		CalendarID: "default",
		Summary:    "maybe",
		Start:      time.Date(2026, 2, 22, 9, 15, 0, 0, time.UTC),
		End:        time.Date(2026, 2, 22, 9, 45, 0, 0, time.UTC),
		Opacity:    calstore.OpacityOpaque,
		Status:     calstore.StatusTentative,
	}
	if err := store.PutEvent(context.Background(), tentativeEvent); err != nil {
		t.Fatalf("PutEvent tentative: %v", err)
	}

	cancelledEvent := &calstore.Event{
		ID:         "cancelled-1",
		CalendarID: "default",
		Summary:    "cancelled",
		Start:      time.Date(2026, 2, 22, 9, 20, 0, 0, time.UTC),
		End:        time.Date(2026, 2, 22, 9, 50, 0, 0, time.UTC),
		Opacity:    calstore.OpacityOpaque,
		Status:     calstore.StatusCancelled,
	}
	if err := store.PutEvent(context.Background(), cancelledEvent); err != nil {
		t.Fatalf("PutEvent cancelled: %v", err)
	}

	transparentEvent := &calstore.Event{
		ID:         "transparent-1",
		CalendarID: "default",
		Summary:    "free",
		Start:      time.Date(2026, 2, 22, 10, 30, 0, 0, time.UTC),
		End:        time.Date(2026, 2, 22, 11, 30, 0, 0, time.UTC),
		Opacity:    calstore.OpacityTransparent,
		Status:     calstore.StatusConfirmed,
	}
	if err := store.PutEvent(context.Background(), transparentEvent); err != nil {
		t.Fatalf("PutEvent transparent: %v", err)
	}

	handler := NewHandler(store, "")
	body := `<?xml version="1.0" encoding="utf-8"?>
<c:free-busy-query xmlns:c="urn:ietf:params:xml:ns:caldav">
  <c:time-range start="20260222T090000Z" end="20260222T110000Z"/>
</c:free-busy-query>`

	req := httptest.NewRequest("REPORT", "http://example.com/caldav/user/calendars/default/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/xml")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/calendar") {
		t.Fatalf("Content-Type = %q, want text/calendar", got)
	}

	resp := rec.Body.String()
	if !strings.Contains(resp, "BEGIN:VFREEBUSY") {
		t.Fatalf("missing VFREEBUSY in response: %s", resp)
	}
	if !strings.Contains(resp, "FREEBUSY:20260222T100000Z/20260222T110000Z") {
		t.Fatalf("missing clipped opaque FREEBUSY period: %s", resp)
	}
	if !strings.Contains(resp, "FREEBUSY;FBTYPE=BUSY-TENTATIVE:20260222T091500Z/20260222T094500Z") {
		t.Fatalf("missing tentative FREEBUSY period: %s", resp)
	}
	if strings.Contains(resp, "20260222T103000Z/20260222T110000Z") {
		t.Fatalf("transparent event leaked into FREEBUSY periods: %s", resp)
	}
	if strings.Contains(resp, "20260222T092000Z/20260222T095000Z") {
		t.Fatalf("cancelled event leaked into FREEBUSY periods: %s", resp)
	}
}

func TestFreeBusyQueryReportInvalidRangeReturnsBadRequest(t *testing.T) {
	handler := NewHandler(calstore.NewMemoryStore(), "")
	body := `<?xml version="1.0" encoding="utf-8"?>
<c:free-busy-query xmlns:c="urn:ietf:params:xml:ns:caldav">
</c:free-busy-query>`

	req := httptest.NewRequest("REPORT", "http://example.com/caldav/user/calendars/default/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/xml")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestFreeBusyQueryReportOnObjectReturnsForbidden(t *testing.T) {
	handler := NewHandler(calstore.NewDummyStore(), "")
	body := `<?xml version="1.0" encoding="utf-8"?>
<c:free-busy-query xmlns:c="urn:ietf:params:xml:ns:caldav">
  <c:time-range start="20260201T000000Z" end="20260301T000000Z"/>
</c:free-busy-query>`

	req := httptest.NewRequest("REPORT", "http://example.com/caldav/user/calendars/dummy/dummy-202602-01.ics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/xml")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}
