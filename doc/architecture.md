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
  ├── retention.StartJob()            // background goroutine; runs at AUTOMATED_TASKS_RUN_AT (default 08:00)
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
| `internal/auth` | Apple Sign-In (OAuth 2.0 / OIDC), bcrypt username/password, role-based session management; loads users from `USERS_CONFIG_FILE` at startup and calls `DomainStore.SyncUsers` |
| `internal/email` | SMTP client, Go `text/template` templates, domain-level send functions (reserved, result, change, cancel, admin-notify, admin-conflict) |
| `internal/token` | HMAC-SHA256 token sign / verify for management links |
| `internal/retention` | Background goroutine (runs daily at `AUTOMATED_TASKS_RUN_AT`): reminder e-mails (Pass 0), billing (Pass 1), retention-due notification (Pass 2), confirmation-link expiry check (Pass 3), pending-deletion escalation |
| `internal/billing` | Invoice generation and file storage (`DATA_DIR/invoices/`); triggers Staff e-mail with invoice content |
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
    // --- Staff / Users ---
    SyncUsers(ctx, users []*User) error // called at startup to sync config file into store
    ListStaff(ctx) ([]*Staff, error)
    GetStaff(ctx, id) (*Staff, error)
    GetUserByEmail(ctx, email string) (*User, error)

    // --- Services ---
    ListServices(ctx) ([]*Service, error)
    GetService(ctx, id) (*Service, error)
    CreateService(ctx, s *Service) error
    UpdateService(ctx, s *Service) error
    DeleteService(ctx, id) error        // returns ErrConflict if active bookings exist

    // --- Availability windows ---
    ListAvailability(ctx, userID string, start, end time.Time) ([]*Availability, error)
    GetAvailability(ctx, userID, uid string) (*Availability, error)
    UpsertAvailability(ctx, a *Availability) error
    DeleteAvailability(ctx, userID, uid string) error

    // --- Contacts ---
    GetOrCreateContact(ctx, email string, c *Contact) (*Contact, error)
    GetContact(ctx, id) (*Contact, error)
    // UpdateContactLastAppointment updates last_appointment_end_at to the given
    // appointment end time when it is later than the stored value, and resets
    // billing_generated = false and retention_state = 'active' in that case.
    UpdateContactLastAppointment(ctx, contactID string, appointmentEndAt time.Time) error

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
    ListActiveBookingsInWindow(ctx, userID string, start, end time.Time) ([]*Booking, error)

    // --- Settings ---
    GetSettings(ctx) (*Settings, error)
    UpdateSettings(ctx, s *Settings) error
    GetHMACSecret(ctx) ([]byte, error)
    SetHMACSecret(ctx, secret []byte) error

    // --- Retention ---
    // ListRetentionDue returns contacts whose last_appointment_end_at + retention
    // period has passed and whose retention_state = 'active'.
    ListRetentionDue(ctx, retentionPeriod time.Duration) ([]*Contact, error)
    // MarkRetentionNotified sets retention_state = 'notified' and
    // retention_notified_at = now() for the given contact.
    MarkRetentionNotified(ctx, contactID string) error
    // ListConfirmationExpired returns contacts where retention_state = 'notified'
    // and retention_notified_at + 7 days ≤ now.
    ListConfirmationExpired(ctx) ([]*Contact, error)
    // AddToPendingDeletion sets retention_state = 'pending_deletion'.
    AddToPendingDeletion(ctx, contactID string) error
    // ListPendingDeletion returns all contacts with retention_state = 'pending_deletion'.
    ListPendingDeletion(ctx) ([]*Contact, error)
    // DeleteContact permanently deletes the contact and all associated
    // BookingSession and Booking rows (cascade). Called by the confirmation-link
    // handler and by the Staff pending-deletion UI.
    DeleteContact(ctx, contactID string) error

    // --- Billing ---
    // ListBillingDue returns contacts whose last_appointment_end_at ≤ now
    // and billing_generated = false.
    ListBillingDue(ctx) ([]*Contact, error)
    // MarkBillingGenerated sets billing_generated = true for the contact.
    MarkBillingGenerated(ctx, contactID string) error
    // ListBookingsForContact returns all non-cancelled bookings for a contact,
    // ordered by start_at ascending.
    ListBookingsForContact(ctx, contactID string) ([]*Booking, error)

    // --- Reminders ---
    // ListBookingsDueReminder returns confirmed bookings whose start_at falls
    // exactly leadDays calendar days from today (in the server's local timezone)
    // and whose reminded_at IS NULL.
    ListBookingsDueReminder(ctx context.Context, leadDays int) ([]*Booking, error)
    // MarkReminderSent sets reminded_at = now() for the given booking.
    MarkReminderSent(ctx context.Context, bookingID string) error
}
```

---

## 6. Domain Model

### 6.1 Entities

```text
User
  id                  UUID  PK
  email               TEXT  NOT NULL  UNIQUE  -- login username; also the CalDAV principal name for staff
  password_hash       TEXT  NOT NULL          -- bcrypt hash (cost ≥ 12)
  role                TEXT  NOT NULL          -- 'staff' | 'administrator'
  apple_oauth_enabled BOOLEAN NOT NULL DEFAULT FALSE
  apple_subject       TEXT                    -- Apple id_token `sub` claim; NULL when apple_oauth_enabled = FALSE
  name                TEXT                    -- display name derived from email or set manually
  created_at          TIMESTAMPTZ
  updated_at          TIMESTAMPTZ

