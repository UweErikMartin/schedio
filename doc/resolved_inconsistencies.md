# Resolved Inconsistencies and Specifications

All items below were identified during requirements review and have been fully resolved.
The resolutions are incorporated into `requirements.md`.

---

## Inconsistencies (Resolved)

- **[I1]** — Each timeslot is an independent booking record. The customer builds a list of bookings line by line during step 2; all lines share the same service and contact data (via the Contact entity) submitted in one session. See step 2 for details.
- **[I1a]** — One combined confirmation e-mail is sent per session. The e-mail lists all bookings of the session, each with its own individual management link and a separate `.ics` attachment.
- **[I1b]** *(updated by NI2, NI9)* — The admin reviews each booking individually via the schedio admin session review page (reached via the `URL` embedded in the CalDAV event). Once the admin has completed the review of the full session, schedio sends the customer one session result e-mail listing all outcomes (confirmed and rejected bookings).
- **[I1c]** — A booking session is a first-class data model entity. It groups the individual booking records created in one customer interaction and persists across page navigations, allowing the customer to freely add or remove bookings before final submission. After submission the session is closed; the individual booking records continue independently.
- **[I2]** — There is no separate "correct" flow. The customer can only **change** a booking via the management link. Input validation during the booking process is limited to contact data only: name must not be empty, e-mail must have valid syntax, telephone number must match a valid scheme. Timeslot selection is not validated beyond availability.
- **[I3]** — After a booking change a separate change-summary e-mail is sent. It is not the admin-confirmation mail. The e-mail contains a single `.ics` attachment with all of the customer's bookings (from the same session) as individual `VEVENT` components, allowing the customer's calendar to update all entries in one import. Step 7 updated accordingly.
- **[I4]** — A signed token embedded in the URL is used. The token encodes the booking ID and carries an HMAC-SHA256 signature verified server-side. URL length impact is negligible (~80–130 chars for the token, ~200 chars total URL). Step 8 updated accordingly.
- **[NI1]** — Contact data is stored as a first-class **Contact** entity. Each booking record holds a reference to the Contact rather than embedding a copy. This ensures contact data is available per booking after session dissolution and enables future identification of a customer across sessions by e-mail address.
- **[NI2]** — schedio does not poll or hook into CalDAV status changes for confirmation detection. Instead, each CalDAV event carries a `URL` property pointing to a schedio admin page for that session. The admin navigates there from Apple Calendar and confirms bookings via the schedio web UI. The session result e-mail is sent by schedio once the admin has completed the full session review.
- **[NI3]** — Each per-booking management link is scoped to exactly one booking. Following the link opens a single-booking management page. Step 7 updated accordingly.
- **[NI4]** — A session is tied to exactly one service. Changing the service discards the current session and all its booking lines; a new session for the selected service is started. Step 1 updated accordingly.
- **[NI5]** — Telephone number accepts both international format (e.g. `+49 89 12345678`) and local format. German-only restriction was removed. Step 3 and M6 updated accordingly.
- **[NI6]** — Dual-backend persistence model adopted: when PostgreSQL is configured it is the single source of truth and CalDAV is a read/write facade; when no PostgreSQL is configured the `CalendarStore` (memory) is the persistence layer and CalDAV is the primary interface. NFR-2 (`STORE_BACKEND`) and M19 updated accordingly.
- **[NI7]** — Step 7 updated to describe all three operations available on the single-booking management page: reschedule, cancel, and add another timeslot.
- **[NI8]** — Edit a service description updated to include daily booking limit as an editable field.
- **[NI9]** — After the admin reviews all bookings in a session and decides on each (confirm or reject), schedio sends the customer one session result e-mail summarising all outcomes. Step 6 and M12 updated accordingly.

---

## Missing Specifications (Resolved)

