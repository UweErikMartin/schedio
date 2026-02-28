# schedio — Architecture

## 1. Architectural Principles

| Principle | Rationale |
| --- | --- |
| **Single binary, single container** | Simplified deployment on Kubernetes; no sidecar processes. |
| **Standard-library HTTP only** | No third-party router. Keeps the dependency tree small and predictable. |
| **PostgreSQL as single source of truth** | Domain state lives in PostgreSQL; all other views (CalDAV, REST, UI) are derived from it. |
| **CalDAV as a read/write facade** | The CalDAV protocol surface is translated into domain operations on the store layer. No CalDAV-native persistence is introduced when the `postgres` backend is active. |
| **Interface-driven store layer** | Two store interfaces (`CalendarStore`, `DomainStore`) allow the `memory` backend to be substituted for local development without touching business logic. |
| **Explicit error propagation** | No panics; errors are wrapped and propagated up to the HTTP handler boundary. |
| **Structured logging via klog** | All log output goes through `k8s.io/klog/v2`; no `fmt.Print*` or `log` package. |

---

## 2. System Context

```text
┌──────────────────────────────────────────────────────────────┐
│                        Kubernetes cluster                    │
│                                                              │
│   ┌──────────────────────────────────────────────────────┐   │
│   │                   schedio  (Pod)                     │   │
│   │                                                      │   │
│   │  ┌────────────────────────────────────────────────┐  │   │
│   │  │              Go binary  (:8080)                │  │   │
│   │  │  ┌──────────┐ ┌──────────┐ ┌───────────────┐  │  │   │
│   │  │  │ REST API │ │ Admin UI │ │  CalDAV (/    │  │  │   │
│   │  │  │ /api/v1/ │ │ /admin/  │ │  caldav/)     │  │  │   │
│   │  │  └──────────┘ └──────────┘ └───────────────┘  │  │   │
│   │  │           Domain / Service Layer               │  │   │
│   │  │    ┌───────────────────────────────────────┐   │  │   │
│   │  │    │ DomainStore interface                 │   │  │   │
│   │  │    │  PostgresStore  │  MemoryStore        │   │  │   │
│   │  │    └───────────────────────────────────────┘   │  │   │
│   │  └────────────────────────────────────────────────┘  │   │
│   │                                                      │   │
│   │   PVC: DATA_DIR (T&C PDFs)                           │   │
│   └──────────────────────────────────────────────────────┘   │
│                                                             │
│   ┌──────────┐                                              │
│   │PostgreSQL│   (separate Pod / managed RDS)                │
│   └──────────┘                                              │
└──────────────────────────────────────────────────────────────┘

External actors
  Customer browser  ──→  REST API + static assets
  Admin browser     ──→  Admin REST API + static assets
  Apple Calendar    ──→  CalDAV endpoint
  Apple OAuth       ──→  /auth/apple callback
  SMTP relay        ←──  email subsystem
```

---

## 3. Component Architecture

```text
cmd/schedio/main.go
  │
  ├── config.ParseCommandLineArgs()   // flags + env vars
  ├── store.NewBackend()              // selects postgres or memory
  │     ├── postgres.Store            // implements CalendarStore + DomainStore
  │     └── memory.Store             // implements CalendarStore + DomainStore
  ├── token.NewSigner()               // loads or generates HMAC secret
  ├── email.NewSender()               // SMTP client
  ├── retention.StartJob()            // background goroutine
  └── server.NewRouter()
        ├── middleware.LoggingMiddleware
        ├── handlers (customer REST)
        │     ├── ServiceHandler
        │     ├── AvailabilityHandler
        │     ├── SessionHandler
        │     └── BookingHandler       // management-link endpoints
        ├── handlers (admin REST)
        │     ├── AuthHandler          // login / Apple OAuth / logout
        │     ├── AdminDashboardHandler
        │     ├── AdminServiceHandler
        │     ├── AdminSessionHandler  // session review / confirm / reject
        │     ├── AdminBookingHandler  // no-show
        │     └── AdminSettingsHandler // T&C PDF, settings, HMAC secret
        ├── handlers.HandleWebUserInterface  // embedded SPA
        ├── handlers.NewOpenAPIHandler       // Swagger UI at /api/
        ├── handlers.HandleHealthz           // /healthz
        ├── caldav.NewHandler()              // CalDAV facade
        └── caldav.NewDiscoveryHandler()     // well-known + principals
```

---

## 4. Package Structure

### Existing packages (to be extended)

| Package | Current role | Extension |
| --- | --- | --- |
| `cmd/schedio` | Entry point | Add backend selection, token signer, email sender, retention job |
| `internal/config` | CLI flags | Add env-var bindings for all new variables |
| `internal/store` | `CalendarStore` interface + memory/dummy | Add `DomainStore` interface; add `postgres` backend |
| `internal/caldav` | CalDAV protocol handler | Extend to translate writes into `DomainStore` calls |
| `internal/handlers` | Health, OpenAPI, web | Add customer REST handlers; add admin REST handlers |
| `internal/middleware` | Logging | Add auth middleware (cookie check for admin routes) |
| `internal/server` | Router | Add all new routes |

### New packages

| Package | Responsibility |
| --- | --- |
| `internal/domain` | Pure business logic: availability calculation, double-booking prevention, booking state machine, conflict detection |
| `internal/auth` | Apple Sign-In (OAuth 2.0 / OIDC), bcrypt username/password, signed HTTP-only cookie session management |
| `internal/email` | SMTP client, Go `text/template` templates, domain-level send functions (reserved, result, change, cancel, admin-notify, admin-conflict) |
| `internal/token` | HMAC-SHA256 token sign / verify for management links |
| `internal/retention` | Background goroutine that periodically deletes expired contact and booking data |
| `internal/store/postgres` | PostgreSQL implementation of `CalendarStore` + `DomainStore`; embedded SQL migrations via `//go:embed` |

---

## 5. Store Interfaces

### 5.1 Existing — `CalendarStore` (`internal/store/store.go`)

Covers CalDAV-level primitives used by the CalDAV façade:

```go
type CalendarStore interface {
    GetCalendar(ctx, id)              (*Calendar, error)
    ListCalendars(ctx)                ([]*Calendar, error)
    GetEvent(ctx, calendarID, eventID) (*Event, error)
    ListEvents(ctx, calendarID, start, end) ([]*Event, error)
    PutEvent(ctx, event)              error
    DeleteEvent(ctx, calendarID, eventID, etag) error
    CTag(ctx, calendarID)             (string, error)
}
```

