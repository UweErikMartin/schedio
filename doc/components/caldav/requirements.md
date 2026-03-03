# CalDAV Facade — Component Requirements

## 1. Purpose and Scope

The CalDAV facade provides a standards-compliant CalDAV endpoint at `/caldav/`
that allows Staff users to manage their availability and monitor booking events
using any CalDAV-capable calendar client, in particular Apple Calendar on iOS
and iPadOS.

The facade is **not** an independent data store. It is a read/write view over the
domain store (PostgreSQL in production, `MemoryStore` in development). All domain
data — timeslots and bookings — originate from and are persisted to the domain
store. The CalDAV protocol surface is translated into domain operations; no
CalDAV-native persistence is introduced.

**Primary client:** Apple Calendar / iOS Calendar (tested against iPadOS 18).  
**Secondary clients:** any RFC 4791–compliant CalDAV client.

---

## 2. Authentication

### CDV-AUTH-1 — HTTP Basic Authentication
The CalDAV endpoint (`/caldav/`) must enforce HTTP Basic authentication on every
request except `OPTIONS`. The credential check must validate the presented
password against the Staff user's stored bcrypt password hash (cost ≥ 12).

### CDV-AUTH-2 — Authenticated Identity
After successful authentication the Staff user's identity (email, display name)
must be injected into the request context and used to scope all store queries
to that user's calendars.

### CDV-AUTH-3 — Development Bypass
A `--no-auth` flag (or equivalent startup option) must allow bypassing Basic
Auth for local development over plain HTTP. When bypass is active the first Staff
user in the store must be injected as the authenticated principal. This mode
must never be enabled in production.

### CDV-AUTH-4 — Session Cookie Not Required
The CalDAV endpoint must not require the `schedio_session` cookie used by the
admin web UI. CalDAV clients authenticate exclusively via HTTP Basic.

---

## 3. Service Discovery

Service discovery follows RFC 6764 and RFC 4791 §6. The following sequence must
succeed end-to-end for a CalDAV client to activate the account.

### CDV-DISC-1 — Well-Known Redirect
`GET` or `PROPFIND /.well-known/caldav` must return `301 Moved Permanently`
redirecting to `{rootPath}/caldav/`.

### CDV-DISC-2 — Root Current-User-Principal
`PROPFIND {rootPath}/` must respond `207 Multi-Status` containing the
`<DAV:current-user-principal>` href pointing to the authenticated user's
principal path (`{rootPath}/caldav/user/`). Non-`PROPFIND` requests on `/`
must be passed through to the web UI handler.

### CDV-DISC-3 — Principal Resource
`PROPFIND {rootPath}/caldav/user/` must respond `207 Multi-Status` with all of
the following properties:

| Property | Namespace | Required value |
| --- | --- | --- |
| `calendar-home-set` | `caldav` | `{rootPath}/caldav/user/calendars/` |
| `schedule-inbox-URL` | `caldav` | `{rootPath}/caldav/user/inbox/` |
| `schedule-outbox-URL` | `caldav` | `{rootPath}/caldav/user/outbox/` |
| `schedule-default-calendar-URL` | `caldav` | href of the first calendar; omitted (not empty) when no calendars exist |
| `calendar-user-address-set` | `caldav` | `mailto:<user-email>` |
| `calendar-user-type` | `caldav` | `INDIVIDUAL` |
| `current-user-privilege-set` | `DAV` | Must include `read`, `write`, `write-properties`, `write-content`, `read-acl`, `read-current-user-privilege-set`, `read-free-busy` |
| `current-user-principal` | `DAV` | Self-referential href |
| `resourcetype` | `DAV` | `<collection/><principal/>` |
| `displayname` | `DAV` | Human-readable display name of the authenticated user |

### CDV-DISC-4 — Principals Path Support
`PROPFIND {rootPath}/principals/` (without authentication) must respond `207`
containing a `calendar-home-set` href pointing to
`{rootPath}/caldav/user/calendars/`. This path is used by some CalDAV clients
for initial setup without credentials.

### CDV-DISC-5 — CalDAV Root PROPFIND
`PROPFIND {rootPath}/caldav/` must respond `207 Multi-Status` containing
`<DAV:current-user-principal>` so the client completes the Basic Auth
challenge–response cycle and stores credentials for subsequent requests.

---

## 4. DAV Capability Header

