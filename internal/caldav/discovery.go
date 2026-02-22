package caldav

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
)

// ── XML namespace constants (shared by discovery responses) ──────────────────

const (
	nsCalDAV = "urn:ietf:params:xml:ns:caldav"
)

// writeXML writes a pre-rendered XML string as an application/xml response.
func writeXML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprint(w, body)
}

// xmlEscape escapes special XML characters so they are safe to embed in
// attribute values or text content.
func xmlEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s)) //nolint:errcheck // strings.Builder never errors
	return b.String()
}

// NewDiscoveryHandler returns an http.Handler that implements the three
// CalDAV service-discovery steps defined by RFC 6764:
//
//  1. GET/PROPFIND /.well-known/caldav
//     → 301 permanent redirect to {rootPath}/caldav/
//
//  2. PROPFIND {rootPath}/ (the server root)
//     → 207 multistatus with <current-user-principal> pointing to
//     {rootPath}/principals/
//
//  3. PROPFIND {rootPath}/principals/
//     → 207 multistatus with <calendar-home-set> pointing to
//     {rootPath}/caldav/
//
// Mount this handler on "/.well-known/caldav", "{rootPath}/principals/",
// and wrap it into the root "/" handler (method-dispatching before the web UI).
func NewDiscoveryHandler(rootPath string) *DiscoveryHandler {
	return &DiscoveryHandler{rootPath: rootPath}
}

// DiscoveryHandler handles CalDAV discovery requests.
type DiscoveryHandler struct {
	rootPath string
}

// WellKnownHandler redirects /.well-known/caldav to the caldav collection.
func (d *DiscoveryHandler) WellKnownHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, d.rootPath+"/caldav/", http.StatusMovedPermanently)
}

// PrincipalsHandler handles requests on /principals/ and returns the
// calendar-home-set so the client knows where to find calendars.
func (d *DiscoveryHandler) PrincipalsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("DAV", "1, 2, calendar-access, calendar-schedule")
	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Allow", "OPTIONS, PROPFIND")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusNoContent)
	case "PROPFIND":
		writeXML(w, http.StatusMultiStatus, d.principalsMultistatus(r.URL.Path))
	default:
		w.Header().Set("Allow", "OPTIONS, PROPFIND")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// RootPropfindHandler handles PROPFIND on the server root ("/") and returns
// a current-user-principal response. Non-PROPFIND requests are passed through
// to the next handler (the web UI).
func (d *DiscoveryHandler) RootPropfindHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("DAV", "1, 2, calendar-access")
		writeXML(w, http.StatusMultiStatus, d.rootMultistatus())
	})
}

// ── XML responses ─────────────────────────────────────────────────────────────

func (d *DiscoveryHandler) rootMultistatus() string {
	principal := d.rootPath + "/caldav/user/"
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:c="%s">
  <d:response>
    <d:href>%s</d:href>
    <d:propstat>
      <d:prop>
        <d:current-user-principal>
          <d:href>%s</d:href>
        </d:current-user-principal>
        <d:resourcetype><d:collection/></d:resourcetype>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>
`, nsCalDAV, xmlEscape(d.rootPath+"/"), xmlEscape(principal))
}

func (d *DiscoveryHandler) principalsMultistatus(href string) string {
	calHome := d.rootPath + "/caldav/user/calendars/"
	inbox := d.rootPath + "/caldav/user/inbox/"
	outbox := d.rootPath + "/caldav/user/outbox/"
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:c="%s">
  <d:response>
    <d:href>%s</d:href>
    <d:propstat>
      <d:prop>
        <d:resourcetype><d:collection/><d:principal/></d:resourcetype>
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
        <d:displayname>Calendar Principal</d:displayname>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>
`, nsCalDAV, xmlEscape(href), xmlEscape(href), xmlEscape(calHome), xmlEscape(inbox), xmlEscape(outbox))
}