-- Staff is an alias view / sub-set of User where role = 'staff'.
-- Availability and Booking foreign keys reference User.id (staff users only).

Service
  id                UUID  PK
  name              TEXT  NOT NULL
  summary           TEXT  NOT NULL              -- short one-line label for the selection list
  description       TEXT                        -- full-length detail text
  price             NUMERIC(10,2)  NOT NULL
  duration_minutes  INTEGER  NOT NULL
  daily_limit       INTEGER  NOT NULL DEFAULT 0  -- 0 = unlimited
  created_at        TIMESTAMPTZ
  updated_at        TIMESTAMPTZ

Availability
  id              UUID  PK
  user_id         UUID  FK → User  -- must be a user with role = 'staff'
  caldav_uid      TEXT  UNIQUE  -- iCal UID
  caldav_etag     TEXT
  start_at        TIMESTAMPTZ  NOT NULL
  end_at          TIMESTAMPTZ  NOT NULL
  rrule           TEXT         -- iCal RRULE value; NULL for single events
  recurrence_id   TIMESTAMPTZ  -- set for individual overrides of a recurring series
  created_at      TIMESTAMPTZ
  updated_at      TIMESTAMPTZ

Contact
  id                      UUID  PK
  first_name              TEXT  NOT NULL
  last_name               TEXT  NOT NULL
  email                   TEXT  NOT NULL  UNIQUE
  phone                   TEXT  NOT NULL
  created_at              TIMESTAMPTZ
  last_appointment_end_at TIMESTAMPTZ  -- updated to max(existing, new end time) on each booking; drives retention and billing
  retention_state         TEXT  NOT NULL DEFAULT 'active'  -- 'active' | 'notified' | 'pending_deletion'
  retention_notified_at   TIMESTAMPTZ  -- set when retention-notify e-mail is sent
  billing_generated       BOOLEAN  NOT NULL DEFAULT FALSE  -- reset to false when last_appointment_end_at moves forward

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
  user_id         UUID  FK → User   -- staff user who owns the booked availability window
  start_at        TIMESTAMPTZ  NOT NULL
  end_at          TIMESTAMPTZ  NOT NULL
  state           TEXT  -- see §6.2
  cancel_reason   TEXT  -- 'customer' | 'admin' | 'noshow' | NULL
  sequence        INTEGER  NOT NULL DEFAULT 0
  caldav_uid      TEXT  UNIQUE
  caldav_etag     TEXT
  created_at      TIMESTAMPTZ
  updated_at      TIMESTAMPTZ
  reminded_at     TIMESTAMPTZ  -- set when reminder e-mail is sent; NULL means not yet sent