### CDV-DAV-1 — Required Tokens
Every response from the CalDAV endpoint (including service-discovery responses)
must include the header:

```
DAV: 1, 2, calendar-access, calendar-auto-schedule
```

The exact string `calendar-auto-schedule` (not `calendar-schedule`) must appear.
iOS Calendar checks for this token to enable the per-event "Show As Free/Busy"
(`TRANSP`) editing field.

### CDV-DAV-2 — Header Enforcement
The `DAV:` header value must be locked to the required capability string;
the inner protocol library must not be allowed to produce a different or
conflicting value.

---

## 5. Calendar Layout

### CDV-CAL-1 — Fixed Calendar Set
Each authenticated Staff user has exactly two calendars served under
`{rootPath}/caldav/user/calendars/`:

| Calendar ID | Name | Writable by CalDAV client |
| --- | --- | --- |
| `timeslots` | Timeslots | Yes |
| `bookings` | Bookings | No |

The calendar set is determined by the server; clients must not be permitted to
create new calendars (`MKCALENDAR`).

### CDV-CAL-2 — Calendar Creation Forbidden
`MKCALENDAR` requests must be rejected with `403 Forbidden`. The response must
not abort client synchronisation (i.e. `405 Method Not Allowed` must not be
returned, as some clients interpret it as a fatal error and stop syncing).

### CDV-CAL-3 — Timeslots Calendar
The **Timeslots** calendar represents the Staff user's general, service-agnostic
availability windows. All CRUD operations initiated by the CalDAV client on this
calendar must be translated into domain store operations on `Timeslot` records.
See §6 (Timeslot Write Path).

### CDV-CAL-4 — Bookings Calendar
The **Bookings** calendar is a read-only computed view of domain `Booking`
records. Booking events are created exclusively by the domain layer. `PUT` and
`DELETE` requests targeting the bookings calendar must be rejected with
`405 Method Not Allowed`.

---

## 6. Timeslot Write Path

### CDV-TS-0 — Event Type Taxonomy
The CalDAV protocol distinguishes four kinds of timeslot representations, each
of which requires different storage and deletion handling. All four may appear
in the Timeslots calendar.

| Type | CalDAV mechanism | Identifying signal | Meaning |
| --- | --- | --- | --- |
| **Single event** | Separate `.ics` resource | `UID`, no `RRULE`, no `RECURRENCE-ID` | One discrete availability window. |
| **Recurring series root** | Single `.ics` resource for all occurrences | `UID` + `RRULE`, no `RECURRENCE-ID` | A repeating availability pattern. The client expands the rule locally into individual occurrences. |
| **Recurring override** | Separate `.ics` resource (different filename, same calendar) | Same `UID` as the series root + `RECURRENCE-ID` | Replaces exactly one occurrence of the parent series. `RECURRENCE-ID` equals the original `DTSTART` of the overridden occurrence. All other occurrences remain governed by the series root. |
| **Recurrence exclude** | `EXDATE` property added to the series root `.ics` via `PUT` | `RRULE` still present; new `EXDATE` value(s) in the updated series root | Removes one or more specific occurrences from the series without creating a new resource. The excluded dates appear as free slots in the client. The client implements this by sending a `PUT` to the series root resource with the additional `EXDATE` property appended — **no `DELETE` request is issued**. |

The server must distinguish all four types:
- Types 1–3 are identified by the presence or absence of `RRULE` and
  `RECURRENCE-ID` on the resource being written or deleted.
- Type 4 (recurrence exclude) is identified during `PUT` handling by
  comparing the `EXDATE` values in the incoming payload with those currently
  stored for the series root: any newly added `EXDATE` date-times are newly
  excluded occurrences.

### CDV-TS-1 — PUT (Create / Update)
When a CalDAV client sends `PUT /caldav/user/calendars/timeslots/{uid}.ics`:

1. Parse the iCal payload from the request body, extracting: `UID`, `SUMMARY`,
   `DTSTART`, `DTEND`, `RRULE`, `RECURRENCE-ID`, `EXDATE` (zero or more),
   `LAST-MODIFIED`.