- **[M1]** — A service consists of: **name** (string), **description** (string), **price** (decimal + currency), **duration** (integer, minutes), and **daily booking limit** (integer; `0` = unlimited). Services are managed via a dedicated admin-only service administration page supporting add, edit, and delete. Deletion is blocked if active bookings reference the service.
- **[M2]** — The administrator defines availability via a dedicated service-agnostic **Timeslots** CalDAV calendar managed in Apple Calendar. schedio finds bookable start times by selecting availability windows whose duration covers the selected service's duration and which are not already occupied by an active booking.
- **[M3]** — There is no per-slot capacity greater than 1. Double-booking prevention must be implemented at the store layer via an atomic critical section: a database transaction with row-level lock for the PostgreSQL backend; an in-process mutex for the memory backend (debugging only). CalDAV ETags additionally protect against concurrent edits of an already-created booking record via `ErrConflict`.
- **[M4]** — Five booking states: **Reserved** (`STATUS:TENTATIVE`), **Confirmed** (`STATUS:CONFIRMED`), **Cancelled by customer**, **No-show**, **Cancelled by administrator** (last three share `STATUS:CANCELLED`; originator and timing tracked in an additional field).
- **[M5]** — No-show deadline: configurable in hours; default **24 hours**. Cancellations made after the deadline are treated as no-shows. The slot is freed only for cancellations made before the deadline.
- **[M6]** — Telephone number is mandatory. Both international format (e.g. `+49 89 12345678`) and local format are accepted.
- **[M7]** — T&C is a PDF uploaded by the admin, stored in a directory configured via `DATA_DIR` (Kubernetes: PVC mounted at `DATA_DIR`). Customers must check an acceptance checkbox before submitting.
- **[M8]** — SMTP email; env vars: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM_ADDRESS`, `SMTP_FROM_NAME` (injected from a Kubernetes Secret).
- **[M9]** — ICS `VEVENT` fields: `UID` (booking ID), `SUMMARY` (service name), `DTSTART`/`DTEND`, `ORGANIZER` (`ADMIN_EMAIL`), `LOCATION` (from General Settings), `DESCRIPTION`, `SEQUENCE` (starts at 0, incremented on each change).
- **[M10]** *(resolved via NI3)* — Each management link is scoped to exactly one booking.
- **[M11]** — Single-booking management page operations: **Reschedule** (updates booking + `SEQUENCE`, sends change-summary e-mail), **Cancel** (frees slot before no-show deadline; no-show after), **Add another timeslot** (creates a new independent session pre-filled with original contact data and service).
- **[M12]** — The admin reviews and decides on each booking individually on the session review page. After completing the full session review, schedio sends the customer one session result e-mail listing all outcomes.
- **[M13]** — Dashboard four sections: General Settings (T&C PDF, no-show deadline, currency, appointment location, management link secret), Services, Bookings of the Day, Pending Confirmations.
- **[M14]** — Admin auth: Sign in with Apple (env: `APPLE_CLIENT_ID`, `APPLE_TEAM_ID`, `APPLE_KEY_ID`, `APPLE_PRIVATE_KEY`, `APPLE_ALLOWED_SUBJECT`) + username/password (env: `ADMIN_USERNAME`, `ADMIN_PASSWORD_HASH`). Both methods may be enabled simultaneously.
- **[M15]** — Multi-staff support. Each staff member has individual Timeslots and Bookings CalDAV calendars. Staff configured via a YAML config file mounted into the container. Customers are unaware of individual staff members.
- **[M16]** — Times displayed in browser-reported timezone (`Intl.DateTimeFormat`). Stored in UTC internally. ICS uses UTC with `Z` suffix.
- **[M17]** — Admin notification e-mail sent on each new session submission, containing a booking summary and a link to the Pending Confirmations page.
- **[M18]** — Data retention: Contact and booking records deleted after `DATA_RETENTION_DAYS` days (default **30**) from the customer's most recent booking date. Background maintenance job inside the container.
- **[M19]** — Dual-backend persistence: PostgreSQL backend → PostgreSQL is source of truth, CalDAV is read/write facade; memory backend → CalendarStore is persistence layer, CalDAV reflects its state directly. CalDAV event statuses: `STATUS:TENTATIVE` (reserved), `STATUS:CONFIRMED` (confirmed), `STATUS:CANCELLED` (rejected/cancelled).
- **[M20]** — Submission conflict: reject the request, navigate customer back to step 2, auto-fill conflicting lines with next available slot, highlight them visually. Customer can accept, change, or remove.
- **[M21]** — HMAC-SHA256 signing secret auto-generated at startup if absent; persisted in the active store backend. Admin can download/upload it from General Settings page. Uploading a new secret immediately invalidates all previously issued management links.
- **[M22]** — Session review decisions are independent per booking; rejecting one does not affect others. Customer receives one session result e-mail once the admin completes the full session review.
- **[M23]** — Default `DATA_RETENTION_DAYS` = **30** days. M18 updated.
- **[M24]** — "Add another timeslot" from the management page creates a new independent session, pre-filled with the original contact data and service, and goes through the full booking flow including admin confirmation.
- **[M25]** — No-show state is set **manually by the administrator** via the admin UI (e.g. a button on the Bookings of the Day panel or the session detail page). No automatic background transition is performed. M4 updated accordingly.

---

## User and Role Management â€” Resolved Inconsistencies (NI10â€“NI16)

Opened 2026-03-01 after addition of requirements.md Â§4 User and Role Management.
All resolved 2026-03-01.

- **[NI10]** â€” Removed ADMIN_USERNAME and ADMIN_PASSWORD_HASH env vars.
  All users (staff and administrators) are defined in a YAML config file.
  rchitecture.md updated to reference USERS_CONFIG_FILE env var.
  internal/auth package loads the file at startup and calls DomainStore.SyncUsers.

- **[NI11]** â€” Staff entity renamed to User in rchitecture.md Â§6.1.
  Added fields: email TEXT NOT NULL UNIQUE (login identifier / CalDAV principal name),
  
ole TEXT NOT NULL (staff | dministrator),
  pple_oauth_enabled BOOLEAN NOT NULL DEFAULT FALSE, pple_subject TEXT.
  identifier field removed; email serves as the unique login identifier.
  All FK references updated from staff_id FK->Staff to user_id FK->User.

- **[NI12]** â€” Removed APPLE_ALLOWED_SUBJECT global env var.
  Apple Sign-In subject matching is now per-user via User.apple_subject field.
  rchitecture.md Â§9.2 updated accordingly.

- **[NI13]** â€” Added SyncUsers(ctx, users []*User) error and
  GetUserByEmail(ctx, email string) (*User, error) to DomainStore in
  rchitecture.md Â§5.2. User config loading in internal/auth calls
  SyncUsers at server startup before requests are served.

- **[NI14]** â€” Added Role column to rchitecture.md Â§7.3 admin route table.
  Dashboard and session-review routes require role staff.
  Services and settings routes require role dministrator.
  Auth middleware updated to RequireAuth + RequireRole(role) pattern.

- **[NI15]** â€" Removed pple-enabled boolean attribute from `<x-login-form>`.
  Login form now always renders both password input and a (disabled) "Anmelden
  mit Apple" button. On username field blur/Enter with a non-empty value, the
  form calls GET /auth/apple/available?username={encoded-email}; if the
  response is { "apple_enabled": true } the button is enabled.
  userinterface.md Â§8.4 updated with 14 testable behaviours.
  pi/openapi.yaml updated with GET /auth/apple/available endpoint.

- **[NI16]** â€” Added GET /auth/me endpoint returning { username, role } for
  the authenticated session. rchitecture.md Â§7.2 and pi/openapi.yaml
  updated. User schema added to pi/openapi.yaml components/schemas.

---

## Data Retention and Billing -- Resolved Inconsistencies (NI17-NI26)

Opened 2026-03-01 after addition of requirements.md Data Retention (SS5) and Billing (SS6).
All resolved 2026-03-01.

- **[NI17]** -- Retention flow redesigned from silent deletion to email-confirmation workflow.
  architecture.md SS12 replaced with four-pass daily goroutine: (1) billing pass,
  (2) retention-notify pass (sends email to all Staff with 7-day signed confirmation link),
  (3) confirmation-expiry pass (escalates to pending_deletion after 7 days),
  (4) no automatic deletions. DomainStore.DeleteExpiredContacts removed;
  replaced by NotifyRetentionDue, MarkRetentionNotified, ListConfirmationExpired,
  AddToPendingDeletion, ListPendingDeletion, DeleteContact, ListBillingDue,
  MarkBillingGenerated, ListBookingsForContact.

- **[NI18]** -- Added retention_state (TEXT NOT NULL DEFAULT 'active', values: active/notified/pending_deletion)
  and retention_notified_at (TIMESTAMPTZ) fields to the Contact entity in architecture.md SS6.1.

- **[NI19]** -- Added retention_period_days (INTEGER NOT NULL DEFAULT 30) to the Settings entity
  in architecture.md SS6.1. Added retention_period_days to Settings and SettingsInput schemas
  in api/openapi.yaml. DATA_RETENTION_DAYS env-var is now a seed value for first startup only.
  userinterface.md SS8.8.1 settings form updated to include "Aufbewahrungsfrist (Tage)" field.

- **[NI20]** -- Renamed last_booking_at -> last_appointment_end_at on Contact entity.
  Renamed store method UpdateContactLastBooking -> UpdateContactLastAppointment.
  Retention/billing trigger now uses appointment end time (start_at + duration) rather
  than booking creation time.

- **[NI21]** -- Added three retention API endpoints to architecture.md SS7.3 and api/openapi.yaml:
  GET /admin/api/v1/retention/pending (staff), DELETE /admin/api/v1/retention/pending/{contactID} (staff),
  GET /admin/api/v1/retention/confirm?token= (signed token, no auth cookie required; 410 on expiry).
  Added DeletionCandidate schema and admin-retention tag to openapi.yaml.

- **[NI22]** -- Added x-retention-list component to userinterface.md SS8.9 with 10 testable behaviours.
  Updated x-admin-nav to include "Datenloesung" link (active value: 'retention').
  Added 'retention' route to x-admin-app routing table.
  Added x-retention-list.js to the component file listing.

- **[NI23]** -- Replaced Contact.name (single field) with first_name and last_name fields throughout:
  architecture.md SS6.1 Contact entity updated.
  api/openapi.yaml Contact schema updated (first_name, last_name; example split).
  DeletionCandidate schema uses first_name/last_name.
  userinterface.md SS5.9 x-contact-form updated (Vorname/Nachname inputs, 4-field form).
  userinterface.md SS9.3 contact JSON shape updated.
  userinterface.md SS11.3 contact form grid updated.
  x-session-summary testable behaviour updated.

- **[NI24]** -- Added internal/billing package to architecture.md SS4 new-packages table.
  Invoice format resolved as plain text (.txt). Storage path: DATA_DIR/invoices/yyyy-mm-dd-LastName-FirstName.txt.
  SS12 documents billing.GenerateAndSend flow (file write + Staff email).
  architecture.md SS14.2 DATA_DIR description updated to mention invoices/ subdirectory.

- **[NI25]** -- Added retention-notify and billing-invoice email types to architecture.md SS10.2
  (template directories) and SS10.3 (email types table). Recipients for both: all Staff users.

- **[NI26]** -- Added billing_generated (BOOLEAN NOT NULL DEFAULT FALSE) to Contact entity.
  Defined restart rules: billing_generated and retention_state reset when last_appointment_end_at
  moves forward. Cancelled-booking rule documented: only non-cancelled bookings contribute to
  last_appointment_end_at. Background job billing pass uses billing_generated flag to avoid
  duplicate invoice generation.

---

## Automated Tasks / Reminder E-Mail -- Resolved Inconsistencies (NI27-NI30)

Opened 2026-03-01 after addition of `requirements.md` §2 (Reminder Lead Time setting)
and §5 (Automated Tasks -- Reminder E-Mail).
All resolved 2026-03-01.

- **[NI27]** -- Added `reminder_lead_time_days` (integer, minimum 1, example 1) to the
  `Settings` component schema in `api/openapi.yaml`, including it in the `required` array.
  Added the same field (without example) to the `SettingsInput` schema.

- **[NI28]** -- Updated `<x-settings-form>` in `doc/userinterface.md` §8.8.1:
  Component description extended to include "reminder lead time".
  Behaviour #2 updated to list "Erinnerungsfrist (Tage)" among rendered fields.
  New testable behaviour added: "Reminder lead time is a positive integer input
  (default 1); labelled 'Erinnerungsfrist (Tage)'."

- **[NI29]** -- Updated the Settings JSON shape in `doc/userinterface.md` §9.6 to
  include both `"retention_period_days": 30` (missing since NI19) and
  `"reminder_lead_time_days": 1` (new). Also corrected the mojibake in the
  `appointment_location` example value.

- **[NI30]** -- Updated `doc/architecture.md` in seven locations:
  (1) Component tree (§3): added `AUTOMATED_TASKS_RUN_AT` scheduling note to
      `retention.StartJob()` comment.
  (2) New packages table (§4): updated `internal/retention` description to include
      reminder pass (Pass 0) and reference `AUTOMATED_TASKS_RUN_AT`.
  (3) DomainStore interface (§5.2): added `ListBookingsDueReminder(ctx, leadDays int)`
      and `MarkReminderSent(ctx, bookingID string)` methods.
  (4) Booking entity (§6.1): added `reminded_at TIMESTAMPTZ` column (nullable;
      set when reminder e-mail is sent).
  (5) Settings entity (§6.1): added `reminder_lead_time_days INTEGER NOT NULL DEFAULT 1`.
  (6) Background job section (§12): renamed section to include "Reminders"; changed
      from "every 24 hours" to `AUTOMATED_TASKS_RUN_AT` scheduling; added Pass 0
      (Reminder e-mails) with full store-call and email-send specification; updated
      pass count from five to four (Passes 0--3).
  (7) Environment variables (§14): added `AUTOMATED_TASKS_RUN_AT` (default `08:00`,
      HH:MM 24-hour server local time).
  Additionally: added `reminder/` template directory to §10.2 and `reminder`
  email type/trigger/recipient row to §10.3 email types table.
  Added `AutomatedTasksRunAt string` field to the config struct in §17.2.

---

## Sender Name (Absender-Name) — Resolved Inconsistency (NI31)

Opened and resolved 2026-03-01 after user request to make the e-mail From
display name configurable in the admin settings UI.

- **[NI31]** -- Added `sender_name` / `SenderName` across all layers:
  - `requirements.md` §2 (General Settings): added **Absender-Name (Sender Name)**
    bullet explaining the default value, the startup flag, and the admin UI behaviour.
  - `doc/architecture.md` §6.1 Settings entity: added `sender_name TEXT NOT NULL DEFAULT 'Schedio Buchungssystem'`.
  - `api/openapi.yaml`: added `sender_name` to `Settings` schema (required) and
    `SettingsInput` schema (optional).
  - `doc/userinterface.md` §8.8.1: added "Absender-Name" to component description,
    field list, and a new testable behaviour. §9.6 JSON example updated.
  - `internal/store/model_domain.go`: added `SenderName string` (and `ReminderLeadTimeDays int`)
    to `Settings` struct.
  - `internal/store/memory.go`: seeded `SenderName: "Schedio Buchungssystem"` and
    `ReminderLeadTimeDays: 1` in `NewMemoryStore` defaults.
  - `internal/email/sender.go`: added `sync.RWMutex` + `fromNameOverride string` fields,
    `SetFromName(name string)` method, and `resolveFromName()` used in `send()`. The
    override takes effect immediately for all subsequent sends without a restart.
  - `internal/handlers/admin/settings.go`: implemented `SettingsHandler` with `Get`
    (GET /admin/api/v1/settings) and `Put` (PUT /admin/api/v1/settings) handlers.
    PUT propagates `sender_name` changes to the `email.Sender` via `SetFromName`.
  - `internal/server/router.go`: imports `admin` package; seeds `SenderName` in the
    store from `args.SenderName` (`--smtpSenderName`) at startup if not already set; synchronises the
    stored value into the email sender; registers GET and PUT routes for
    `/admin/api/v1/settings`.
