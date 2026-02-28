# GitHub Copilot Instructions

## Project overview

**schedio** is a Go web-service scaffold that bundles:

- embedded static web assets (HTML / CSS / JS) compiled into the binary
- an OpenAPI endpoint with Swagger UI
- a CalDAV endpoint backed by a pluggable `CalendarStore` interface
- Docker container support

Module name: `schedio`  
Minimum Go version: see `go.mod`

## Repository layout

```
cmd/schedio/          # application entry-point (main.go)
internal/
  doc/                # requirements, design docs, protocol notes, etc.
  caldav/             # CalDAV protocol implementation (go-webdav adapter)
  config/             # CLI argument parsing and runtime config
  handlers/           # net/http handlers (OpenAPI, Swagger, CalDAV, health, web)
  middleware/         # HTTP middleware (logging, …)
  server/             # router / server composition
  store/              # CalendarStore interface + in-memory / dummy implementations
api/                  # OpenAPI YAML source (embedded via go:embed)
web/                  # frontend assets (embedded via go:embed)
```

## Language and style

- **Go only** – no code generation frameworks, no ORMs.
- Follow standard Go conventions (`gofmt`, `golangci-lint`).
- Package names are short, lowercase, single words (`caldav`, `store`, `middleware`).
- Exported identifiers use full, clear names; avoid abbreviations except well-known ones (`ctx`, `w`, `r`, `err`).
- Keep functions small and single-purpose; prefer composition through interfaces over inheritance.
- All exported types, functions, and package-level variables must have a Go doc comment.
- Error strings are lowercase and do not end with punctuation (Go convention).

## Error handling

- Always check and handle errors explicitly; never discard them with `_` unless the reason is stated in a comment.
- Wrap errors with context using `fmt.Errorf("…: %w", err)`.
- Domain sentinel errors live in `internal/store/store.go` (`ErrNotFound`, `ErrConflict`, `ErrReadOnly`); add new sentinels there for new domain errors.

## Logging

- Use **`k8s.io/klog/v2`** for all structured logging; do not use `log`, `fmt.Print*`, or any other logger.
- Verbosity levels: `klog.Info` for normal operation, `klog.V(2).Info` for debug detail, `klog.Error` / `klog.Fatal` for errors.
- Initialise flags with `klog.InitFlags(nil)` in `main`; call `defer klog.Flush()`.

## HTTP and routing

- Use the standard library `net/http` only – no third-party router.
- Handlers must honour `context.Context` from `r.Context()` and propagate it to store calls.
- Set sensible server timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`).
- Health check lives at `/healthz`; it returns `200 OK` with no body.

## CalDAV

- The CalDAV layer is implemented via `github.com/emersion/go-webdav/caldav`.
- Business logic is accessed only through the `CalendarStore` interface (`internal/store/store.go`); the handler never touches persistence directly.
- New store implementations must satisfy all methods of `CalendarStore`; add tests under `internal/caldav/` using the existing test helpers.
- The `davEnforcingWriter` wrapper in `internal/caldav/handler.go` ensures correct `DAV:` response headers; do not bypass it.

## Store interface

```go
// Add new methods to CalendarStore only when genuinely needed by the protocol.
// Keep the interface minimal so all implementations stay in sync.
type CalendarStore interface { … }
```

Implementations: `MemoryStore` (default), `DummyStore` (read-only seed data for demos).  
New implementations go in `internal/store/` following the same file-per-implementation pattern.

## Testing

- Use the standard `testing` package; no third-party assertion libraries.
- Table-driven tests are preferred.
- Test files live next to the code they test (`*_test.go` in the same package).
- External (black-box) tests use the `_test` package suffix where appropriate.
- Tests must not rely on network access, real file systems, or timing; use in-memory stores and stub data.

## Configuration

- Runtime configuration is parsed in `internal/config/` via `flag` and environment variables.
- Do not add global variables for config; pass `config.Args` explicitly through constructors.

## Embedded assets

- Static web assets are embedded with `//go:embed` in `web/web.go`.
- The OpenAPI YAML is embedded in `api/openapi.go`.
- Do not read files from disk at runtime; always serve from the embedded FS.

## Dependencies

Before adding a new dependency:
1. Check whether the standard library covers the need.
2. Prefer packages already present in `go.mod`.
3. Avoid dependencies that pull in large transitive trees.

---