Settings                      -- single row (id = 1)
  id                        INTEGER  PK DEFAULT 1
  no_show_deadline_hours    INTEGER  NOT NULL DEFAULT 24
  retention_period_days     INTEGER  NOT NULL DEFAULT 30    -- admin-configurable; DATA_RETENTION_DAYS seeds this on first startup
  reminder_lead_time_days   INTEGER  NOT NULL DEFAULT 1     -- admin-configurable number of days before an appointment at which the reminder e-mail is sent
  sender_name               TEXT     NOT NULL DEFAULT 'Schedio Buchungssystem'  -- From: display name for customer e-mails; seeded from SMTP_FROM_NAME
  currency                  TEXT     NOT NULL DEFAULT 'EUR'
  appointment_location      TEXT
  tandc_filename            TEXT    -- filename within DATA_DIR

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
| `GET` | `/auth/apple/available` | Returns `{ apple_enabled: bool }` for a given `?username=` without revealing whether the account exists |
| `POST` | `/auth/logout` | Invalidate session cookie |
| `GET` | `/auth/me` | Returns `{ username, role }` for the current session; `401` when unauthenticated |

### 7.3 Admin-facing (require auth cookie)

The `Role` column states which role(s) may call the endpoint. The auth middleware
enforces this at the route level: `staff` routes reject `administrator` sessions
with `403 Forbidden` and vice versa. Both roles may call `staff | administrator`
routes.

| Method | Path | Role | Description |
| --- | --- | --- | --- |
| `GET` | `/admin/api/v1/dashboard` | `staff` | Dashboard data (bookings of day + pending confirmations) |
| `GET` | `/admin/api/v1/sessions/{id}` | `staff` | Session review page data |
| `POST` | `/admin/api/v1/sessions/{id}/bookings/{bookingID}/confirm` | `staff` | Confirm individual booking |
| `POST` | `/admin/api/v1/sessions/{id}/bookings/{bookingID}/reject` | `staff` | Reject individual booking |
| `POST` | `/admin/api/v1/bookings/{id}/noshow` | `staff` | Mark booking as no-show |
| `GET` | `/admin/api/v1/retention/pending` | `staff` | List contacts in `pending_deletion` state |
| `DELETE` | `/admin/api/v1/retention/pending/{contactID}` | `staff` | Permanently delete a contact and all their bookings |
| `GET` | `/admin/api/v1/retention/confirm` | — (signed token) | Confirm deletion via e-mail link; token encodes contactID + 7-day expiry |
| `GET` | `/admin/api/v1/services` | `administrator` | List all services |
| `POST` | `/admin/api/v1/services` | `administrator` | Create service |
| `PUT` | `/admin/api/v1/services/{id}` | `administrator` | Update service |
| `DELETE` | `/admin/api/v1/services/{id}` | `administrator` | Delete service |
| `GET` | `/admin/api/v1/settings` | `administrator` | Get general settings |
| `PUT` | `/admin/api/v1/settings` | `administrator` | Update general settings |
| `POST` | `/admin/api/v1/settings/tandc` | `administrator` | Upload T&C PDF (`multipart/form-data`) |
| `GET` | `/admin/api/v1/settings/secret` | `administrator` | Download HMAC secret |
| `POST` | `/admin/api/v1/settings/secret` | `administrator` | Upload / replace HMAC secret |

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
    availability/       ← Availability-Calendar (admin manages availability)
    bookings/           ← Bookings calendar (booking events appear here)
```

The `Availability-Calendar` is the admin's availability canvas, managed exclusively
via Apple Calendar or any CalDAV-capable client. The `Bookings` calendar is
read-only for CalDAV clients; booking events are written there by schedio's
domain layer, not by CalDAV PUT requests from clients.

### 8.2 Availability write path (PUT from Apple Calendar)

```text
PUT /caldav/user/{staffID}/calendars/availability/{uid}.ics
  1. caldav.Handler receives request
  2. Parse iCal data from request body
  3. DomainStore.UpsertAvailability(ctx, staffID, parsed)
  4. Conflict check: DomainStore.ListActiveBookingsInWindow(ctx, staffID, old_start, old_end)
  5. If conflicts → email.SendAdminConflictNotification(conflicts)
  6. Return 201 Created / 204 No Content with new ETag
```

### 8.3 Availability delete path (DELETE from Apple Calendar)

```text
DELETE /caldav/user/{staffID}/calendars/availability/{uid}.ics
  1. DomainStore.ListActiveBookingsInWindow for deleted window
  2. If conflicts → email.SendAdminConflictNotification(conflicts)
  3. DomainStore.DeleteAvailability(ctx, staffID, uid)
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

- **PostgreSQL backend**: a `SELECT … FOR UPDATE` on the availability row plus an
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