2. Determine the event type (CDV-TS-0) and apply type-specific pre-processing
   before persisting:

   - **Single event or recurring override** — no pre-processing; proceed to
     step 3.

   - **Recurring series root — initial creation** (no existing record in the
     store): persist the payload as-is, including any `EXDATE` values already
     present. No per-occurrence conflict check is performed for those initial
     `EXDATE` values because no bookings can exist in a brand-new series.
     Proceed to step 3; skip step 4 (no prior time window to compare against).

   - **Recurring series root — update** (an existing record is found): diff the
     incoming `EXDATE` set against those stored for the series root and handle
     each of the two possible delta cases before calling `UpsertTimeslot`:

     **a) Newly added `EXDATE` values** (present in payload, absent from store):
     For each such date-time, this is a recurrence exclude (type 4):
     1. Compute the occurrence window: start = the `EXDATE` date-time;
        end = start + (series root `DTEND` − series root `DTSTART`) i.e. the
        same duration as a normal series occurrence.
     2. If an override record exists for this `RECURRENCE-ID` date-time, use the
        override's own `[DTSTART, DTEND)` as the conflict-check window instead
        (it may differ from the series duration), then
        **delete that override record** via
        `DomainStore.DeleteTimeslotOverride(staffID, uid, recurrenceID)`.
     3. Perform a conflict check over the resolved window (see CDV-TS-3).

     **b) Removed `EXDATE` values** (present in store, absent from payload):
     For each such date-time, the staff member is re-enabling a previously
     excluded occurrence:
     1. The occurrence reverts to being a normal series occurrence governed
        solely by the `RRULE`. No conflict check is needed (availability is
        being restored, not reduced).
     2. If an orphaned override record exists for this `RECURRENCE-ID`
        date-time, **delete it** via
        `DomainStore.DeleteTimeslotOverride(staffID, uid, recurrenceID)`.
        The re-enabled series occurrence takes precedence over any orphaned
        override.

3. Call `DomainStore.UpsertTimeslot` with the authenticated Staff user's ID,
   persisting the full updated record including the final `EXDATE` list.
4. If `DTSTART`, `DTEND`, or `RRULE` changed (not an `EXDATE`-only update),
   perform a broad conflict check via
   `DomainStore.ListActiveBookingsInWindow(staffID, start, end)` over the
   window covered by the old and/or new event time range (see CDV-TS-3).
   For `EXDATE`-only updates this step is skipped; the per-occurrence checks
   in step 2 are sufficient.
5. Return `201 Created` (new resource) or `204 No Content` (update) with a
   server-assigned `ETag` value.

### CDV-TS-2 — DELETE
When a CalDAV client sends `DELETE /caldav/user/calendars/timeslots/{uid}.ics`
the server must inspect the stored record identified by the path to determine
which event type (CDV-TS-0) is being deleted and apply the corresponding
behaviour. **Recurrence excludes (type 4) are never delivered as `DELETE`
requests**; they arrive as `PUT` requests on the series root (see CDV-TS-1).
`DELETE` handling therefore covers only types 1–3.

#### Case A — Single event
The resource has no `RRULE` and no `RECURRENCE-ID`.

1. Perform a conflict check for the window `[DTSTART, DTEND)` of the deleted
   event (see CDV-TS-3).
2. Call `DomainStore.DeleteTimeslot(staffID, uid)` to remove the record.
3. Return `204 No Content`.

#### Case B — Recurring series root
The resource has an `RRULE` and no `RECURRENCE-ID`. Deleting it removes the
entire repeating series, including all override records that share the same
`UID`.

1. Perform a conflict check across the effective occurrences of the series.
   `EXDATE`-excluded dates must be **skipped** in this scan because they were
   never offered as bookable slots and cannot have active bookings against them:
   - If the `RRULE` has a bounded end (`UNTIL` or `COUNT`), enumerate all
     non-excluded occurrences, compute each window as
     `[occurrence DTSTART, occurrence DTSTART + series duration)`, and call
     `DomainStore.ListActiveBookingsInWindow` for each (or equivalently for the
     combined span, filtering out excluded dates on the domain side).
   - If the `RRULE` is unbounded, use
     `DomainStore.ListActiveBookingsForTimeslotUID(staffID, uid)` which returns
     all bookings associated with any timeslot record sharing this `UID`,
     inherently ignoring excluded dates since those dates were never bookable.
2. Delete all override records for this series by calling
   `DomainStore.DeleteTimeslotOverrides(staffID, uid)`.
3. Delete the series root by calling `DomainStore.DeleteTimeslot(staffID, uid)`.
4. Return `204 No Content`.