## CalDAV protocol notes (iOS Calendar / iPadOS interoperability)

This section records hard-won knowledge about the iOS/iPadOS CalDAV client
(tested against iPadOS 18). Use it when modifying `internal/caldav/`.

### Discovery flow

iOS performs a fixed sequence of requests on first account setup and after a
protocol-level reset. All requests must succeed for the account to activate.

```
1. PROPFIND /.well-known/caldav          → 301 → /caldav/
2. PROPFIND /                            → 207  current-user-principal href
3. PROPFIND /caldav/user/               → 207  principal props (see below)
4. PROPFIND /caldav/user/calendars/     → 207  Depth:1  calendar list
5. PROPFIND /caldav/user/inbox/         → 207  schedule-inbox props
6. PROPFIND /caldav/user/outbox/        → 207  schedule-outbox props + privileges
```

Steps 1–3 are service discovery (RFC 6764 / RFC 4791 §6). Steps 4–6 are
scheduling discovery (RFC 6638).

### DAV: capability header

The `DAV:` response header (set by `davCapabilities` in
`internal/caldav/handler.go` and `internal/caldav/discovery.go`) **must**
include the following tokens for iOS to enable its full calendar UI:

| Token | Why it matters |
|---|---|
| `calendar-access` | RFC 4791 — basic CalDAV |
| `calendar-auto-schedule` | RFC 6638 — server-side scheduling. **Must be `auto-schedule`, not plain `calendar-schedule`.** iOS checks for this exact token to enable the per-event "Show As Free/Busy" (TRANSP) editing field. |

Do not change `calendar-auto-schedule` back to `calendar-schedule`; the "Show
As" / "Anzeigen als" toggle disappears entirely if the wrong token is used.

The `DAV:` header is enforced by `davEnforcingWriter` so the inner go-webdav
handler cannot override it. Both code paths (`handler.go` and `discovery.go`)
use the shared `davCapabilities` constant.

### Principal PROPFIND (`/caldav/user/`)

iOS parses the following properties from the principal resource. **All are
required** for the full event-editing UI to work:

| Property | Namespace | Required value / notes |
|---|---|---|
| `calendar-home-set` | caldav | Href to `/caldav/user/calendars/` |
| `schedule-inbox-URL` | caldav | Href to inbox |
| `schedule-outbox-URL` | caldav | Href to outbox |
| `schedule-default-calendar-URL` | caldav | Href to the first/default calendar. **Required by RFC 6638 §2.4.2** when scheduling is advertised. Omit (not empty) when no calendars exist. |
| `calendar-user-address-set` | caldav | At least one mailto: href |
| `calendar-user-type` | caldav | `INDIVIDUAL` (not a room / resource). Absence can suppress the editing UI. |
| `current-user-privilege-set` | DAV | Include `read`, `write`, `write-properties`, `write-content`, `read-acl`, `read-current-user-privilege-set`, `read-free-busy` |
| `current-user-principal` | DAV | Self-referential href |
| `resourcetype` | DAV | `<collection/><principal/>` |
| `displayname` | DAV | Human readable name |

### Calendar collection PROPFIND

iOS issues `PROPFIND Depth:1` on the calendar home to enumerate collections and
a `PROPFIND Depth:0` on each collection to read its attributes. Both must
include `current-user-privilege-set` (RFC 3744 §5.4) with write privileges so
iOS enables the event-create / event-edit buttons.