### 5.2 New — `DomainStore` (`internal/store/store.go`)

Covers all domain-level persistence required by business logic and REST handlers. The PostgreSQL backend implements both interfaces; the memory backend also implements both (domain operations are backed by in-process maps protected by a `sync.RWMutex`).

```go
type DomainStore interface {
    // --- Staff ---
    ListStaff(ctx) ([]*Staff, error)
    GetStaff(ctx, id) (*Staff, error)

    // --- Services ---
    ListServices(ctx) ([]*Service, error)
    GetService(ctx, id) (*Service, error)
    CreateService(ctx, s *Service) error
    UpdateService(ctx, s *Service) error
    DeleteService(ctx, id) error        // returns ErrConflict if active bookings exist

    // --- Timeslots (availability windows) ---
    ListTimeslots(ctx, staffID string, start, end time.Time) ([]*Timeslot, error)
    GetTimeslot(ctx, staffID, uid string) (*Timeslot, error)
    UpsertTimeslot(ctx, t *Timeslot) error
    DeleteTimeslot(ctx, staffID, uid string) error

    // --- Contacts ---
    GetOrCreateContact(ctx, email string, c *Contact) (*Contact, error)
    GetContact(ctx, id) (*Contact, error)
    UpdateContactLastBooking(ctx, contactID string, at time.Time) error

    // --- Booking sessions ---
    CreateSession(ctx, s *BookingSession) error
    GetSession(ctx, id) (*BookingSession, error)
    UpdateSession(ctx, s *BookingSession) error
    ListPendingSessions(ctx) ([]*BookingSession, error) // reserved, oldest first

    // --- Bookings ---
    // CreateBooking is atomic: it checks availability and inserts the booking
    // in a single critical section (DB transaction + row-level lock for postgres;
    // mutex for memory).
    CreateBooking(ctx, b *Booking) error            // returns ErrConflict on overlap
    GetBooking(ctx, id) (*Booking, error)
    UpdateBooking(ctx, b *Booking) error
    ListBookingsForSession(ctx, sessionID) ([]*Booking, error)
    ListBookingsForDay(ctx, date time.Time) ([]*Booking, error)
    ListActiveBookingsInWindow(ctx, staffID string, start, end time.Time) ([]*Booking, error)

    // --- Settings ---
    GetSettings(ctx) (*Settings, error)
    UpdateSettings(ctx, s *Settings) error
    GetHMACSecret(ctx) ([]byte, error)
    SetHMACSecret(ctx, secret []byte) error

    // --- Retention ---
    DeleteExpiredContacts(ctx, olderThan time.Time) (int, error)
}
```

---

## 6. Domain Model

### 6.1 Entities

```text
Staff
  id          UUID  PK
  name        TEXT
  identifier  TEXT  UNIQUE  -- matches config.yaml entry

Service
  id                UUID  PK
  name              TEXT  NOT NULL
  description       TEXT
  price             NUMERIC(10,2)  NOT NULL
  duration_minutes  INTEGER  NOT NULL
  daily_limit       INTEGER  NOT NULL DEFAULT 0  -- 0 = unlimited
  created_at        TIMESTAMPTZ
  updated_at        TIMESTAMPTZ

Timeslot
  id              UUID  PK
  staff_id        UUID  FK → Staff
  caldav_uid      TEXT  UNIQUE  -- iCal UID
  caldav_etag     TEXT
  start_at        TIMESTAMPTZ  NOT NULL
  end_at          TIMESTAMPTZ  NOT NULL
  rrule           TEXT         -- iCal RRULE value; NULL for single events
  recurrence_id   TIMESTAMPTZ  -- set for individual overrides of a recurring series
  created_at      TIMESTAMPTZ
  updated_at      TIMESTAMPTZ

Contact
  id               UUID  PK
  name             TEXT  NOT NULL
  email            TEXT  NOT NULL  UNIQUE
  phone            TEXT  NOT NULL
  created_at       TIMESTAMPTZ
  last_booking_at  TIMESTAMPTZ  -- updated on every new booking; drives retention

BookingSession
  id          UUID  PK
  service_id  UUID  FK → Service
  contact_id  UUID  FK → Contact
  state       TEXT  -- 'open' | 'submitted' | 'closed'
  created_at  TIMESTAMPTZ
  submitted_at TIMESTAMPTZ

Booking
  id              UUID  PK
  session_id      UUID  FK → BookingSession
  service_id      UUID  FK → Service
  contact_id      UUID  FK → Contact
  staff_id        UUID  FK → Staff
  start_at        TIMESTAMPTZ  NOT NULL
  end_at          TIMESTAMPTZ  NOT NULL
  state           TEXT  -- see §6.2
  cancel_reason   TEXT  -- 'customer' | 'admin' | 'noshow' | NULL
  sequence        INTEGER  NOT NULL DEFAULT 0
  caldav_uid      TEXT  UNIQUE
  caldav_etag     TEXT
  created_at      TIMESTAMPTZ
  updated_at      TIMESTAMPTZ

Settings                      -- single row (id = 1)
  id                    INTEGER  PK DEFAULT 1
  no_show_deadline_hours INTEGER NOT NULL DEFAULT 24
  currency              TEXT    NOT NULL DEFAULT 'EUR'
  appointment_location  TEXT
  tandc_filename        TEXT    -- filename within DATA_DIR

HMACSecret                    -- single row
  id          INTEGER  PK DEFAULT 1
  secret      BYTEA   NOT NULL
  created_at  TIMESTAMPTZ
```

### 6.2 Booking State Machine

```text
                    ┌─────────┐
   submit session → │ Reserved│ (STATUS:TENTATIVE)
                    └────┬────┘
              admin      │      admin
              confirms   │      rejects
                ┌────────┴────────┐
                ▼                 ▼
          ┌──────────┐     ┌──────────────────┐
          │Confirmed │     │Cancelled by admin │
          │CONFIRMED │     │CANCELLED          │
          └────┬─────┘     └──────────────────┘
               │
    customer cancels          customer cancels
    before deadline           after deadline
         ┌─────┴──────┐
         ▼            ▼
  ┌──────────────┐  ┌─────────┐
  │Cancelled by  │  │ No-show │  ← also set manually by admin
  │customer      │  │CANCELLED│    (e.g. did not appear)
  │CANCELLED     │  └─────────┘
  └──────────────┘

cancel_reason field distinguishes the three CANCELLED states:
  'customer' | 'admin' | 'noshow'
```