#### Case C — Recurring override
The resource has a `RECURRENCE-ID`. Deleting it removes only that single
overridden occurrence; the parent series and all other overrides are unaffected.
The occurrence at the original `RECURRENCE-ID` datetime reverts to being
governed by the parent series `RRULE` (i.e. it reappears as a normal series
occurrence, unless the series root carries an `EXDATE` for that date, in which
case the occurrence remains excluded and no timeslot becomes available).

1. Perform a conflict check that covers **both** of the following windows
   (see CDV-TS-3), because the revert changes the available window:
   - The override's modified window: `[override DTSTART, override DTEND)`.
   - The original series-occurrence window: `[RECURRENCE-ID datetime,
     RECURRENCE-ID datetime + series duration)` — only if this window differs
     from the override window (i.e. the override shifted the time). If the
     `RECURRENCE-ID` date-time is covered by an `EXDATE` on the series root,
     skip this second check because the occurrence will remain excluded after
     the override is removed.
2. Call `DomainStore.DeleteTimeslotOverride(staffID, uid, recurrenceID)` to
   remove only this override record.
3. Return `204 No Content`.

#### Error cases (all three)
- If no timeslot record matching the path is found: return `404 Not Found`.
- If an `If-Match` precondition is present and the stored ETag does not match:
  return `412 Precondition Failed` before performing any deletion or conflict
  check.

### CDV-TS-3 — Conflict Notification
Whenever a timeslot is created, modified, or deleted, the server must check
whether any active booking (state `reserved` or `confirmed`) falls within the
affected time window by calling
`DomainStore.ListActiveBookingsInWindow(staffID, start, end)`. If one or more
conflicts are found, an admin-conflict notification email must be sent to all
Staff users. The email must list each conflicting booking with:
- appointment date and time,
- service name,
- customer name, email address, and telephone number.

The timeslot operation itself must not be rolled back due to existing booking
conflicts; the conflict notification is informational only.

### CDV-TS-4 — ETag Concurrency
Optimistic concurrency must be supported via the standard HTTP `If-Match`
(update-only) and `If-None-Match: *` (create-only) precondition headers:

- `If-None-Match: *` — reject with `412 Precondition Failed` if the resource
  already exists.
- `If-Match: <etag>` — reject with `412 Precondition Failed` if the stored
  ETag does not match the presented value.

### CDV-TS-5 — Recurring Event Storage
The domain store must persist all four event types defined in CDV-TS-0:

- **Single event** — a `Timeslot` record with no `rrule` and no
  `recurrence_id`.
- **Recurring series root** — a `Timeslot` record with the raw `RRULE` string
  and no `recurrence_id`. The `RRULE` value is stored verbatim; occurrence
  expansion is performed at query time by the availability logic, not by the
  CalDAV layer.
- **Recurring override** — a separate `Timeslot` record with the same `UID` as
  the series root, a non-null `recurrence_id` (equal to the original occurrence
  `DTSTART`), and its own `DTSTART`/`DTEND`. Must not modify the series root
  record.
- **Recurrence exclude** — represented not as a separate record but as one or
  more `EXDATE` date-time values stored on the series root `Timeslot` record
  (e.g. in an `exdates` array column or a normalised child table). The
  availability calculator must skip all `EXDATE` date-times when computing
  bookable slots. When a series root is returned to a CalDAV client via
  `GET` or `REPORT`, all stored `EXDATE` values must be included in the
  serialised iCal payload so the client's local expansion matches the server's
  view.

---

## 7. Booking Read Path (Bookings Calendar)

### CDV-BK-1 — VEVENT Synthesis
Each domain `Booking` record must be presented to the CalDAV client as a
synthesised `VEVENT` component. The iCal properties must be derived as follows:

| iCal property | Source |
| --- | --- |
| `UID` | `booking.caldav_uid` (booking ID) |
| `SUMMARY` | `service.name` |
| `DTSTART` | `booking.start_at` (UTC, `Z` suffix) |
| `DTEND` | `booking.end_at` (UTC, `Z` suffix) |
| `STATUS` | `TENTATIVE` (reserved) · `CONFIRMED` (confirmed) · `CANCELLED` (all cancelled variants) |
| `ORGANIZER` | `ADMIN_EMAIL` env var |
| `LOCATION` | `settings.appointment_location` |
| `DESCRIPTION` | `service.description`, booking reference, and price |
| `SEQUENCE` | `booking.sequence` |
| `URL` | `https://{host}/admin/session/{sessionID}` (admin session review link) |
| `TRANSP` | `OPAQUE` for reserved/confirmed · `TRANSPARENT` for cancelled |
| `DTSTAMP` | `booking.updated_at` (or current time when unset) |

