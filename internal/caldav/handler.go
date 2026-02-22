package caldav

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	calstore "schedio/internal/store"

	ical "github.com/emersion/go-ical"
	extcaldav "github.com/emersion/go-webdav/caldav"
)

// davEnforcingWriter wraps an http.ResponseWriter and guarantees that the
// "DAV" response header is always set to the required capability string,
// regardless of what the inner go-webdav handler writes.  go-webdav uses
// Header().Add() which produces a second, conflicting Dav: header; this
// wrapper replaces anything the inner handler put there at WriteHeader time.
type davEnforcingWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

const davCapabilities = "1, 2, calendar-access, calendar-schedule"

func (w *davEnforcingWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.ResponseWriter.Header().Set("DAV", davCapabilities)
	w.ResponseWriter.WriteHeader(code)
}

func (w *davEnforcingWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController and HTTP/2 code access the underlying
// ResponseWriter (e.g. for Flusher, Hijacker, etc.).
func (w *davEnforcingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// NewHandler returns an http.Handler that implements the CalDAV protocol,
// backed by the provided CalendarStore.
//
// rootPath is the URL prefix under which /caldav is mounted (e.g. "" or "/ui").
// It must match the prefix used when registering the handler in the router.
func NewHandler(store calstore.CalendarStore, rootPath string) http.Handler {
	inner := &extcaldav.Handler{
		Backend: &storeAdapter{store: store, rootPath: rootPath},
		Prefix:  rootPath + "/caldav",
	}
	// Precompute paths: these are constant for the lifetime of the handler.
	principalPath := rootPath + "/caldav/user/"
	calHomePath := rootPath + "/caldav/user/calendars/"
	inboxPath := rootPath + "/caldav/user/inbox/"
	outboxPath := rootPath + "/caldav/user/outbox/"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap to ensure our DAV capabilities always appear in the response,
		// even when the inner go-webdav handler sets its own Dav: header.
		dw := &davEnforcingWriter{ResponseWriter: w}
		w = dw
		w.Header().Set("DAV", davCapabilities)

		// Minimal scheduling discovery support (RFC 6638) so iOS treats the
		// principal as scheduling-capable and enables free/busy related UI.
		if r.URL.Path == principalPath && r.Method == "PROPFIND" {
			writeXML(w, http.StatusMultiStatus, principalMultistatus(rootPath))
			return
		}
		if r.URL.Path == calHomePath && r.Method == "PROPFIND" {
			depth := strings.TrimSpace(strings.ToLower(r.Header.Get("Depth")))
			includeChildren := depth == "1" || depth == "infinity"
			ms, err := calendarHomeMultistatus(r.Context(), store, rootPath, includeChildren)
			if err != nil {
				http.Error(w, "failed to build calendar home response", http.StatusInternalServerError)
				return
			}
			writeXML(w, http.StatusMultiStatus, ms)
			return
		}
		if r.Method == "PROPFIND" {
			if calID, ok := calendarIDFromCollectionPath(r.URL.Path, rootPath); ok {
				depth := strings.TrimSpace(r.Header.Get("Depth"))
				if depth == "" || depth == "0" {
					ms, err := calendarCollectionMultistatus(r.Context(), store, rootPath, calID)
					if err != nil {
						http.Error(w, "failed to build calendar collection response", http.StatusInternalServerError)
						return
					}
					writeXML(w, http.StatusMultiStatus, ms)
					return
				}
			}
		}
		if r.Method == "REPORT" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read REPORT body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			if isFreeBusyQuery(body) {
				calID, isCollection := calendarIDFromCollectionPath(r.URL.Path, rootPath)
				if !isCollection {
					http.Error(w, "free-busy-query can only target a collection", http.StatusForbidden)
					return
				}
				vfb, fbErr := freeBusyQueryCalendarData(r.Context(), store, calID, body)
				if fbErr != nil {
					http.Error(w, fbErr.Error(), http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(vfb))
				return
			}
		}
		if (r.URL.Path == inboxPath || r.URL.Path == outboxPath) && r.Method == http.MethodOptions {
			allow := "OPTIONS, PROPFIND"
			if r.URL.Path == outboxPath {
				allow = "OPTIONS, PROPFIND, POST"
			}
			w.Header().Set("Allow", allow)
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == inboxPath && r.Method == "PROPFIND" {
			cals, _ := store.ListCalendars(r.Context())
			var calHrefs []string
			for _, c := range cals {
				calHrefs = append(calHrefs, rootPath+"/caldav/user/calendars/"+c.ID+"/")
			}
			writeXML(w, http.StatusMultiStatus, schedulingCollectionMultistatusWithCalHrefs(inboxPath, "schedule-inbox", calHrefs))
			return
		}
		if r.URL.Path == outboxPath && r.Method == "PROPFIND" {
			writeXML(w, http.StatusMultiStatus, schedulingCollectionMultistatusWithCalHrefs(outboxPath, "schedule-outbox", nil))
			return
		}
		if r.URL.Path == outboxPath && r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read scheduling request", http.StatusBadRequest)
				return
			}
			xmlResp, postErr := schedulingOutboxResponse(r.Context(), store, body)
			if postErr != nil {
				http.Error(w, postErr.Error(), http.StatusBadRequest)
				return
			}
			writeXML(w, http.StatusOK, xmlResp)
			return
		}

		// iOS often sends PROPPATCH to set display/color metadata on calendar
		// collections. go-webdav currently returns 501 for PROPPATCH, which may
		// cause clients to hide edit capabilities. Treat this as a no-op success
		// for calendar collections so clients keep writable UX enabled.
		if r.Method == "PROPPATCH" && strings.HasPrefix(r.URL.Path, rootPath+"/caldav/user/calendars/") {
			w.Header().Set("Content-Type", "application/xml; charset=utf-8")
			w.WriteHeader(http.StatusMultiStatus)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>%s</d:href>
    <d:propstat>
      <d:prop/>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>
`, r.URL.Path)
			return
		}

		// iOS Calendar sends MKCALENDAR to push its own local calendar.
		// 405 (Method Not Allowed) causes iOS to abort syncing entirely;
		// 403 (Forbidden) tells it "you can't create here" but still lets
		// it display and sync the server's existing calendars.
		if r.Method == "MKCALENDAR" {
			http.Error(w, "calendar creation not supported", http.StatusForbidden)
			return
		}
		inner.ServeHTTP(dw, r)
	})
}

func principalMultistatus(rootPath string) string {
	principal := rootPath + "/caldav/user/"
	calHome := rootPath + "/caldav/user/calendars/"
	inbox := rootPath + "/caldav/user/inbox/"
	outbox := rootPath + "/caldav/user/outbox/"

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:c="%s">
	<d:response>
		<d:href>%s</d:href>
		<d:propstat>
			<d:prop>
				<d:supported-report-set>
					<d:supported-report><d:report><d:expand-property/></d:report></d:supported-report>
					<d:supported-report><d:report><d:principal-match/></d:report></d:supported-report>
					<d:supported-report><d:report><d:principal-property-search/></d:report></d:supported-report>
					<d:supported-report><d:report><d:principal-search-property-set/></d:report></d:supported-report>
				</d:supported-report-set>
				<d:current-user-principal>
					<d:href>%s</d:href>
				</d:current-user-principal>
				<c:calendar-home-set>
					<d:href>%s</d:href>
				</c:calendar-home-set>
				<c:calendar-user-address-set>
					<d:href>mailto:user@example.com</d:href>
				</c:calendar-user-address-set>
				<c:schedule-inbox-URL>
					<d:href>%s</d:href>
				</c:schedule-inbox-URL>
				<c:schedule-outbox-URL>
					<d:href>%s</d:href>
				</c:schedule-outbox-URL>
				<d:resourcetype><d:collection/><d:principal/></d:resourcetype>
			</d:prop>
			<d:status>HTTP/1.1 200 OK</d:status>
		</d:propstat>
	</d:response>
</d:multistatus>
`, nsCalDAV, xmlEscape(principal), xmlEscape(principal), xmlEscape(calHome), xmlEscape(inbox), xmlEscape(outbox))
}

func schedulingCollectionMultistatusWithCalHrefs(path, kind string, calHrefs []string) string {
	resource := "<d:collection/>"
	if kind == "schedule-inbox" {
		resource += "<c:schedule-inbox/>"
	} else {
		resource += "<c:schedule-outbox/>"
	}
	additional := ""
	if kind == "schedule-inbox" && len(calHrefs) > 0 {
		var hrefs string
		for _, h := range calHrefs {
			hrefs += "<d:href>" + xmlEscape(h) + "</d:href>"
		}
		additional += `
				<c:calendar-free-busy-set>` + hrefs + `</c:calendar-free-busy-set>`
	}
	if kind == "schedule-outbox" {
		additional = `<d:current-user-privilege-set>
					<d:privilege><d:read/></d:privilege>
					<d:privilege><d:read-acl/></d:privilege>
					<d:privilege><d:read-current-user-privilege-set/></d:privilege>
					<d:privilege><c:schedule-send/></d:privilege>
					<d:privilege><c:schedule-send-invite/></d:privilege>
					<d:privilege><c:schedule-send-reply/></d:privilege>
					<d:privilege><c:schedule-send-freebusy/></d:privilege>
				</d:current-user-privilege-set>
				<d:supported-report-set>
					<d:supported-report><d:report><d:expand-property/></d:report></d:supported-report>
					<d:supported-report><d:report><d:principal-match/></d:report></d:supported-report>
					<d:supported-report><d:report><d:principal-property-search/></d:report></d:supported-report>
					<d:supported-report><d:report><d:principal-search-property-set/></d:report></d:supported-report>
				</d:supported-report-set>`
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:c="%s">
	<d:response>
		<d:href>%s</d:href>
		<d:propstat>
			<d:prop>
				<d:resourcetype>%s</d:resourcetype>%s
			</d:prop>
			<d:status>HTTP/1.1 200 OK</d:status>
		</d:propstat>
	</d:response>
</d:multistatus>
`, nsCalDAV, xmlEscape(path), resource, additional)
}

func calendarHomeMultistatus(ctx context.Context, store calstore.CalendarStore, rootPath string, includeChildren bool) (string, error) {
	home := rootPath + "/caldav/user/calendars/"
	inbox := rootPath + "/caldav/user/inbox/"
	outbox := rootPath + "/caldav/user/outbox/"

	cals, err := store.ListCalendars(ctx)
	if err != nil {
		return "", err
	}

	// Compute the default calendar href (first calendar) and the free-busy hrefs
	// for the schedule-inbox so iOS knows which calendars participate in scheduling.
	defaultCalHref := ""
	freeBusyHrefs := ""
	for _, c := range cals {
		h := home + c.ID + "/"
		freeBusyHrefs += "<d:href>" + xmlEscape(h) + "</d:href>"
		if defaultCalHref == "" {
			defaultCalHref = h
		}
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav" xmlns:cs="http://calendarserver.org/ns/" xmlns:s="http://sabredav.org/ns">`)

	defaultCalXML := ""
	if defaultCalHref != "" {
		defaultCalXML = "\n\t\t\t<cal:schedule-default-calendar-URL><d:href>" + xmlEscape(defaultCalHref) + "</d:href></cal:schedule-default-calendar-URL>"
	}
	b.WriteString(`<d:response>
	<d:href>` + xmlEscape(home) + `</d:href>
	<d:propstat>
		<d:prop>
			<d:resourcetype><d:collection/></d:resourcetype>
			<d:supported-report-set>
				<d:supported-report><d:report><d:expand-property/></d:report></d:supported-report>
				<d:supported-report><d:report><d:principal-match/></d:report></d:supported-report>
				<d:supported-report><d:report><d:principal-property-search/></d:report></d:supported-report>
				<d:supported-report><d:report><d:principal-search-property-set/></d:report></d:supported-report>
				<d:supported-report><d:report><d:sync-collection/></d:report></d:supported-report>
			</d:supported-report-set>` + defaultCalXML + `
		</d:prop>
		<d:status>HTTP/1.1 200 OK</d:status>
	</d:propstat>
</d:response>`)

	if !includeChildren {
		b.WriteString(`</d:multistatus>`)
		return b.String(), nil
	}

	for _, cal := range cals {
		href := home + cal.ID + "/"
		ctag, ctagErr := store.CTag(ctx, cal.ID)
		if ctagErr != nil || ctag == "" {
			ctag = "0"
		}
		tz := cal.Timezone
		if tz == "" {
			tz = "UTC"
		}

		b.WriteString(`<d:response>
	<d:href>` + xmlEscape(href) + `</d:href>
	<d:propstat>
		<d:prop>
			<d:resourcetype><d:collection/><cal:calendar/></d:resourcetype>
			<d:supported-report-set>
				<d:supported-report><d:report><d:expand-property/></d:report></d:supported-report>
				<d:supported-report><d:report><d:principal-match/></d:report></d:supported-report>
				<d:supported-report><d:report><d:principal-property-search/></d:report></d:supported-report>
				<d:supported-report><d:report><d:principal-search-property-set/></d:report></d:supported-report>
				<d:supported-report><d:report><d:sync-collection/></d:report></d:supported-report>
				<d:supported-report><d:report><cal:calendar-multiget/></d:report></d:supported-report>
				<d:supported-report><d:report><cal:calendar-query/></d:report></d:supported-report>
				<d:supported-report><d:report><cal:free-busy-query/></d:report></d:supported-report>
			</d:supported-report-set>
			<cs:getctag>` + xmlEscape(ctag) + `</cs:getctag>
			<s:sync-token>` + xmlEscape(ctag) + `</s:sync-token>
			<cal:supported-calendar-component-set>
				<cal:comp name="VEVENT"/>
			</cal:supported-calendar-component-set>
			<cal:schedule-calendar-transp><cal:opaque/></cal:schedule-calendar-transp>
			<d:displayname>` + xmlEscape(cal.Name) + `</d:displayname>
			<cal:calendar-description>` + xmlEscape(cal.Description) + `</cal:calendar-description>
			<cal:calendar-timezone>` + xmlEscape(tz) + `</cal:calendar-timezone>
			<x1:calendar-order xmlns:x1="http://apple.com/ns/ical/">0</x1:calendar-order>
			<x1:calendar-color xmlns:x1="http://apple.com/ns/ical/">` + xmlEscape(cal.Color) + `</x1:calendar-color>
			<d:current-user-privilege-set>
				<d:privilege><d:read/></d:privilege>
				<d:privilege><d:write/></d:privilege>
				<d:privilege><d:write-properties/></d:privilege>
				<d:privilege><d:write-content/></d:privilege>
				<d:privilege><cal:read-free-busy/></d:privilege>
			</d:current-user-privilege-set>
		</d:prop>
		<d:status>HTTP/1.1 200 OK</d:status>
	</d:propstat>
</d:response>`)
	}

	for _, extra := range []struct {
		href string
		tag  string
	}{{inbox, "schedule-inbox"}, {outbox, "schedule-outbox"}} {
		extraProps := ""
		if extra.tag == "schedule-inbox" && freeBusyHrefs != "" {
			extraProps = "\n\t\t\t<cal:calendar-free-busy-set>" + freeBusyHrefs + "</cal:calendar-free-busy-set>"
		}
		b.WriteString(`<d:response>
	<d:href>` + xmlEscape(extra.href) + `</d:href>
	<d:propstat>
		<d:prop>
			<d:resourcetype><d:collection/><cal:` + extra.tag + `/></d:resourcetype>` + extraProps + `
		</d:prop>
		<d:status>HTTP/1.1 200 OK</d:status>
	</d:propstat>
</d:response>`)
	}

	b.WriteString(`</d:multistatus>`)
	return b.String(), nil
}

func calendarIDFromCollectionPath(path, rootPath string) (string, bool) {
	prefix := rootPath + "/caldav/user/calendars/"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, "/") {
		return "", false
	}
	rel := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if rel == "" || strings.Contains(rel, "/") {
		return "", false
	}
	return rel, true
}