---

## 7. URL / Route Map

### 7.1 Customer-facing (no auth required)

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/` | Booking SPA (embedded HTML/JS/CSS) |
| `GET` | `/api/v1/services` | List available services |
| `GET` | `/api/v1/availability` | Available start times (`?service_id=&date=`) |
| `POST` | `/api/v1/sessions` | Create booking session |
| `GET` | `/api/v1/sessions/{id}` | Get session state |
| `POST` | `/api/v1/sessions/{id}/bookings` | Add a booking line to the session |
| `DELETE` | `/api/v1/sessions/{id}/bookings/{lineID}` | Remove a booking line |
| `POST` | `/api/v1/sessions/{id}/submit` | Submit session (final confirmation) |
| `GET` | `/api/v1/bookings/{id}` | Get booking (requires `?token=`) |
| `POST` | `/api/v1/bookings/{id}/reschedule` | Reschedule booking (requires `?token=`) |
| `DELETE` | `/api/v1/bookings/{id}` | Cancel booking (requires `?token=`) |
| `POST` | `/api/v1/bookings/{id}/new-session` | Start new session from booking (requires `?token=`) |

### 7.2 Auth

| Method | Path | Description |
| --- | --- | --- |
| `GET/POST` | `/auth/login` | Username / password login form + handler |
| `GET` | `/auth/apple` | Redirect to Apple OAuth 2.0 / OIDC |
| `GET` | `/auth/apple/callback` | Apple OAuth callback; establishes session cookie |
| `POST` | `/auth/logout` | Invalidate session cookie |

### 7.3 Admin-facing (require auth cookie)

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/admin/api/v1/dashboard` | Dashboard data (bookings of day + pending confirmations) |
| `GET` | `/admin/api/v1/services` | List all services |
| `POST` | `/admin/api/v1/services` | Create service |
| `PUT` | `/admin/api/v1/services/{id}` | Update service |
| `DELETE` | `/admin/api/v1/services/{id}` | Delete service |
| `GET` | `/admin/api/v1/sessions/{id}` | Session review page data |
| `POST` | `/admin/api/v1/sessions/{id}/bookings/{bookingID}/confirm` | Confirm individual booking |
| `POST` | `/admin/api/v1/sessions/{id}/bookings/{bookingID}/reject` | Reject individual booking |
| `POST` | `/admin/api/v1/bookings/{id}/noshow` | Mark booking as no-show |
| `GET` | `/admin/api/v1/settings` | Get general settings |
| `PUT` | `/admin/api/v1/settings` | Update general settings |
| `POST` | `/admin/api/v1/settings/tandc` | Upload T&C PDF (`multipart/form-data`) |
| `GET` | `/admin/api/v1/settings/secret` | Download HMAC secret |
| `POST` | `/admin/api/v1/settings/secret` | Upload / replace HMAC secret |

### 7.4 Infrastructure

| Method | Path | Description |
| --- | --- | --- |
| `*` | `/.well-known/caldav` | RFC 6764 redirect → `/caldav/` |
| `*` | `/principals/` | CalDAV principal PROPFIND |
| `*` | `/caldav/` | CalDAV endpoint (all methods) |
| `GET` | `/api/` | Swagger UI |
| `GET` | `/api/openapi.yaml` | OpenAPI 3.x spec (embedded) |
| `GET` | `/healthz` | Health check (`200 OK`) |

---

## 8. CalDAV Facade Design

### 8.1 Calendar layout (per staff member)

```text
/caldav/user/{staffID}/
  calendars/
    timeslots/          ← Timeslots calendar (admin manages availability)
    bookings/           ← Bookings calendar (booking events appear here)
```

The `Timeslots` calendar is the admin's availability canvas, managed exclusively
via Apple Calendar or any CalDAV-capable client. The `Bookings` calendar is
read-only for CalDAV clients; booking events are written there by schedio's
domain layer, not by CalDAV PUT requests from clients.

### 8.2 Timeslot write path (PUT from Apple Calendar)

```text
PUT /caldav/user/{staffID}/calendars/timeslots/{uid}.ics
  1. caldav.Handler receives request
  2. Parse iCal data from request body
  3. DomainStore.UpsertTimeslot(ctx, staffID, parsed)
  4. Conflict check: DomainStore.ListActiveBookingsInWindow(ctx, staffID, old_start, old_end)
  5. If conflicts → email.SendAdminConflictNotification(conflicts)
  6. Return 201 Created / 204 No Content with new ETag
```

### 8.3 Timeslot delete path (DELETE from Apple Calendar)

```text
DELETE /caldav/user/{staffID}/calendars/timeslots/{uid}.ics
  1. DomainStore.ListActiveBookingsInWindow for deleted window
  2. If conflicts → email.SendAdminConflictNotification(conflicts)
  3. DomainStore.DeleteTimeslot(ctx, staffID, uid)
  4. Return 204 No Content
```

### 8.4 Booking read path (PROPFIND / REPORT from Apple Calendar)

```text
PROPFIND /caldav/user/{staffID}/calendars/bookings/
  1. DomainStore.ListBookings(ctx, staffID, start, end)
  2. Synthesise iCal VEVENT per booking (see §8.5)
  3. Return 207 Multi-Status
```

### 8.5 VEVENT synthesis for bookings

| iCal property | Source |
| --- | --- |
| `UID` | `booking.caldav_uid` (= booking ID) |
| `SUMMARY` | `service.name` |
| `DTSTART` / `DTEND` | `booking.start_at` / `booking.end_at` (UTC, `Z` suffix) |
| `STATUS` | `TENTATIVE` / `CONFIRMED` / `CANCELLED` |
| `ORGANIZER` | `ADMIN_EMAIL` env var |
| `LOCATION` | `settings.appointment_location` |
| `DESCRIPTION` | `service.description` + booking reference + price |
| `SEQUENCE` | `booking.sequence` |
| `URL` | `https://{host}/admin/session/{sessionID}` — admin review link |
| `TRANSP` | `OPAQUE` (confirmed/reserved) / `TRANSPARENT` (cancelled) |

### 8.6 Double-booking prevention

The availability check and the booking write are executed inside a single
atomic critical section:

- **PostgreSQL backend**: a `SELECT … FOR UPDATE` on the timeslot row plus an
  overlap check query run inside a serializable transaction. If a conflicting
  booking is found the transaction is rolled back and `ErrConflict` is returned.
- **Memory backend** (development only): a `sync.Mutex` is held for the
  duration of the check-plus-write.

---

## 9. Auth Subsystem (`internal/auth`)

### 9.1 User sessions

Admin sessions are maintained via a **signed, HTTP-only cookie** (`schedio_session`).
The cookie value is a base64-encoded JSON payload plus an HMAC-SHA256 signature
using a dedicated session signing key (separate from the management-link
HMAC secret, derived from `ADMIN_PASSWORD_HASH` or generated at startup).
The cookie carries `SameSite=Lax; HttpOnly; Secure` attributes.

### 9.2 Apple Sign-In (OAuth 2.0 / OIDC)

Flow:

1. Admin visits `/auth/apple` → redirect to Apple's authorisation endpoint.
2. Apple redirects to `/auth/apple/callback` with `code` and `id_token`.
3. `id_token` is validated (signature, `iss`, `aud`, expiry).
4. `sub` claim is compared to `APPLE_ALLOWED_SUBJECT`; mismatch → `403 Forbidden`.
5. On match: session cookie is set and admin is redirected to `/admin/`.

Environment variables:

| Variable | Purpose |
| --- | --- |
| `APPLE_CLIENT_ID` | OAuth 2.0 client ID (Services ID) |
| `APPLE_TEAM_ID` | Apple Team ID |
| `APPLE_KEY_ID` | Signing key ID |
| `APPLE_PRIVATE_KEY` | ES256 private key (PEM, from Kubernetes Secret) |
| `APPLE_ALLOWED_SUBJECT` | Allowed Apple `sub` claim |

### 9.3 Username / password

- Credential lookup: `ADMIN_USERNAME` + `ADMIN_PASSWORD_HASH` (bcrypt, cost ≥ 12).
- POST `/auth/login` with `application/x-www-form-urlencoded` body.
- On success: session cookie as above.
- Brute-force protection: constant-time compare with `bcrypt.CompareHashAndPassword`.

### 9.4 Auth middleware

`middleware.RequireAdmin` is applied to all `/admin/` routes. It reads and
validates the session cookie; on failure it responds `401 Unauthorized` for
API requests or redirects to `/auth/login` for browser requests
(`Accept: text/html`).

---

## 10. Email Subsystem (`internal/email`)

### 10.1 Transport

SMTP with STARTTLS (port 587) or implicit TLS (port 465), configured via the
`SMTP_*` env vars. The `net/smtp` package is used directly; no third-party
mailer library.

### 10.2 Templates

Templates are Go `text/template` files embedded with `//go:embed`. One
directory per email type:

```text
internal/email/templates/
  reserved/         subject.txt  body.txt  (plain text)
  session-result/   subject.txt  body.txt
  change-summary/   subject.txt  body.txt
  cancellation/     subject.txt  body.txt
  admin-notify/     subject.txt  body.txt
  admin-conflict/   subject.txt  body.txt
```

### 10.3 Email types and triggers

| Email type | Trigger | Recipient |
| --- | --- | --- |
| `reserved` | Customer submits session | Customer |
| `session-result` | Admin completes full session review | Customer |
| `change-summary` | Customer reschedules a booking | Customer |
| `cancellation` | Customer cancels a booking | Customer |
| `admin-notify` | Customer submits session | Administrator (`ADMIN_EMAIL`) |
| `admin-conflict` | Timeslot modification / deletion affects active bookings | Administrator |

### 10.4 `.ics` attachments

Each customer-facing email carries one or more `.ics` attachments:

- **`reserved`**: one `VCALENDAR` per booking (separate file per booking),
  each with `STATUS:TENTATIVE`.
- **`session-result`**: one `VCALENDAR` per confirmed booking with
  `STATUS:CONFIRMED`; cancelled bookings carry `STATUS:CANCELLED`.
- **`change-summary`**: a single `VCALENDAR` containing all of the customer's
  current bookings as individual `VEVENT` components (updated `SEQUENCE` values).
- **`cancellation`**: one `VCALENDAR` for the cancelled booking with
  `STATUS:CANCELLED`.

---

## 11. Token Subsystem (`internal/token`)

Management links use HMAC-SHA256-signed URL tokens:

```text
token = base64url( bookingID + ":" + timestamp + ":" + HMAC(secret, bookingID + ":" + timestamp) )
```

- **Sign**: called when composing the `reserved` email.
- **Verify**: called by every management-link handler before processing.
  Returns the decoded `bookingID` on success or an error on invalid / tampered
  tokens. Tokens do not expire; they are invalidated only by replacing the
  `HMACSecret`.
- The secret stored in `HMACSecret` is loaded at startup by `token.NewSigner`.
  If no secret exists the signer generates a 32-byte random secret, persists it
  via `DomainStore.SetHMACSecret`, and uses it for all subsequent requests.

---

## 12. Background Job — Data Retention (`internal/retention`)

```go
// StartJob runs a goroutine that fires once at startup and then
// every 24 hours, deleting contact and booking data older than
// DATA_RETENTION_DAYS days.
func StartJob(ctx context.Context, store DomainStore, retentionDays int)
```

Deletion logic:

1. Compute `cutoff = now() - retentionDays * 24h`.
2. `DomainStore.DeleteExpiredContacts(ctx, cutoff)` — cascades to
   `BookingSession` and `Booking` rows via `ON DELETE CASCADE` foreign keys.
3. Log count of deleted contacts at `klog.V(2)`.

---

## 13. Startup Sequence

```text
main()
  1. klog.InitFlags(nil)
  2. config.ParseCommandLineArgs(os.Args)        // flags + env vars
  3. store.NewBackend(cfg)
       postgres: open connection, ping, run migrations
       memory:   initialise in-process maps + mutex
  4. token.NewSigner(store)                      // load or generate HMAC secret
  5. email.NewSender(cfg)                        // validate SMTP config
  6. retention.StartJob(ctx, store, cfg.RetentionDays)
  7. server.NewRouter(cfg, store, signer, sender) // build HTTP mux
  8. http.Server{}.ListenAndServe()
  9. wait for SIGINT / SIGTERM
 10. graceful shutdown with 10-second timeout
```

---