### CDV-BK-2 — Time-Range Filtering
`REPORT` (calendar-query) requests on the bookings calendar must pre-filter
by the requested time range before synthesising VEVENT objects to avoid
generating all events in memory when only a narrow range is needed.

### CDV-BK-3 — Status Mapping
The booking state machine maps to iCal `STATUS` as follows:

| Domain state | iCal STATUS |
| --- | --- |
| `reserved` | `TENTATIVE` |
| `confirmed` | `CONFIRMED` |
| `cancelled` (any reason) | `CANCELLED` |

---

## 8. PROPFIND Privilege Injection

### CDV-PRIV-1 — Collection Depth-0
`PROPFIND Depth:0` on a calendar collection (`/caldav/user/calendars/{id}/`)
must include `current-user-privilege-set` with at least `read`, `write`,
`write-properties`, `write-content`, `read-acl`, `read-current-user-privilege-set`,
and `read-free-busy`.

### CDV-PRIV-2 — Collection Depth-1
`PROPFIND Depth:1` on a calendar collection must return one `<d:response>`
entry for the collection itself and one entry per event in that collection.
Each event entry must include `current-user-privilege-set` with write privileges.
Without per-event write privileges iOS Calendar renders all events as read-only.

### CDV-PRIV-3 — Individual Event Resource
`PROPFIND` on a single `.ics` path (`/caldav/user/calendars/{id}/{uid}.ics`)
must return the event's `getetag`, `getcontenttype`, and
`current-user-privilege-set` (read + write privileges).

### CDV-PRIV-4 — PROPPATCH No-Op
`PROPPATCH` requests on calendar collections (used by iOS to set display name
and colour metadata) must respond `207 Multi-Status` with `200 OK` for all
proposed property changes without persisting them. Returning `501 Not Implemented`
causes some clients to hide write-level UI elements.

---

## 9. Scheduling Support (Inbox / Outbox)

### CDV-SCHED-1 — Inbox PROPFIND
`PROPFIND /caldav/user/inbox/` must respond `207 Multi-Status` with resource
type `<collection/><schedule-inbox/>` and a `calendar-free-busy-set` listing
the hrefs of all calendars owned by the authenticated user.

### CDV-SCHED-2 — Outbox PROPFIND
`PROPFIND /caldav/user/outbox/` must respond `207 Multi-Status` with resource
type `<collection/><schedule-outbox/>` and a `current-user-privilege-set`
that includes `schedule-send`, `schedule-send-invite`, `schedule-send-reply`,
and `schedule-send-freebusy`.

### CDV-SCHED-3 — Outbox OPTIONS
`OPTIONS /caldav/user/outbox/` must include `POST` in the `Allow` response
header. `OPTIONS /caldav/user/inbox/` must not include `POST`.

### CDV-SCHED-4 — Free/Busy Query via Outbox POST
`POST /caldav/user/outbox/` with a `text/calendar` body containing a
`METHOD:REQUEST` VCALENDAR with a `VFREEBUSY` component must:

1. Parse the requested time range from the `DTSTART`/`DTEND` properties of
   the `VFREEBUSY` component.
2. Query `DomainStore.ListActiveBookingsInWindow` for the requested window.
3. Synthesise a response body containing one `VFREEBUSY` component per
   attendee requested, with `FREEBUSY;FBTYPE=BUSY` periods derived from the
   booked appointments.
4. Each `FREEBUSY` busy period must be a separate property line (one period
   per line, not a comma-separated list on one line).
5. Respond `HTTP 200 OK` with `Content-Type: text/calendar` and a
   `Schedule-Response` header.
6. iCal line endings embedded in XML responses must use `\n` (not `\r\n`);
   `\r` characters must be stripped before embedding.

---

## 10. Free/Busy Query via REPORT

### CDV-FB-1 — Calendar-Query REPORT
`REPORT` with a `<free-busy-query>` request body on a calendar collection must
return the free/busy periods as a `text/calendar` body derived from
`DomainStore.ListActiveBookingsInWindow` for the requested time range.

---