Key properties per calendar collection:
- `schedule-calendar-transp` → `<opaque/>` (participates in free/busy)
- `supported-calendar-component-set` → at minimum `VEVENT`
- `free-busy-query` in `supported-report-set`
- Apple extension `cs:getctag` (use the store's `CTag` value)
- Apple extension `x1:calendar-color` and `x1:calendar-order`

### Per-event PROPFIND and Depth:1 on a collection

The inner go-webdav handler does **not** emit `current-user-privilege-set` on
`.ics` resources. The outer handler in `internal/caldav/handler.go` intercepts:

- `PROPFIND Depth:1` on a collection → `calendarCollectionDepth1Multistatus`
  returns one `<response>` for the collection plus one per event, each with
  `current-user-privilege-set` (read + write privileges).
- `PROPFIND` on a single `.ics` path → `calendarObjectMultistatus` returns the
  event with `getetag`, `getcontenttype`, and `current-user-privilege-set`.

Without write privileges on individual event resources iOS shows events as
read-only regardless of collection-level privileges.

### TRANSP / "Show As Free/Busy"

`TRANSP` in a VEVENT maps to `Event.Opacity` in the store:

| iCal `TRANSP` | `store.Opacity` constant |
|---|---|
| `OPAQUE` (default, busy) | `OpacityOpaque` |
| `TRANSPARENT` (free) | `OpacityTransparent` |

The adapter in `internal/caldav/adapter.go` handles both directions:
- `icalToEvent`: reads `PropTransparency`; falls back to the non-standard
  `OPACITY` property for backward compat.
- `eventToObject`: always writes `TRANSP:OPAQUE` or `TRANSP:TRANSPARENT`.

The "Anzeigen als" (Show As) toggle in iOS Calendar requires **all** of the
following to be true simultaneously:
1. `DAV:` header contains `calendar-auto-schedule`
2. Principal exposes `schedule-default-calendar-URL`
3. Principal exposes `calendar-user-type: INDIVIDUAL`
4. Individual event resources carry `current-user-privilege-set` with write
   privileges

### Free/busy (VFREEBUSY) via outbox POST

- iOS sends a `POST` to `/caldav/user/outbox/` with a `METHOD:REQUEST` VCALENDAR
  containing a VFREEBUSY component.
- The response must be `HTTP 200` with a `Schedule-Response` header and a
  `Content-Type: text/calendar` body containing one VFREEBUSY per attendee with
  `FBTTYPE:BUSY` lines.
- Each FREEBUSY period **must** be on its own `FREEBUSY` property line (RFC 5545
  §3.8.2.6 — multiple values per property are also valid, but one-per-line is
  safer across clients).
- The iCal body embedded in XML responses must use `\n` line endings, not
  `\r\n`; the handler strips `\r` before embedding.

### Outbox OPTIONS / PROPFIND

- `OPTIONS /caldav/user/outbox/` must include `POST` in the `Allow` header.
- `PROPFIND /caldav/user/outbox/` must return `current-user-privilege-set`
  including `schedule-send`, `schedule-send-invite`, `schedule-send-reply`,
  and `schedule-send-freebusy`.

### Reference servers for comparison

When debugging iOS interoperability issues, compare against a known-good server:

- **Baikal (SabreDAV 4.7.0)** at `https://buchung.boeger-martin.de/dav.php`
  (Digest auth — use `curl --anyauth -u "user:password"`).
  Principal path: `/dav.php/principals/calendar/`.

Useful curl one-liner:
```sh
curl --anyauth -u "user:password" -X PROPFIND \
  "https://buchung.boeger-martin.de/dav.php/principals/calendar/" \
  -H "Depth: 0" -H "Content-Type: text/xml" \
  --data '<?xml version="1.0"?><A:propfind xmlns:A="DAV:"><A:allprop/></A:propfind>'
```

---

## Frontend / Web UI

### Design documents

- `doc/userinterface.md` — primary UI specification (1 288 lines, 12 sections,
  29 components). Created and cross-checked against `doc/requirements.md` for
  consistency. **Read this document before touching any file in `web/`.**
- `doc/requirements.md` — functional requirements (M1–M25 resolved).
- `doc/architecture.md` — system architecture (17 sections, gap analysis in
  §17.19 gives a 16-step sequencing for backend implementation).

### Web component conventions

- **Pure JavaScript, no framework** — Custom Elements v1 (`HTMLElement`
  subclass, `customElements.define`). No npm dependencies in component source
  files.
- **Shadow DOM** `mode: 'open'` on every component.
- **Styling** — CSS custom properties only; design tokens live in
  `web/css/tokens.css` and are imported via `@import` inside every shadow root.
- **Module system** — ES modules (`type="module"`). Bundling is a build step,
  not a source-code concern.
- **HTTP** — `fetch` + `async`/`await`; no global state for in-flight requests.
- **Routing** — `history.pushState` / `popstate` managed by top-level
  `<x-booking-app>` and `<x-admin-app>`.
- **Accessibility** — WCAG 2.1 Level AA mandatory (see §10 of userinterface.md).
- **Language** — German UI labels throughout.

### Component inventory (29 components)

**Customer booking SPA (`web/js/booking/`):**

| Component | File | Responsibility |
| --- | --- | --- |
| `<x-booking-app>` | `booking-app.js` | Top-level orchestrator, step state machine |
| `<x-stepper>` | `stepper.js` | Horizontal progress indicator |
| `<x-service-picker>` | `x-service-picker.js` (existing) | Service list + detail, fetches `/api/v1/services` |
| `<x-date-time-picker>` | `x-date-time-picker.js` (existing) | Calendar + time-slot picker |
| `<x-booking-line>` | `booking-line.js` | Single booking line (one picker + remove button) |
| `<x-booking-list>` | `booking-list.js` | Ordered list of booking lines + "add" button |
| `<x-contact-form>` | `contact-form.js` | Name / email / phone with validation |
| `<x-session-summary>` | `session-summary.js` | Read-only session summary (steps 4 and 5) |
| `<x-tandc-accept>` | `tandc-accept.js` | T&C PDF link + mandatory acceptance checkbox |
| `<x-booking-success>` | `booking-success.js` | Post-submission step 5 view |

**Customer booking management view (`web/js/manage/`):**

| Component | File | Responsibility |
| --- | --- | --- |
| `<x-booking-manager>` | `booking-manager.js` | Orchestrator for token-protected management page |
| `<x-booking-card>` | `booking-card.js` | Single booking row with reschedule / cancel actions |
| `<x-reschedule-picker>` | `reschedule-picker.js` | Inline date-time picker for rescheduling |
| `<x-cancel-confirm>` | `cancel-confirm.js` | Cancellation confirmation dialog |

**Shared utilities (`web/js/shared/`):**

| Component | File | Responsibility |
| --- | --- | --- |
| `<x-spinner>` | `spinner.js` | Loading indicator |
| `<x-toast>` | `toast.js` | Transient notification |
| `<x-dialog>` | `dialog.js` | Modal dialog wrapper |
| `<x-error-banner>` | `error-banner.js` | Persistent error display |

**Admin SPA (`web/js/admin/`):**

| Component | File | Responsibility |
| --- | --- | --- |
| `<x-admin-app>` | `admin-app.js` | Top-level admin orchestrator + routing |
| `<x-admin-nav>` | `admin-nav.js` | Side-navigation |
| `<x-login-form>` | `login-form.js` | Admin login form |
| `<x-admin-dashboard>` | `admin-dashboard.js` | Dashboard shell |
| `<x-dashboard-today>` | `dashboard-today.js` | Today's reserved + confirmed bookings |
| `<x-dashboard-pending>` | `dashboard-pending.js` | Pending (unconfirmed) sessions |
| `<x-service-list>` | `service-list.js` | Service catalogue list |
| `<x-service-form>` | `service-form.js` | Create / edit service |
| `<x-session-review>` | `session-review.js` | Confirm / reject a booking session |
| `<x-settings-form>` | `settings-form.js` | Global settings editor |
| `<x-tandc-upload>` | `tandc-upload.js` | T&C PDF upload |
| `<x-secret-manager>` | `secret-manager.js` | HMAC signing-secret rotation |

### Key design decisions (do not change without updating userinterface.md)

- **Management link URL format** — `/?id=<bookingID>&token=<signedToken>`.
  Two separate query parameters; `token` is an HMAC-SHA256 of the booking ID,
  and `id` is provided separately for convenient frontend URL construction.
  The requirements phrase "signed token" refers to the combined intent, not a
  single URL parameter.
- **Remove button on first booking line** — hidden only when it is the **sole**
  remaining line, not unconditionally hidden.
- **First booking line pre-selection** — on step 2 initial render, the first
  `<x-booking-line>` has its `value` preset to the earliest available slot.
- **`<x-booking-success>` (step 5)** — receives all data via JS properties from
  `<x-booking-app>`; makes no API calls.
- **Admin dashboard today panel** — shows only bookings with state `reserved` or
  `confirmed`; cancelled and no-show bookings are excluded.
- **Pending sessions table** — columns include earliest requested booking date
  and time, so the admin can prioritise review.

### Suggested implementation order

Start with components that have no dependencies on other custom elements:

1. Shared utilities: `<x-spinner>`, `<x-toast>`, `<x-dialog>`, `<x-error-banner>`
2. Leaf booking components: `<x-stepper>`, `<x-booking-line>`, `<x-contact-form>`,
   `<x-tandc-accept>`, `<x-booking-success>`, `<x-session-summary>`
3. Composite booking components: `<x-booking-list>`, then `<x-booking-app>`
4. Management view components
5. Admin SPA components (login → dashboard → service management → settings)