## 14. Configuration Reference

All configuration is passed to the binary via CLI flags (existing pattern) or
environment variables. Environment variables override flag defaults.

### 14.1 Existing flags (unchanged)

| Flag | Env | Default | Description |
| --- | --- | --- | --- |
| `--bind-address` | — | `""` | Listen address |
| `--port` | — | `8080` | Listen port |
| `--root-path` | — | `""` | HTTP path prefix |
| `--verbose` | — | `0` | klog verbosity |
| `--dummy` | — | `false` | Use DummyStore |

### 14.2 New environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `STORE_BACKEND` | `memory` | `postgres` or `memory` |
| `POSTGRES_HOST` | — | PostgreSQL host |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_DB` | — | Database name |
| `POSTGRES_USER` | — | Database user |
| `POSTGRES_PASSWORD` | — | Database password |
| `POSTGRES_SSLMODE` | `require` | SSL mode |
| `SMTP_HOST` | — | SMTP server host |
| `SMTP_PORT` | `587` | SMTP port |
| `SMTP_USERNAME` | — | SMTP username |
| `SMTP_PASSWORD` | — | SMTP password |
| `SMTP_FROM_ADDRESS` | — | Sender address |
| `SMTP_FROM_NAME` | — | Sender display name |
| `ADMIN_EMAIL` | — | Administrator e-mail (ORGANIZER in ICS) |
| `ADMIN_USERNAME` | — | Admin username for password login |
| `ADMIN_PASSWORD_HASH` | — | bcrypt hash of admin password |
| `APPLE_CLIENT_ID` | — | Apple OAuth client ID |
| `APPLE_TEAM_ID` | — | Apple Team ID |
| `APPLE_KEY_ID` | — | Apple signing key ID |
| `APPLE_PRIVATE_KEY` | — | ES256 private key (PEM) |
| `APPLE_ALLOWED_SUBJECT` | — | Permitted Apple `sub` claim |
| `DATA_DIR` | `/data` | Directory for T&C PDFs (mount PVC here) |
| `DATA_RETENTION_DAYS` | `30` | Days before contact data is deleted |

---

## 15. Database Migration Strategy

Migration SQL files are embedded in the binary via `//go:embed`:

```text
internal/store/postgres/migrations/
  0001_initial_schema.sql
  0002_add_noshow_column.sql   (example future migration)
  …
```

Migrations are applied in lexicographic order at startup using a simple
home-grown runner (no external tool). A `schema_migrations` table records
applied versions. If the current schema version matches the binary's highest
migration, startup proceeds immediately.

---

## 16. Deployment Topology (Kubernetes)

```yaml
# Sketch — not a production-ready manifest
Deployment: schedio
  containers:
    - name: schedio
      image: schedio:latest
      ports: [{containerPort: 8080}]
      envFrom:
        - secretRef: {name: schedio-secrets}   # POSTGRES_*, SMTP_*, APPLE_*, ADMIN_*
      env:
        - {name: STORE_BACKEND, value: postgres}
        - {name: DATA_DIR, value: /data}
        - {name: DATA_RETENTION_DAYS, value: "30"}
      volumeMounts:
        - {name: data, mountPath: /data}
      livenessProbe:
        httpGet: {path: /healthz, port: 8080}
  volumes:
    - name: data
      persistentVolumeClaim: {claimName: schedio-data}
```

The PostgreSQL database and the `schedio-data` PVC are the only stateful
resources. The schedio Pod itself is stateless beyond these two mounts and can
be replaced or scaled (note: scaling beyond one replica requires distributed
locking for the session-cookie signing key if not derived from a stable secret).

---

## 17. Current Codebase — Gap Analysis and Migration Plan

The existing codebase was created ad-hoc before requirements elicitation. It
implements a working CalDAV server with an embedded web UI skeleton, but it
contains no booking domain logic, no authentication, no email system, and no
PostgreSQL persistence. Every existing file must be assessed against the target
architecture and either kept as-is, refactored, extended, or removed.

The table below uses the following change codes:

| Code | Meaning |
| --- | --- |
| `KEEP` | No changes needed; already correct. |
| `EXTEND` | Add new logic without breaking existing behaviour. |
| `REFACTOR` | Restructure internally; external behaviour changes. |
| `REMOVE` | Delete entirely; replaced by something else. |
| `NEW` | Does not exist yet; must be created from scratch. |

---

### 17.1 `cmd/schedio/main.go` — REFACTOR

**Current state:**

- Reads flags via `config.ParseCommandLineArgs`.
- Selects `DummyStore` (flag `--dummy`) or `MemoryStore`; no PostgreSQL path.
- Constructs `http.Server` + `LoggingMiddleware` + `server.NewRouter`.
- Graceful shutdown on `SIGINT`/`SIGTERM`.

**Required changes:**

1. Replace the `--dummy` / `MemoryStore` branch with `store.NewBackend(cfg)` that
   reads `STORE_BACKEND` and returns the correct implementation.
2. Call `token.NewSigner(ctx, store)` after the store is initialised.
3. Call `email.NewSender(cfg)` and validate SMTP reachability.
4. Call `retention.StartJob(ctx, store, cfg.DataRetentionDays)`.
5. Pass `store`, `signer`, and `sender` into `server.NewRouter`.
6. Remove the direct `calstore.NewDummyStore()` / `calstore.NewMemoryStore()` calls
   from `main`; store selection belongs in `store.NewBackend`.

**Do not change:** graceful shutdown logic, klog initialisation, server timeout values.

---

### 17.2 `internal/config/config.go` — REFACTOR

**Current state:**
Struct fields (all read from flags or a YAML config file today):

```text
Host, Port, BindAddress, RootPath, Verbose, Dummy           ← keep / adapt
SmtpUsername, SmtpPassword, SmtpHost, SmtpPort, AdminMail   ← keep, rename
MailTemplate                                                  ← REMOVE (unused)
CalendarURL, CalendarUsername, CalendarPassword              ← REMOVE (old external CalDAV polling model)
ConfigFile                                                    ← REMOVE (see below)
```

**Required changes:**

1. **Remove** `MailTemplate`, `CalendarURL`, `CalendarUsername`, `CalendarPassword`.
   These are remnants of an earlier design where schedio polled an external CalDAV
   server. The target architecture serves CalDAV directly; no external calendar URL
   is needed.
