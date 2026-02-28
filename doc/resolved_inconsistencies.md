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