1. User visits `/auth/apple` → the login username typed so far is forwarded as
   state; redirect to Apple's authorisation endpoint.
2. Apple redirects to `/auth/apple/callback` with `code` and `id_token`.
3. `id_token` is validated (signature, `iss`, `aud`, expiry).
4. `sub` claim is looked up against `User.apple_subject` in the loaded users
   config; no match → `403 Forbidden`.
5. Matched user must have `apple_oauth_enabled = true`; otherwise → `403 Forbidden`.
6. On success: session cookie is set (carrying `userID` and `role`) and the
   user is redirected to the appropriate landing page (`/admin/` for
   administrator, `/admin/` for staff).

Global environment variables (Apple Sign-In must be configured for any
per-user `apple_oauth_enabled: true` to take effect):

| Variable | Purpose |
| --- | --- |
| `APPLE_CLIENT_ID` | OAuth 2.0 client ID (Services ID) |
| `APPLE_TEAM_ID` | Apple Team ID |
| `APPLE_KEY_ID` | Signing key ID |
| `APPLE_PRIVATE_KEY` | ES256 private key (PEM, from Kubernetes Secret) |

### 9.3 Username / password

- All named users are loaded from the YAML file at `USERS_CONFIG_FILE` path at
  startup; the parsed entries are written to the store via `DomainStore.SyncUsers`.
- `POST /auth/login` with `application/x-www-form-urlencoded` body
  (`username`, `password`).
- Credential lookup: `GetUserByEmail(username)` → `bcrypt.CompareHashAndPassword`
  against the stored hash (cost ≥ 12).
- On success: session cookie set with `userID` and `role`.
- Brute-force protection: constant-time compare; fixed-duration artificial delay
  on failure.

### 9.4 `/auth/apple/available` discovery endpoint

`GET /auth/apple/available?username=<email>` returns
`{ "apple_enabled": true|false }`. The response is `true` only when:

1. Apple Sign-In env vars (`APPLE_CLIENT_ID`, etc.) are all set, **and**
2. a user record with that email exists **and** has `apple_oauth_enabled = true`.

To prevent user-enumeration the response is always `200 OK` with
`{ "apple_enabled": false }` for any unknown email (never `404`).

### 9.5 Auth middleware

Two middleware functions are provided in `internal/middleware`:

- `RequireAuth` — validates the session cookie; responds `401 Unauthorized`
  for API requests or redirects to `/auth/login` for browser requests.
  Applied to all `/admin/` routes and to the CalDAV endpoint.
- `RequireRole(role string)` — checks that the authenticated session's role
  matches the required role; responds `403 Forbidden` on mismatch. Applied
  per-route group as described in §7.3.

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
  retention-notify/ subject.txt  body.txt  -- sent to all Staff when retention deadline reached
  billing-invoice/  subject.txt  body.txt  -- sent to all Staff with invoice content
  reminder/         subject.txt  body.txt  -- sent to Customer reminder_lead_time_days before their confirmed appointment