2. **Remove** `ConfigFile` / YAML-config-file support. Application configuration is
   provided as environment variables in Kubernetes; reading a YAML file for app
   config conflicts with that model and complicates secret injection.
   _Exception_: the per-staff `config.yaml` (staff list) is a separate concern and
   is read by `internal/staff` — it is not the general application config file.
3. **Add** all new fields listed in §14.2 (as environment variable bindings, not
   flags):

```go
StoreBackend        string  // STORE_BACKEND
PostgresHost        string  // POSTGRES_HOST
PostgresPort        int     // POSTGRES_PORT
PostgresDB          string  // POSTGRES_DB
PostgresUser        string  // POSTGRES_USER
PostgresPassword    string  // POSTGRES_PASSWORD
PostgresSSLMode     string  // POSTGRES_SSLMODE
SmtpFromAddress     string  // SMTP_FROM_ADDRESS
SmtpFromName        string  // SMTP_FROM_NAME
AdminUsername       string  // ADMIN_USERNAME
AdminPasswordHash   string  // ADMIN_PASSWORD_HASH
AppleClientID       string  // APPLE_CLIENT_ID
AppleTeamID         string  // APPLE_TEAM_ID
AppleKeyID          string  // APPLE_KEY_ID
ApplePrivateKey     string  // APPLE_PRIVATE_KEY
AppleAllowedSubject string  // APPLE_ALLOWED_SUBJECT
DataDir             string  // DATA_DIR (default: /data)
DataRetentionDays   int     // DATA_RETENTION_DAYS (default: 30)
```

1. Environment variables are read via `os.Getenv` in `ParseCommandLineArgs`;
   they shadow the flag defaults so that both flag-based local development and
   env-var-based Kubernetes deployment work without code changes.

---

### 17.3 `internal/config/cmdline.go` — REFACTOR

**Current state:**

- Registers flags with `flag.FlagSet`.
- Optionally reads a YAML config file via `flagutil.SetFlagsFromEnv`
  (uses `github.com/coreos/pkg/flagutil` — an unrelated external dependency).
- Calls `flagutil.SetFlagsFromEnv` to overlay env-var values.

**Required changes:**

1. Remove the YAML config-file reading path (`--configFile` flag, `readConfigFile`,
   `gopkg.in/yaml.v3` import for app config, `github.com/coreos/pkg/flagutil`).
2. After flag parsing, call `os.Getenv` for each new env var and write into the
   `Config` struct. This replaces `flagutil.SetFlagsFromEnv`.
3. Keep existing flags (`--port`, `--bindAddress`, `--rootPath`, `--verbose`);
   they remain useful for local development.
4. Remove flags for deleted fields: `--smtpUsername`, `--smtpPassword`,
   `--smtpHost`, `--smtpPort`, `--adminMail`, `--mailTemplate`,
   `--calendarUrl`, `--calendarUsername`, `--calendarPassword`, `--dummy`.
   SMTP and auth config must come from env vars only (they carry secrets).

---

### 17.4 `internal/store/store.go` — EXTEND

**Current state:**

- Defines sentinel errors `ErrNotFound`, `ErrConflict`, `ErrReadOnly`.
- Defines `CalendarStore` interface (7 methods, CalDAV primitives).

**Required changes:**

1. Add sentinel error `ErrForbidden` for auth/access-control failures.
2. Add `DomainStore` interface (see §5.2) — all domain-level persistence methods.
3. Add `NewBackend(cfg *config.Config) (Backend, error)` factory function.
   `Backend` is a composite interface embedding both `CalendarStore` and `DomainStore`:

```go
type Backend interface {
    CalendarStore
    DomainStore
}
```

1. Do **not** change the existing `CalendarStore` interface or sentinel errors;
   the CalDAV handler layer depends on them.

---

### 17.5 `internal/store/model.go` — EXTEND

**Current state:**

- Defines CalDAV-level types: `Calendar`, `Event`, `EventStatus`, `EventOpacity`,
  `Attendee`, `ParticipationStatus`.

**Required changes:**

1. Keep all existing CalDAV types unchanged.
2. Add domain model types in a new file `internal/store/model_domain.go` to
   avoid mixing CalDAV-layer and domain-layer structs in one file:

```go
Staff, Service, Timeslot, Contact, BookingSession, Booking,
BookingState, Settings, HMACSecret
```

The `BookingState` type mirrors the state machine in §6.2 and is stored in
`Booking.State`; the `CancelReason` string field distinguishes the three
`CANCELLED` sub-states.

---

### 17.6 `internal/store/memory.go` — EXTEND

**Current state:**
Implements `CalendarStore` only: in-process maps for calendars and events,
protected by `sync.RWMutex`.

**Required changes:**

1. Add `DomainStore` implementation: in-process maps for all domain entities
   (`services`, `timeslots`, `contacts`, `sessions`, `bookings`, `settings`,
   `hmacSecret`), all covered by the existing `sync.RWMutex`.
2. `CreateBooking` is the critical method: it must hold the write lock for the
   full duration of the overlap check and the insert, preventing any concurrent
   goroutine from inserting a conflicting booking between the check and the write.
3. Add the two per-staff CalDAV calendars (`timeslots`, `bookings`) to the
   calendar map at initialisation, replacing the single hard-coded `"default"`
   calendar.
4. The existing `MemoryStore` implements `CalendarStore`; after this change it
   also satisfies `DomainStore`, making it a valid `Backend`.

---

### 17.7 `internal/store/dummy.go` — REMOVE

**Current state:**
Read-only `CalendarStore` pre-seeded with randomly generated events for the
current calendar month. Used via the `--dummy` flag for CalDAV demos.

**Decision:** Remove. The `DummyStore` does not implement `DomainStore` and
cannot satisfy `Backend`. The demo / development use case is covered by the
`MemoryStore` + `config.yaml` staff seed data. Remove the file and the
associated `--dummy` flag references in `cmdline.go` and `main.go`.

---

### 17.8 `internal/caldav/handler.go` — REFACTOR

**Current state:**
~900 lines. Wraps `go-webdav`'s `caldav.Handler`. Contains:

- `davEnforcingWriter` — ensures correct `DAV:` response header.
- `calendarCollectionDepth1Multistatus` — synthesises `current-user-privilege-set`
  on event resources (iOS interoperability).
- `calendarObjectMultistatus` — synthesises per-event privilege set.
- Outbox POST handler for `VFREEBUSY`.