## 11. TRANSP / "Show As Free/Busy"

### CDV-TRANSP-1 — Mapping
The iCal `TRANSP` property maps to the domain opacity field:

| iCal `TRANSP` | Domain constant |
| --- | --- |
| `OPAQUE` (default, busy) | `OpacityOpaque` |
| `TRANSPARENT` (free) | `OpacityTransparent` |

### CDV-TRANSP-2 — Read (iCal → Domain)
When parsing a `PUT` payload, the adapter must read `TRANSP`; the non-standard
`OPACITY` property must also be accepted as a fallback for backward
compatibility.

### CDV-TRANSP-3 — Write (Domain → iCal)
When synthesising a VEVENT, the adapter must always write a `TRANSP` property;
`OPAQUE` when busy, `TRANSPARENT` when free. The property must never be omitted.

### CDV-TRANSP-4 — "Show As" Toggle Preconditions
All four of the following conditions must be true simultaneously for iOS
Calendar to display the per-event "Show As" (TRANSP) toggle:

1. `DAV:` header contains `calendar-auto-schedule`.
2. Principal response includes `schedule-default-calendar-URL`.
3. Principal response includes `calendar-user-type: INDIVIDUAL`.
4. Individual event resources carry `current-user-privilege-set` with write
   privileges.

---

## 12. Calendar Properties

### CDV-CPROP-1 — Supported Component Set
Every calendar collection response must include
`<supported-calendar-component-set>` listing at minimum `VEVENT`.

### CDV-CPROP-2 — Free/Busy Report Support
Every calendar collection response must include `<free-busy-query>` in
`<supported-report-set>`.

### CDV-CPROP-3 — CTag
Every calendar collection response must include the Apple CalendarServer
`cs:getctag` property. Its value must change whenever the contents of the
calendar change, allowing clients to detect updates without fetching all events.

### CDV-CPROP-4 — Schedule Transparency
Every calendar collection response must include
`<cal:schedule-calendar-transp><cal:opaque/></cal:schedule-calendar-transp>`
to indicate the calendar participates in free/busy queries.

### CDV-CPROP-5 — Calendar Colour and Order
Every calendar collection response should include the Apple-extension properties
`x1:calendar-color` and `x1:calendar-order` so clients can render calendars
with distinct visual identifiers.

---

## 13. iCal Format Requirements

### CDV-ICAL-1 — Required Fields
Every synthesised VEVENT must include `DTSTAMP` (RFC 5545 §3.8.7.2 — exactly
one occurrence required). The value must be stable across identical requests;
use the event's last-modification time and fall back to current UTC only when
unset.

### CDV-ICAL-2 — Product Identifier
The `PRODID` of every generated VCALENDAR must be `-//schedio//schedio//EN`.

### CDV-ICAL-3 — Calendar Scale
Every generated VCALENDAR must include `CALSCALE:GREGORIAN`.

### CDV-ICAL-4 — All-Day Events
Timeslot events with `VALUE=DATE` on `DTSTART` must be interpreted and stored
as all-day events. When serialised back, an all-day event must use `VALUE=DATE`
on both `DTSTART` and `DTEND`.

### CDV-ICAL-5 — Time Zones
All date-time values stored in the domain store use UTC. When serialising for
CalDAV responses, date-time values must be expressed in UTC (`Z` suffix).

---

## 14. Error Handling

### CDV-ERR-1 — Not Found
A request for a calendar or event that does not exist must return `404 Not Found`.

### CDV-ERR-2 — Precondition Failed
An `If-Match` or `If-None-Match` precondition that is not satisfied must return
`412 Precondition Failed`.

### CDV-ERR-3 — Forbidden
An attempt to modify a read-only resource (e.g. write to the bookings calendar,
or create a new calendar) must return `403 Forbidden`.

### CDV-ERR-4 — Method Not Allowed
An HTTP method that is not applicable to the target resource must return
`405 Method Not Allowed` with an appropriate `Allow` header, **except** for
`MKCALENDAR` which must return `403 Forbidden` (see CDV-CAL-2).

### CDV-ERR-5 — Error Propagation
All store errors must be mapped to appropriate HTTP status codes:

| Store error | HTTP status |
| --- | --- |
| `ErrNotFound` | `404 Not Found` |
| `ErrConflict` | `412 Precondition Failed` |
| `ErrReadOnly` | `403 Forbidden` |
| Unexpected / internal | `500 Internal Server Error` |