```

### 10.3 Email types and triggers

| Email type | Trigger | Recipient |
| --- | --- | --- |
| `reserved` | Customer submits session | Customer |
| `session-result` | Admin completes full session review | Customer |
| `change-summary` | Customer reschedules a booking | Customer |
| `cancellation` | Customer cancels a booking | Customer |
| `admin-notify` | Customer submits session | Administrator (`ADMIN_EMAIL`) |
| `admin-conflict` | Availability modification / deletion affects active bookings | Administrator |
| `retention-notify` | Contact's `last_appointment_end_at + retention_period_days ≤ now` and `retention_state = 'active'` | All Staff users (all users with `role = 'staff'`) |
| `billing-invoice` | Contact's `last_appointment_end_at ≤ now` and `billing_generated = false` | All Staff users |
| `reminder` | Confirmed booking whose `start_at` is exactly `reminder_lead_time_days` calendar days from today and `reminded_at IS NULL` | Customer |

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

## 12. Background Job — Reminders, Data Retention and Billing (`internal/retention`, `internal/billing`)

A single background goroutine (started by `retention.StartJob`) wakes up daily
at the time configured by `AUTOMATED_TASKS_RUN_AT` (default `08:00` local server
time). It performs four sequential passes:

```go
func StartJob(ctx context.Context, store DomainStore, email *email.Sender, billing *billing.Service)
```

### Pass 0 — Reminder e-mails

1. `DomainStore.GetSettings(ctx)` to read current `reminder_lead_time_days`.
2. `DomainStore.ListBookingsDueReminder(ctx, leadDays)` — confirmed bookings
   whose `start_at` falls exactly `reminder_lead_time_days` calendar days from
   today (server local time) and whose `reminded_at` IS NULL.
3. For each booking: send a `reminder` e-mail to the customer containing the
   service name, appointment date and time (formatted in server local timezone),
   appointment location (from Settings), and the management link.
4. `DomainStore.MarkReminderSent(ctx, bookingID)` on success.
5. Log count at `klog.V(2)`.

### Pass 1 — Billing

1. `DomainStore.ListBillingDue(ctx)` — contacts where
   `last_appointment_end_at ≤ now` and `billing_generated = false`.
2. For each contact: call `billing.GenerateAndSend(ctx, contact)` which:
   a. `DomainStore.ListBookingsForContact(ctx, contactID)` — all non-cancelled bookings.
   b. Render invoice as plain text (`DATA_DIR/invoices/yyyy-mm-dd-<LastName>-<FirstName>.txt`).
   c. Write file to disk.
   d. Send `billing-invoice` e-mail to all Staff users.
3. `DomainStore.MarkBillingGenerated(ctx, contactID)` on success.
4. Log count at `klog.V(2)`.

### Pass 2 — Retention notification

1. `DomainStore.GetSettings(ctx)` to read current `retention_period_days`.
2. `DomainStore.ListRetentionDue(ctx, retentionPeriod)` — contacts where
   `last_appointment_end_at + retention_period_days ≤ now` and `retention_state = 'active'`.
3. For each contact:
   a. Send `retention-notify` e-mail to all Staff users. The e-mail includes
      a signed deletion-confirmation link (`/admin/api/v1/retention/confirm?token=…`)
      valid for **7 days**, signed with the same HMAC infrastructure as management links.
   b. `DomainStore.MarkRetentionNotified(ctx, contactID)`.
4. Log count at `klog.V(2)`.

### Pass 3 — Confirmation expiry

1. `DomainStore.ListConfirmationExpired(ctx)` — contacts where
   `retention_state = 'notified'` and `retention_notified_at + 7 days ≤ now`.
2. For each contact: `DomainStore.AddToPendingDeletion(ctx, contactID)`.
3. Log count at `klog.V(2)`.

### Deletion confirmation endpoint

`GET /admin/api/v1/retention/confirm?token=<signed>`

- No session cookie required; the signed token authenticates the action.
- Token encodes `contactID` + expiry timestamp (7 days from issue), signed with
  the HMAC secret via `internal/token`.
- On valid token: `DomainStore.DeleteContact(ctx, contactID)`.
- On expired / invalid token: `410 Gone` with a human-readable message.

### Cancelled-booking rule

Only non-cancelled bookings (state `reserved` or `confirmed`) contribute to
`last_appointment_end_at`. If all of a contact's bookings are cancelled,
`last_appointment_end_at` is `NULL`; the contact is skipped by all retention
and billing passes until a non-cancelled booking exists.

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
  6. billing.NewService(store, email, cfg.DataDir) // invoice generator + file store
  7. retention.StartJob(ctx, store, email, billingSvc) // background: billing + retention
  8. server.NewRouter(cfg, store, signer, sender) // build HTTP mux
  9. http.Server{}.ListenAndServe()
 10. wait for SIGINT / SIGTERM
 11. graceful shutdown with 10-second timeout
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
| `USERS_CONFIG_FILE` | `/etc/schedio/users.yaml` | Path to the YAML user/role configuration file |
| `APPLE_CLIENT_ID` | — | Apple OAuth client ID (required for Apple Sign-In) |
| `APPLE_TEAM_ID` | — | Apple Team ID |
| `APPLE_KEY_ID` | — | Apple signing key ID |
| `APPLE_PRIVATE_KEY` | — | ES256 private key (PEM) |
| `DATA_DIR` | `/data` | Directory for T&C PDFs and invoice files (`DATA_DIR/invoices/`) — mount PVC here |
| `DATA_RETENTION_DAYS` | `30` | Seed value for `Settings.retention_period_days` on first startup only; ignored once the DB row exists |
| `AUTOMATED_TASKS_RUN_AT` | `08:00` | Time of day (HH:MM, 24-hour, server local time) at which the daily background job runs; applies to all automated tasks (reminders, billing, retention) |

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
4. Call `billing.NewService(store, emailSender, cfg.DataDir)` and `retention.StartJob(ctx, store, emailSender, billingSvc)`.
5. Pass `store`, `signer`, and `sender` into `server.NewRouter`.
6. Remove the direct `calstore.NewDummyStore()` / `calstore.NewMemoryStore()` calls
   from `main`; store selection belongs in `store.NewBackend`.

**Changed startup call:** `retention.StartJob(ctx, store, emailSender, billingSvc)` — no
longer reads `cfg.DataRetentionDays` directly; fetches `retention_period_days` from
`Settings` at each daily run so admin changes take effect without a restart.

**Current state:**
Struct fields (all read from flags or a YAML config file today):

```text
Host, Port, BindAddress, RootPath, Verbose, Dummy           ← keep / adapt
SmtpUsername, SmtpPassword, SmtpHost, SmtpPort, AdminMail   ← keep, rename
MailTemplate                                                  ← REMOVE (unused)
CalendarURL                                                  ← KEEP (seeds Settings.CalendarURL on first startup; see §14.2)
CalendarUsername, CalendarPassword                           ← REMOVE (old external CalDAV polling model)
ConfigFile                                                    ← REMOVE (see below)
```

**Required changes:**

1. **Remove** `MailTemplate`, `CalendarUsername`, `CalendarPassword`.
   These are remnants of an earlier design where schedio polled an external CalDAV
   server. The target architecture serves CalDAV directly.
   **Keep** `CalendarURL` — it is repurposed as the CalDAV server URL shown to clients
   and is seeded into `Settings.CalendarURL` at first startup (same pattern as `SenderName`).
   The administrator can subsequently change it via the General Settings admin UI
   without a server restart.
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
SenderName          string  // SMTP_SENDER_NAME
UsersConfigFile     string  // USERS_CONFIG_FILE (default: /etc/schedio/users.yaml)
AppleClientID       string  // APPLE_CLIENT_ID
AppleTeamID         string  // APPLE_TEAM_ID
AppleKeyID          string  // APPLE_KEY_ID
ApplePrivateKey     string  // APPLE_PRIVATE_KEY
DataDir             string  // DATA_DIR (default: /data)
DataRetentionDays   int     // DATA_RETENTION_DAYS (default: 30) — seeds Settings.retention_period_days on first startup only
AutomatedTasksRunAt string  // AUTOMATED_TASKS_RUN_AT (default: "08:00") — daily job run time (HH:MM, 24-hour, server local time)
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
   `--calendarUsername`, `--calendarPassword`, `--dummy`.
   SMTP and auth config must come from env vars only (they carry secrets).
   Keep `--calendarUrl` (repurposed as a seed value for `Settings.CalendarURL`).

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
Staff, Service, Availability, Contact, BookingSession, Booking,
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
   (`services`, `availability`, `contacts`, `sessions`, `bookings`, `settings`,
   `hmacSecret`), all covered by the existing `sync.RWMutex`.
2. `CreateBooking` is the critical method: it must hold the write lock for the
   full duration of the overlap check and the insert, preventing any concurrent
   goroutine from inserting a conflicting booking between the check and the write.
3. Add the two per-staff CalDAV calendars (`availability`, `bookings`) to the
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
2. **Availability write path**: when a `PUT` arrives on a path underneath
   `.../availability/`, parse the iCal body and call
   `DomainStore.UpsertAvailability`. After the write, call
   `DomainStore.ListActiveBookingsInWindow` for the old and new time window; if
   conflicts exist, call `email.SendAdminConflictNotification`. Do **not** forward
   to the inner `go-webdav` handler for availability writes; the store is the source
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

1. Add `availabilityToObject(a *store.Availability) *ical.Component` — synthesises a
   VEVENT representing an availability window for the Availability-Calendar.
2. Add `objectToAvailability(obj *ical.Component) (*store.Availability, error)` — parses
   a VEVENT from an Apple Calendar `PUT` into a `store.Availability` (reads `RRULE`,
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
10. **Billing service** — `internal/billing` (invoice generation, file write, staff e-mail).
11. **Retention job** — `internal/retention` (combined billing + retention background goroutine).
12. **REST handlers** — customer, then admin, then auth. Each wired into router.
13. **CalDAV handler refactor** — translate writes to `DomainStore`; update discovery
    for per-staff paths. iOS-interoperability tests must continue to pass.
14. **PostgreSQL backend** — `internal/store/postgres` with migrations.
    `store.NewBackend` now returns `PostgresStore` when `STORE_BACKEND=postgres`.
15. **Frontend** — SPA booking flow + admin UI in `web/`.
16. **OpenAPI spec** — fill out `api/openapi.yaml` to cover all endpoints.
17. **Integration tests** — end-to-end tests using `MemoryStore` backend.

---

## 18. Customer Dashboard

The **Customer Dashboard** is the self-service page a customer reaches by clicking a management link from any customer-facing e-mail (booking confirmation, session result, change-summary, or reminder). No login, account, or session cookie is required; access is granted entirely by the cryptographic token embedded in the URL.

### 18.1 URL format and SPA activation

Management links use the following URL pattern, served by the same `index.html` as the booking SPA:

```text
/?id=<bookingID>&token=<HMAC-SHA256-signature>
```

- **`id`** — the plain booking ID, used by the frontend to construct the `GET /api/v1/bookings/{id}` request URL.
- **`token`** — HMAC-SHA256 signature of the booking ID computed with the server-side secret (see §11).

When the customer browser loads `/`, the `<x-booking-app>` component inspects `window.location.search`. If both `?id=` and `?token=` are present the component enters **management mode**: the five-step booking flow is replaced by `<x-booking-manager>`, which fetches and displays the single booking identified by `id`.

### 18.2 Component hierarchy

```text
<x-booking-app>            ← detects management mode from URL; stepper not rendered
  └── <x-booking-manager>     ← fetches GET /api/v1/bookings/{id}?token=
        ├── <x-booking-card>         ← displays booking details + action buttons
        ├── <x-reschedule-picker>    ← inline slot picker for rescheduling
        └── <x-cancel-confirm>       ← cancellation confirmation dialog