func calendarCollectionMultistatus(ctx context.Context, store calstore.CalendarStore, rootPath, calID string) (string, error) {
	cal, err := store.GetCalendar(ctx, calID)
	if err != nil {
		return "", err
	}
	href := rootPath + "/caldav/user/calendars/" + cal.ID + "/"

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">
	<d:response>
		<d:href>%s</d:href>
		<d:propstat>
			<d:prop>
				<d:resourcetype><d:collection/><cal:calendar/></d:resourcetype>
				<d:supported-report-set>
					<d:supported-report><d:report><d:expand-property/></d:report></d:supported-report>
					<d:supported-report><d:report><d:principal-match/></d:report></d:supported-report>
					<d:supported-report><d:report><d:principal-property-search/></d:report></d:supported-report>
					<d:supported-report><d:report><d:principal-search-property-set/></d:report></d:supported-report>
					<d:supported-report><d:report><d:sync-collection/></d:report></d:supported-report>
					<d:supported-report><d:report><cal:calendar-multiget/></d:report></d:supported-report>
					<d:supported-report><d:report><cal:calendar-query/></d:report></d:supported-report>
					<d:supported-report><d:report><cal:free-busy-query/></d:report></d:supported-report>
				</d:supported-report-set>
				<d:displayname>%s</d:displayname>
				<cal:calendar-description>%s</cal:calendar-description>
				<cal:supported-calendar-component-set>
					<cal:comp name="VEVENT"/>
				</cal:supported-calendar-component-set>
				<cal:supported-calendar-data>
					<cal:calendar-data content-type="text/calendar" version="2.0"/>
				</cal:supported-calendar-data>
				<cal:schedule-calendar-transp><cal:opaque/></cal:schedule-calendar-transp>
				<d:current-user-privilege-set>
					<d:privilege><d:read/></d:privilege>
					<d:privilege><d:write/></d:privilege>
					<d:privilege><d:write-properties/></d:privilege>
					<d:privilege><d:write-content/></d:privilege>
					<d:privilege><cal:read-free-busy/></d:privilege>
				</d:current-user-privilege-set>
			</d:prop>
			<d:status>HTTP/1.1 200 OK</d:status>
		</d:propstat>
	</d:response>
</d:multistatus>
`, xmlEscape(href), xmlEscape(cal.Name), xmlEscape(cal.Description)), nil
}

func isFreeBusyQuery(body []byte) bool {
	return strings.Contains(strings.ToLower(string(body)), "free-busy-query")
}

func freeBusyQueryCalendarData(ctx context.Context, store calstore.CalendarStore, calID string, body []byte) (string, error) {
	start, end, err := parseFreeBusyTimeRange(body)
	if err != nil {
		return "", err
	}
	events, err := store.ListEvents(ctx, calID, start, end)
	if err != nil {
		return "", err
	}

	return buildFreeBusyCalendar(calID, start, end, events), nil
}

func parseFreeBusyTimeRange(body []byte) (time.Time, time.Time, error) {
	type tr struct {
		Start string `xml:"start,attr"`
		End   string `xml:"end,attr"`
	}

	dec := xml.NewDecoder(bytes.NewReader(body))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid REPORT XML")
		}
		startEl, ok := tok.(xml.StartElement)
		if !ok || startEl.Name.Local != "time-range" {
			continue
		}
		var rangeVal tr
		if err := dec.DecodeElement(&rangeVal, &startEl); err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid free-busy time-range")
		}
		if rangeVal.Start == "" || rangeVal.End == "" {
			return time.Time{}, time.Time{}, fmt.Errorf("missing free-busy time-range start/end")
		}
		start, err := time.Parse("20060102T150405Z", rangeVal.Start)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid free-busy start")
		}
		end, err := time.Parse("20060102T150405Z", rangeVal.End)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid free-busy end")
		}
		if !start.Before(end) {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid free-busy time-range")
		}
		return start.UTC(), end.UTC(), nil
	}

	return time.Time{}, time.Time{}, fmt.Errorf("missing free-busy time-range")
}

func buildFreeBusyCalendar(calID string, start, end time.Time, events []*calstore.Event) string {
	periods, tentativePeriods := collectBusyPeriods(events, start, end)

	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("PRODID:-//schedio//schedio//EN\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("BEGIN:VFREEBUSY\r\n")
	b.WriteString("UID:freebusy-" + calID + "\r\n")
	b.WriteString("DTSTAMP:" + time.Now().UTC().Format("20060102T150405Z") + "\r\n")
	b.WriteString("DTSTART:" + start.UTC().Format("20060102T150405Z") + "\r\n")
	b.WriteString("DTEND:" + end.UTC().Format("20060102T150405Z") + "\r\n")
	if len(periods) > 0 {
		b.WriteString("FREEBUSY:" + strings.Join(periods, ",") + "\r\n")
	}
	if len(tentativePeriods) > 0 {
		b.WriteString("FREEBUSY;FBTYPE=BUSY-TENTATIVE:" + strings.Join(tentativePeriods, ",") + "\r\n")
	}
	b.WriteString("END:VFREEBUSY\r\n")
	b.WriteString("END:VCALENDAR\r\n")

	return b.String()
}

func collectBusyPeriods(events []*calstore.Event, start, end time.Time) ([]string, []string) {
	periods := make([]string, 0, len(events))
	tentativePeriods := make([]string, 0)
	for _, event := range events {
		if event.Opacity == calstore.OpacityTransparent {
			continue
		}
		if event.Status == calstore.StatusCancelled {
			continue
		}
		periodStart := event.Start.UTC()
		periodEnd := event.End.UTC()
		if periodStart.Before(start) {
			periodStart = start
		}
		if periodEnd.After(end) {
			periodEnd = end
		}
		if !periodStart.Before(periodEnd) {
			continue
		}
		period := periodStart.Format("20060102T150405Z") + "/" + periodEnd.Format("20060102T150405Z")
		if event.Status == calstore.StatusTentative {
			tentativePeriods = append(tentativePeriods, period)
			continue
		}
		periods = append(periods, period)
	}
	return periods, tentativePeriods
}

func schedulingOutboxResponse(ctx context.Context, store calstore.CalendarStore, body []byte) (string, error) {
	req, err := parseSchedulingOutboxRequest(body)
	if err != nil {
		return "", err
	}

	cals, err := store.ListCalendars(ctx)
	if err != nil {
		return "", err
	}
	allEvents := make([]*calstore.Event, 0)
	for _, cal := range cals {
		events, listErr := store.ListEvents(ctx, cal.ID, req.Start, req.End)
		if listErr != nil {
			continue
		}
		allEvents = append(allEvents, events...)
	}
	busy, tentative := collectBusyPeriods(allEvents, req.Start, req.End)

	var responses strings.Builder
	for _, attendee := range req.Attendees {
		calendarData := buildSchedulingFreeBusyReply(req.UID, req.Start, req.End, req.Organizer, attendee, busy, tentative)
		responses.WriteString(`<cal:response>
		<cal:recipient><d:href>` + xmlEscape(attendee) + `</d:href></cal:recipient>
		<cal:request-status>2.0;Success</cal:request-status>
		<cal:calendar-data>` + xmlEscape(calendarData) + `</cal:calendar-data>
	</cal:response>`)
	}

	return `<?xml version="1.0" encoding="UTF-8"?>
<cal:schedule-response xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">` + responses.String() + `
</cal:schedule-response>
`, nil
}

type outboxBusyRequest struct {
	UID       string
	Organizer string
	Attendees []string
	Start     time.Time
	End       time.Time
}

func parseSchedulingOutboxRequest(body []byte) (*outboxBusyRequest, error) {
	cal, err := ical.NewDecoder(bytes.NewReader(body)).Decode()
	if err != nil {
		return nil, fmt.Errorf("invalid scheduling message: %w", err)
	}
	method, _ := cal.Props.Text(ical.PropMethod)
	if strings.ToUpper(strings.TrimSpace(method)) != "REQUEST" {
		return nil, fmt.Errorf("invalid scheduling message method")
	}

	var vfreebusy *ical.Component
	for _, child := range cal.Children {
		if strings.EqualFold(child.Name, ical.CompFreeBusy) {
			vfreebusy = child
			break
		}
	}
	if vfreebusy == nil {
		return nil, fmt.Errorf("missing VFREEBUSY component")
	}

	uid, _ := vfreebusy.Props.Text(ical.PropUID)
	// ORGANIZER and ATTENDEE are CAL-ADDRESS type; Props.Text() rejects them
	// because it enforces VALUE=TEXT.  Read the raw .Value directly instead.
	var organizer string
	if p := vfreebusy.Props.Get(ical.PropOrganizer); p != nil {
		organizer = p.Value
	}
	if uid == "" || organizer == "" {
		return nil, fmt.Errorf("invalid scheduling message: missing UID or ORGANIZER")
	}
	attendeeRawProps := vfreebusy.Props.Values(ical.PropAttendee)
	if len(attendeeRawProps) == 0 {
		return nil, fmt.Errorf("missing attendees")
	}
	attendees := make([]string, 0, len(attendeeRawProps))
	for _, ap := range attendeeRawProps {
		if strings.TrimSpace(ap.Value) == "" {
			continue
		}
		attendees = append(attendees, ap.Value)
	}
	if len(attendees) == 0 {
		return nil, fmt.Errorf("missing attendees")
	}

	start, err := vfreebusy.Props.DateTime(ical.PropDateTimeStart, time.UTC)
	if err != nil {
		return nil, fmt.Errorf("invalid DTSTART")
	}
	end, err := vfreebusy.Props.DateTime(ical.PropDateTimeEnd, time.UTC)
	if err != nil {
		return nil, fmt.Errorf("invalid DTEND")
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("invalid time-range")
	}

	return &outboxBusyRequest{
		UID:       uid,
		Organizer: organizer,
		Attendees: attendees,
		Start:     start.UTC(),
		End:       end.UTC(),
	}, nil
}

func buildSchedulingFreeBusyReply(uid string, start, end time.Time, organizer, attendee string, busy []string, tentative []string) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//schedio//schedio//EN\r\n")
	b.WriteString("METHOD:REPLY\r\n")
	b.WriteString("BEGIN:VFREEBUSY\r\n")
	b.WriteString("UID:" + uid + "\r\n")
	b.WriteString("DTSTAMP:" + time.Now().UTC().Format("20060102T150405Z") + "\r\n")
	b.WriteString("DTSTART:" + start.UTC().Format("20060102T150405Z") + "\r\n")
	b.WriteString("DTEND:" + end.UTC().Format("20060102T150405Z") + "\r\n")
	b.WriteString("ORGANIZER:" + organizer + "\r\n")
	b.WriteString("ATTENDEE:" + attendee + "\r\n")
	if len(busy) > 0 {
		b.WriteString("FREEBUSY;FBTYPE=BUSY:" + strings.Join(busy, ",") + "\r\n")
	}
	if len(tentative) > 0 {
		b.WriteString("FREEBUSY;FBTYPE=BUSY-TENTATIVE:" + strings.Join(tentative, ",") + "\r\n")
	}
	b.WriteString("END:VFREEBUSY\r\n")
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}