**Required changes:**

1. Change the constructor to accept `store.Backend` instead of `store.CalendarStore`.
   The CalDAV layer needs access to `DomainStore` methods for write translation.
2. **Timeslot write path**: when a `PUT` arrives on a path underneath
   `.../timeslots/`, parse the iCal body and call
   `DomainStore.UpsertTimeslot`. After the write, call
   `DomainStore.ListActiveBookingsInWindow` for the old and new time window; if
   conflicts exist, call `email.SendAdminConflictNotification`. Do **not** forward
   to the inner `go-webdav` handler for timeslot writes; the store is the source
   of truth.
3. **Booking calendar write guard**: reject `PUT`/`DELETE` requests to the
   `.../bookings/` calendar with `405 Method Not Allowed`. Bookings are written
   exclusively by the domain layer.
4. **Booking read path**: `PROPFIND`/`REPORT` on `.../bookings/` must call
   `DomainStore.ListBookingsForSession` / range query and synthesise VEVENTs
   using the logic in §8.5, rather than calling `CalendarStore.ListEvents`.
5. **Freebusy**: the outbox POST handler should call
   `DomainStore.ListActiveBookingsInWindow` for accurate busy-period calculation
   rather than reading raw CalDAV events.
6. **Keep unchanged**: `davEnforcingWriter`, `davCapabilities` constant,
   `calendarCollectionDepth1Multistatus`, `calendarObjectMultistatus`,
   iOS-interoperability header logic. These are hard-won and must not regress.

---

### 17.9 `internal/caldav/adapter.go` — EXTEND

**Current state:**
Bidirectional conversion between `store.Event` and `go-ical` VEVENT:

- `icalToEvent` — iCal VEVENT → `store.Event`
- `eventToObject` — `store.Event` → iCal VEVENT

**Required changes:**

1. Add `timeslotToObject(t *store.Timeslot) *ical.Component` — synthesises a
   VEVENT representing a timeslot availability window for the CalDAV calendar.
2. Add `objectToTimeslot(obj *ical.Component) (*store.Timeslot, error)` — parses
   a VEVENT from an Apple Calendar `PUT` into a `store.Timeslot` (reads `RRULE`,
   `RECURRENCE-ID`, `DTSTART`, `DTEND`, `UID`, `LAST-MODIFIED`).
3. Add `bookingToObject(b *store.Booking, svc *store.Service, settings *store.Settings, adminEmail string) *ical.Component` — synthesises the VEVENT for a booking (§8.5).
4. Keep existing `icalToEvent` and `eventToObject` for the memory-backend path.

---

### 17.10 `internal/caldav/discovery.go` — REFACTOR

**Current state:**
Handles:

- `/.well-known/caldav` → 301 redirect.
- `PROPFIND /` — returns `current-user-principal`.
- `PROPFIND /caldav/user/` — returns principal properties (calendar home set,
  inbox, outbox, schedule URLs, privilege set, …) for a **single hard-coded user**.

**Required changes:**

1. Replace the hard-coded single-user principal path with a per-staff path:
   `/caldav/user/{staffID}/`. For a single-staff deployment the behaviour is
   identical; for multi-staff deployments the correct staff is resolved from the
   authenticated session or CalDAV credentials.
2. The `calendar-home-set` href must point to `/caldav/user/{staffID}/calendars/`
   and `schedule-default-calendar-URL` to
   `/caldav/user/{staffID}/calendars/bookings/`.
3. For the memory backend (development) the single hard-coded staff ID `"default"`
   is acceptable; for the postgres backend the staff list is read from `config.yaml`
   via the staff loader (§4 new packages).

---

### 17.11 `internal/handlers/health.go` — KEEP

Returns `200 OK` with no body at `/healthz`. No changes needed.

---

### 17.12 `internal/handlers/openapi.go` — REFACTOR

**Current state:**

- Serves the OpenAPI spec template at `/api/docs/openapi.yaml`.
- Serves Swagger UI at `/api/docs/`.
- Uses `github.com/swaggo/http-swagger` (third-party dependency).

**Required changes:**

1. Move the Swagger UI from `/api/docs/` to `/api/` (per §7.4). Update
   `server.NewRouter` route registration accordingly.
2. The `openapi.yaml` template must be updated to describe all new endpoints
   (§7.1–7.3). The embedded file lives in `api/openapi.yaml`.
3. Consider replacing `github.com/swaggo/http-swagger` with an embedded copy of
   the Swagger UI static assets served from the binary itself, to avoid a
   third-party runtime dependency and keep the architecture consistent with the
   standard-library principle. If the dependency is retained, document the
   exception explicitly.

---

### 17.13 `internal/handlers/utils.go` — KEEP

HTTP utility helpers (`buildServerRootURLFromRequest`, etc.). No changes needed.

---

### 17.14 `internal/handlers/web.go` — EXTEND

**Current state:**
Serves static assets from the `web/` embedded filesystem (`HandleWebUserInterface`).
The `web/` directory currently contains a minimal HTML/JS/CSS scaffold with no
booking workflow.

**Required changes:**

1. The handler itself (`HandleWebUserInterface`) is correct and requires no
   changes; it serves whatever is in the embedded `web/` filesystem.
2. The booking SPA in `web/html/`, `web/js/`, `web/css/` must be built out to
   implement the full 8-step customer booking flow and the admin UI. This is the
   largest frontend development effort but does not require changes to the handler.
3. The admin UI (session review, service management, settings, dashboard) is
   served from the same embedded filesystem under `/admin/`. No separate handler
   is needed.

---

### 17.15 `internal/middleware/middleware.go` — EXTEND

**Current state:**
`LoggingMiddleware` only — logs per-request at klog verbosity levels 1–3.

**Required changes:**

Add `RequireAdmin(next http.Handler) http.Handler`:

- Reads the `schedio_session` cookie from the request.
- Validates the HMAC-SHA256 signature and expiry embedded in the cookie value.
- On success: attaches the authenticated admin identity to `r.Context()` and
  calls `next`.
- On failure with `Accept: text/html`: redirects to `/auth/login`.
- On failure without `Accept: text/html` (API): responds `401 Unauthorized`
  with `Content-Type: application/json`.

The existing `LoggingMiddleware` is unchanged and wraps the entire mux.
`RequireAdmin` is applied per-route inside `server.NewRouter`.

---

### 17.16 `internal/server/router.go` — REFACTOR