---

## 15. Double-Booking Prevention

### CDV-DBL-1 — Atomic Check-and-Write
The availability check and the booking creation/update must be executed inside
a single atomic critical section:

- **PostgreSQL backend**: a `SELECT … FOR UPDATE` on the affected timeslot row
  combined with a time-overlap query inside a serializable transaction. A
  conflicting booking causes a transaction rollback and returns `ErrConflict`.
- **Memory backend**: the write mutex must be held for the full duration of the
  check-plus-write.

No concurrent goroutine may insert a conflicting booking between the check and
the write.

---

## 16. Non-Functional Requirements

### CDV-NFR-1 — No Standalone Persistence
The CalDAV endpoint must not introduce its own separate persistence layer. All
state is read from and written to the domain store (`DomainStore` /
`CalendarStore`). In the `postgres` backend mode no CalDAV-specific tables
exist.

### CDV-NFR-2 — Stateless Handler
The CalDAV handler must be stateless between requests; all per-request state is
derived from the authenticated identity and the domain store.

### CDV-NFR-3 — Standard Library HTTP
The CalDAV endpoint must use only the standard-library `net/http` package for
HTTP serving; no third-party HTTP router or framework.

### CDV-NFR-4 — Structured Logging
All log output from the CalDAV layer must use `k8s.io/klog/v2`. Normal
operation is logged at `klog.Info`; per-request detail at `klog.V(2).Info`;
decoded event data at `klog.V(3).Info`.

### CDV-NFR-5 — Testability
The CalDAV handler must be testable without a running network server and without
real authentication. Tests must inject the authenticated principal via the
request context. The store must be replaceable with the in-memory implementation
for all CalDAV tests.

### CDV-NFR-6 — iOS / iPadOS Interoperability
The implementation must be verified against the iOS / iPadOS CalDAV client
(tested against iPadOS 18). All six discovery steps (CDV-DISC-1 through
CDV-DISC-5 plus the scheduling inbox/outbox) must succeed end-to-end before an
account is considered functional. The "Show As Free/Busy" editing toggle must
be visible in the iOS event editor.

---

## 17. Supported CalDAV Methods Summary

| Method | Target | Required behaviour |
| --- | --- | --- |
| `OPTIONS` | Any | Return `DAV:` capability header + `Allow` header; no auth required |
| `PROPFIND` | `/caldav/` | Root multistatus with `current-user-principal` (CDV-DISC-5) |
| `PROPFIND` | `/caldav/user/` | Full principal properties (CDV-DISC-3) |
| `PROPFIND` | `/caldav/user/calendars/` | Calendar home multistatus (Depth:0 and Depth:1) |
| `PROPFIND` | `/caldav/user/calendars/{id}/` | Collection properties (CDV-PRIV-1, CDV-PRIV-2) |
| `PROPFIND` | `/caldav/user/calendars/{id}/{uid}.ics` | Per-event properties (CDV-PRIV-3) |
| `PROPFIND` | `/caldav/user/inbox/` | Inbox properties (CDV-SCHED-1) |
| `PROPFIND` | `/caldav/user/outbox/` | Outbox properties (CDV-SCHED-2) |
| `PROPPATCH` | `/caldav/user/calendars/{id}/` | No-op `207` (CDV-PRIV-4) |
| `REPORT` | `/caldav/user/calendars/{id}/` | calendar-query + free-busy-query (CDV-BK-2, CDV-FB-1) |
| `GET` | `/caldav/user/calendars/{id}/{uid}.ics` | Return iCal body of event |
| `PUT` | `/caldav/user/calendars/timeslots/{uid}.ics` | Upsert timeslot (CDV-TS-1) |
| `PUT` | `/caldav/user/calendars/bookings/{uid}.ics` | `405 Method Not Allowed` (CDV-CAL-4) |
| `DELETE` | `/caldav/user/calendars/timeslots/{uid}.ics` | Delete timeslot (CDV-TS-2) |
| `DELETE` | `/caldav/user/calendars/bookings/{uid}.ics` | `405 Method Not Allowed` (CDV-CAL-4) |
| `MKCALENDAR` | Any | `403 Forbidden` (CDV-CAL-2) |
| `POST` | `/caldav/user/outbox/` | Free/busy scheduling request (CDV-SCHED-4) |