```

All components live in `web/js/manage/`. Full component specifications are in §6 of `doc/userinterface.md`.

### 18.3 Backend handler

The Customer Dashboard API is served by `BookingHandler` in `internal/handlers/customer/booking.go`. Every endpoint requires a valid `?token=` query parameter (see §18.4).

| Method | Path | Action |
| --- | --- | --- |
| `GET` | `/api/v1/bookings/{id}` | Return booking details (service, times, status, contact) |
| `POST` | `/api/v1/bookings/{id}/reschedule` | Move the booking to a new slot |
| `DELETE` | `/api/v1/bookings/{id}` | Cancel the booking |
| `POST` | `/api/v1/bookings/{id}/new-session` | Start a new booking session with contact pre-filled |

### 18.4 Token verification

Every `BookingHandler` method calls `token.Signer.Verify(bookingID, tokenStr)` before performing any domain action:

1. Decodes the base64url token string.
2. Recomputes `HMAC-SHA256(secret, bookingID)` using the stored secret.
3. Compares the provided signature against the computed one with `hmac.Equal` (constant-time comparison).
4. Returns `ErrForbidden` on any mismatch; the handler responds `403 Forbidden`.

No session cookie is read or written on the customer-dashboard path.

### 18.5 State-dependent available actions

The frontend hides or disables actions that are not applicable to the current booking state. The backend enforces the same restrictions independently.

| Booking state | Reschedule | Cancel | Add another booking |
| --- | --- | --- | --- |
| `reserved` | yes | yes | yes |
| `confirmed` | yes | yes | yes |
| `cancelled` | — | — | yes |
| `no-show` | — | — | yes |

Attempting to reschedule or cancel a booking in `cancelled` or `no-show` state returns `409 Conflict`.

### 18.6 What the Customer Dashboard displays

The `GET /api/v1/bookings/{id}` response provides all fields rendered by `<x-booking-card>`:

| Field | Source |
| --- | --- |
| Service name | `service.name` |
| Start / end time | `booking.start_at` / `booking.end_at` |
| Location | `settings.appointment_location` |
| Status | `booking.state` |
| Customer name | `contact.first_name + " " + contact.last_name` |
| Customer e-mail | `contact.email` |
| Customer phone | `contact.phone` |