**Current state:**
Registers six route groups:

- `/.well-known/caldav` — discovery redirect
- `/calendar/dav/` — legacy Apple redirect
- `{rootPath}/principals/` — CalDAV principal
- `{rootPath}/healthz` — health check
- `{rootPath}/api/docs/` — Swagger UI
- `{rootPath}/caldav/` — CalDAV endpoint
- `{rootPath}/` — web UI (catch-all)

**Required changes:**

1. **Update constructor signature**:

```go
// Before
func NewRouter(args *config.Config, store calstore.CalendarStore) http.Handler

// After
func NewRouter(args *config.Config, store store.Backend, signer *token.Signer, sender *email.Sender) http.Handler
```

1. **Move Swagger UI** from `/api/docs/` to `/api/`.

1. **Add customer REST routes** (no auth):

```text
GET  {root}/api/v1/services
GET  {root}/api/v1/availability
POST {root}/api/v1/sessions
GET  {root}/api/v1/sessions/{id}
POST {root}/api/v1/sessions/{id}/bookings
DELETE {root}/api/v1/sessions/{id}/bookings/{lineID}
POST {root}/api/v1/sessions/{id}/submit
GET  {root}/api/v1/bookings/{id}
POST {root}/api/v1/bookings/{id}/reschedule
DELETE {root}/api/v1/bookings/{id}
POST {root}/api/v1/bookings/{id}/new-session
```

1. **Add auth routes** (no auth):

```text
GET/POST {root}/auth/login
GET      {root}/auth/apple
GET      {root}/auth/apple/callback
POST     {root}/auth/logout
```

1. **Add admin routes** (wrapped with `middleware.RequireAdmin`):

```text
GET    {root}/admin/api/v1/dashboard
GET/POST/PUT/DELETE {root}/admin/api/v1/services{...}
GET/POST {root}/admin/api/v1/sessions/{id}{...}
POST   {root}/admin/api/v1/bookings/{id}/noshow
GET/PUT {root}/admin/api/v1/settings
POST   {root}/admin/api/v1/settings/tandc
GET/POST {root}/admin/api/v1/settings/secret
```

1. Remove the `/calendar/dav/` legacy redirect if it is no longer needed, or
   keep it for backward compatibility with existing Apple Calendar configurations.

---

### 17.17 New packages — CREATE FROM SCRATCH

The following packages do not exist yet and must be created in full:

| Package | Key files | Notes |
| --- | --- | --- |
| `internal/domain` | `availability.go`, `booking.go`, `conflict.go` | Pure business logic; no HTTP, no DB. Testable independently. |
| `internal/auth` | `apple.go`, `password.go`, `session.go` | Apple OIDC, bcrypt, cookie signing |
| `internal/email` | `sender.go`, `templates.go`, `reserved.go`, `result.go`, `change.go`, `cancel.go`, `admin.go` | One file per email type |
| `internal/email/templates/` | `reserved/`, `session-result/`, `change-summary/`, `cancellation/`, `admin-notify/`, `admin-conflict/` | `subject.txt` + `body.txt` per type |
| `internal/token` | `token.go` | HMAC-SHA256 sign / verify |
| `internal/retention` | `retention.go` | Background goroutine |
| `internal/staff` | `staff.go` | YAML config loader for staff list; read-only at startup |
| `internal/store/postgres` | `store.go`, `migrations/0001_initial_schema.sql` | Implements `store.Backend` using `database/sql` + `lib/pq` |
| `internal/handlers/customer` | `service.go`, `availability.go`, `session.go`, `booking.go` | Customer-facing REST handlers |
| `internal/handlers/admin` | `dashboard.go`, `service.go`, `session.go`, `booking.go`, `settings.go` | Admin REST handlers |
| `internal/handlers/auth` | `auth.go` | Login/logout/Apple-callback handlers |

---

### 17.18 Dependency changes

| Dependency | Action | Reason |
| --- | --- | --- |
| `github.com/coreos/pkg/flagutil` | **REMOVE** | Used only for YAML config file + env-var flag overlay; replaced by direct `os.Getenv` |
| `gopkg.in/yaml.v3` | **KEEP** | Still needed for `internal/staff` YAML config loading |
| `github.com/emersion/go-webdav` | **KEEP** | CalDAV protocol implementation |
| `github.com/emersion/go-ical` | **KEEP** | iCal parsing / generation |
| `github.com/swaggo/http-swagger` | **EVALUATE** | Consider replacing with embedded Swagger UI assets to stay standard-library |
| `k8s.io/klog/v2` | **KEEP** | Logging |
| `lib/pq` or `pgx/v5` | **ADD** | PostgreSQL driver |

---

### 17.19 Migration sequencing

Recommended implementation order to keep the binary buildable and testable at
every step:

1. **Config refactor** — `internal/config`: remove dead fields, add new env vars.
   Build still passes; no behaviour change yet.
2. **Store extension** — `internal/store/store.go` + `model_domain.go`:
   add `DomainStore` interface and domain types. Add `Backend` composite interface.
3. **MemoryStore domain extension** — `internal/store/memory.go`:
   implement `DomainStore`. All existing CalDAV tests continue to pass.
4. **Remove DummyStore** — delete `internal/store/dummy.go`; remove `--dummy` flag.
5. **`store.NewBackend` factory** — selects `MemoryStore` until the postgres
   backend exists; update `main.go` to use it.
6. **Domain logic** — `internal/domain`: availability, booking, conflict.
   Unit-tested in isolation using the `MemoryStore`.
7. **Token subsystem** — `internal/token`. Unit-tested.
8. **Auth subsystem** — `internal/auth`. Unit-tested.
9. **Email subsystem** — `internal/email` with stub SMTP for tests.
10. **Retention job** — `internal/retention`.
11. **REST handlers** — customer, then admin, then auth. Each wired into router.
12. **CalDAV handler refactor** — translate writes to `DomainStore`; update discovery
    for per-staff paths. iOS-interoperability tests must continue to pass.
13. **PostgreSQL backend** — `internal/store/postgres` with migrations.
    `store.NewBackend` now returns `PostgresStore` when `STORE_BACKEND=postgres`.
14. **Frontend** — SPA booking flow + admin UI in `web/`.
15. **OpenAPI spec** — fill out `api/openapi.yaml` to cover all endpoints.
16. **Integration tests** — end-to-end tests using `MemoryStore` backend.
