# Overview

## General Description

schedio is a single container, appointment booking app which is intended to be deployed in a Kubernetes cluster and offers
the following functionalities.

## Booking of Appointments for a Customer

A customer shall be able to open the booking web page on any web-capable device: smartphone, tablet, or web browser on a PC.

The booking process follows these steps:

1. **Service selection** — The customer selects a service offering from a list of available services. This selection starts a **booking session**. A booking session is a first-class concept in the data model: it groups all individual bookings created in one interaction and persists across page navigations so the customer can move between steps, add or remove bookings, and return without losing his selection. A session is always tied to exactly one service. If the customer navigates back and selects a different service, the existing session and all its booking lines are discarded and a new session for the newly selected service is started.
2. **Timeslot selection** — The timeslot selection is presented as a list of booking lines within the active session. Each line represents one independent booking of the selected service. Timeslots in the **Timeslots** CalDAV calendar (see *Timeslot Management*) represent the administrator's general availability and are service-agnostic. When presenting options to the customer, schedio finds availability windows in the Timeslots calendar that are long enough to accommodate the duration of the selected service and that are not already occupied by an active booking. The start of such a window is offered as a bookable slot:
   - The first line is added automatically and shows the next available start time that fits the service duration as the pre-selected value.
   - The customer can change the date and/or the time of that booking line by interacting with the date and time controls on the same line. Only start times that fall within a sufficiently long free availability window are selectable.
   - After the timeslot of a line is set, the customer can choose to add another booking of the same service. Doing so appends a new line pre-filled with the next available start time after the previously selected appointment ends.
   - Each additional line offers the same controls: change date, change time, and optionally add yet another booking.
   - Lines can be removed individually.
   - Each booking line results in one independent booking record. All records in a session hold a reference to a shared **Contact** entity (see step 3) and share the same service, but are otherwise treated as separate appointments throughout the rest of the flow. The session itself dissolves after the customer submits the final confirmation; from that point the individual booking records — each carrying its own reference to the Contact — stand on their own.
3. **Contact data entry** — The customer enters his contact details: name, e-mail address, and telephone number (for queries). These details are stored as a **Contact** entity in the data model. Each booking record created in the session holds a reference to this Contact entity rather than duplicating the data inline. This ensures that contact data is available independently per booking after the session is closed, and that a single customer can be identified across multiple sessions by his e-mail address. The following validation rules apply before the customer can proceed:
   - Name must not be empty.
   - E-mail address must not be empty and must conform to standard e-mail address syntax.
   - Telephone number must not be empty and must be a valid telephone number in international (`+<CountryCode> …`) or local format.
   No other automatic validation is performed; the correctness of the selected timeslots is the customer's responsibility.
4. **Booking overview and confirmation** — The customer is presented with a summary of his selection: the chosen service, all booking lines with their dates and times, and his contact details. He may navigate back to a previous step to change the service, adjust timeslots, or correct his contact data. He must accept the terms and conditions before submitting. There is no separate "correct" flow; all changes are handled by navigating back within the session.
5. **Confirmation e-mail (reserved)** — After confirmation, the customer receives one e-mail for the entire session containing:
   - A list of all bookings made in the session, each showing the service, date, and time.
   - An individual management link per booking, allowing the customer to view or change that specific appointment.
   - A calendar entry (`.ics` attachment) per booking for import into the customer's preferred calendar application.
   - A notice that all bookings are **reserved but not yet reviewed**, and that a session result e-mail summarising the admin's decisions will follow once the administrator has completed the session review.
6. **Admin session review and confirmation** — Each booking is stored as a CalDAV event with status *tentative* (`STATUS:TENTATIVE`). The CalDAV event's `URL` property contains a link to the schedio admin session review page. The administrator works with Apple Calendar (or any CalDAV-capable client) connected to the schedio CalDAV endpoint. New bookings appear as tentative events. The admin clicks the URL embedded in an event, which opens the session review page for that session. This page lists all individual bookings of the session; the admin confirms or rejects each booking individually. Once the admin has **completed the review** of the session (all bookings have been either confirmed or rejected), schedio sends the customer one **session result e-mail** summarising the outcome: confirmed bookings, rejected bookings, and the overall session status.
7. **Booking management via link** — Each management link in the confirmation e-mail is scoped to exactly one booking. Following a link opens a page showing only that specific appointment. The customer can perform the following actions on that page:
   - **Reschedule** — select a new date and time from the available slots for the same service. After saving, schedio sends the customer a **change-summary e-mail** containing a summary of the changed booking and a single `.ics` attachment with all of the customer's current bookings in the session as individual `VEVENT` components with updated `SEQUENCE` values, so the customer's calendar application can update all affected entries in one import.
   - **Cancel** — cancel the booking. If the cancellation is made before the no-show deadline the slot is freed; after the deadline the state transitions to *no-show*. A cancellation e-mail is sent to the customer in both cases.
   - **Add another timeslot** — start a new independent booking session for the same service and contact data, pre-filled and ready for timeslot selection (see M11 and M24).
8. **Link protection** — Each management link embeds a signed token directly in the URL (e.g. as a query parameter `?token=…`). The token encodes the booking ID and is signed with an HMAC-SHA256 signature using a server-side secret. The server validates the signature on every request; no session or cookie is required. The resulting token is approximately 80–130 characters, keeping the total URL well under 200 characters — safe for all browsers, e-mail clients, and intermediaries. A customer cannot access or modify another customer's booking because the token is cryptographically bound to a specific booking ID. The HMAC secret is generated automatically at startup if no secret is present in the store; it is persisted in the active store backend (database for PostgreSQL, in-memory for the debug backend). The administrator can download the current secret or upload a replacement secret via the General Settings admin page; uploading a new secret immediately invalidates all previously issued management links.

## Overview of Appointments for the Administrator

The administrator dashboard is the main entry point after login. It provides quick access to the following four areas:

- **General Settings** — Upload or replace the Terms and Conditions PDF; configure the no-show deadline, the currency, the appointment location, and the management link secret. See *General Settings* section.
- **Services** — Add, edit, and delete services. See *Service Administration* section.
- **Bookings of the Day** — A chronological list of all bookings (reserved and confirmed) whose appointment date is today. Displayed with the customer name, service, time, and current state. Provides the administrator with a quick operational view of the current day's schedule.
- **Pending Confirmations** — A list of all bookings in *reserved* state awaiting admin confirmation, sorted by submission time (oldest first). Each entry shows the customer name, service, requested date and time, and a link to the admin session page where the bookings of that session can be confirmed.

## Administrative Tasks

1. **Service Administration**

   The administrator manages the list of available services through a dedicated service administration page that is accessible only to authenticated administrators. The following operations are supported:

   - **Add a service** — The administrator can create a new service by providing:
     - **Name** (required) — a short human-readable label shown to customers in the service selection list.
     - **Description** (required) — a longer text describing the service, shown to the customer during selection.
     - **Price** (required) — the cost of the service as a decimal number with currency. A price of zero is permitted (free service).
     - **Duration** (required) — the length of one appointment for this service, expressed in minutes. The duration determines the length of timeslots generated for this service.
     - **Daily booking limit** (required) — the maximum number of bookings for this service that may be accepted per calendar day. A value of `0` means no restriction; any number of bookings per day is allowed. If the limit is greater than `0` and the daily count has been reached, this service is no longer selectable by customers for the remainder of that day. Other services whose daily limits have not yet been reached remain fully available for booking.
   - **Edit a service** — The administrator can change the name, description, price, duration, or daily booking limit of an existing service. Changes take effect for new bookings immediately; existing bookings are not retroactively altered.
   - **Delete a service** — The administrator can remove a service from the list. A service that has active (reserved or confirmed) bookings shall not be deleted; the administrator must cancel or complete those bookings first. A deleted service no longer appears in the customer-facing service selection list.

2. **Timeslot Management**

   Available timeslots are managed exclusively by the administrator via a dedicated CalDAV calendar named **Timeslots**. This calendar is served by the schedio CalDAV endpoint and is visible as a separate calendar in Apple Calendar (or any CalDAV-capable client). The Timeslots calendar represents the administrator's general availability and is **service-agnostic**: events in it do not specify which service can be booked; they only define when the administrator is available. schedio matches a customer's service selection against these availability windows at booking time.

   The administrator manages availability by adding, modifying, and deleting events in the Timeslots calendar directly in Apple Calendar:

   - **Add a timeslot** — The administrator creates a calendar event in the Timeslots calendar. The event's start time and duration define an availability window during which appointments can be booked. Any service whose duration fits within the window may be booked into it.
   - **Add recurring timeslots** — The administrator can create recurring events (daily, weekly, or custom recurrence rules) to define repeating availability without entering each slot individually. Individual occurrences of a recurring event can be modified or deleted independently without affecting the rest of the series.
   - **Modify a timeslot** — The administrator can change the start time, duration, or recurrence of an existing availability window. Existing bookings that fall within the original window are not automatically changed or cancelled. However, if the modification causes one or more active bookings to no longer fit within the revised window (conflict), schedio sends a notification e-mail to the administrator. This e-mail contains a list of all conflicting bookings, each with the booking date and time, the service booked, and the customer's contact data (name, e-mail address, telephone number), so that the administrator can contact the affected customers directly.
   - **Delete a timeslot** — The administrator can delete a single occurrence or an entire recurring series from the Timeslots calendar. Existing bookings that fall within the deleted window are not automatically cancelled. If the deletion affects one or more active bookings, schedio sends a notification e-mail to the administrator listing all affected bookings with their booking date and time, the service booked, and the customer's contact data (name, e-mail address, telephone number), so that the administrator can contact the affected customers directly.

   schedio determines bookable start times for a given service as follows:
   - Find all events in the Timeslots calendar whose duration is greater than or equal to the selected service's duration.
   - Within each such event, identify start-aligned positions that are not already occupied by an active booking of the same or greater duration.
   - Offer those positions as selectable start times to the customer.

3. **General Settings**

   The administrator can configure deployment-wide settings through a dedicated general settings page accessible only to authenticated administrators. The following settings are available:

   - **Terms and Conditions (PDF)** — The administrator uploads a PDF document containing the terms and conditions that customers must accept before confirming a booking. The uploaded PDF is stored by schedio and served to customers on the booking confirmation page (step 4) as a downloadable link. Customers must check an acceptance checkbox before they can submit. The administrator can replace the PDF at any time; the new version takes effect for all subsequent bookings immediately. Existing bookings are not affected.
   - **No-show deadline** — The number of hours before an appointment start time after which a customer cancellation is treated as a no-show. Default: 24 hours.
   - **Currency** — The ISO 4217 currency code used when displaying service prices (e.g. `EUR`, `USD`). This setting applies to all services and is stored in the database. It can be changed by the administrator at any time; the change takes effect immediately for all price displays.
   - **Appointment Location** — The physical or virtual location included in calendar invitations (ICS `LOCATION` field), e.g. a street address or a video-call URL. Stored in the database; changes take effect for all subsequently generated `.ics` files.
   - **Management Link Secret** — The HMAC-SHA256 secret used to sign customer management link tokens. schedio generates and stores this secret automatically at first startup. The administrator can download the current secret as a file (e.g. for backup or migration) and can upload a replacement secret to overwrite the stored one. Uploading a new secret immediately invalidates all previously issued management links.

---

## Open Points / TODO

All inconsistencies and missing specifications have been resolved. See:

- [resolved_inconsistencies.md](resolved_inconsistencies.md) — full resolution history for items I1–I4, NI1–NI9, and M1–M25.
- [open_inconsistencies.md](open_inconsistencies.md) — currently no open items.

---

## Non-Functional Requirements (NFR)

### NFR-1 — API Documentation

schedio exposes an OpenAPI 3.x specification for its HTTP API. The specification is embedded in the binary (via `go:embed`) and served at a dedicated endpoint. A Swagger UI page is provided at `/api/` so that developers and integrators can browse and interactively test all endpoints without any external tooling.

The OpenAPI document must cover:

- All customer-facing booking endpoints (service list, timeslot availability, session submission, management link operations).
- All admin-facing endpoints (service CRUD, settings, booking confirmation/rejection, dashboard data).
- Authentication schemes (cookie session for admin endpoints; HMAC-signed token for management-link endpoints).

### NFR-2 — Persistence

The primary persistence store is **PostgreSQL**. All domain entities (Services, Contacts, BookingSessions, Bookings, Staff) are stored in a PostgreSQL database. The connection is configured via the following environment variables:

| Variable | Description |
| --- | --- |
| `POSTGRES_HOST` | Database host name |
| `POSTGRES_PORT` | Database port (default `5432`) |
| `POSTGRES_DB` | Database name |
| `POSTGRES_USER` | Database user |
| `POSTGRES_PASSWORD` | Database password (injected from a Kubernetes Secret) |
| `POSTGRES_SSLMODE` | SSL mode (e.g. `require`, `disable`) |

The schema is managed by schedio itself via embedded migration scripts applied at startup. No external migration tool is required.

**Architecture: CalDAV as a facade** — PostgreSQL is the single source of truth for all domain data. The CalDAV endpoint (`/caldav/`) is a **read/write facade** that presents a computed view of the PostgreSQL data: the Timeslots and Bookings calendars are constructed on-the-fly from the database. All mutations — booking creation, confirmation, cancellation, timeslot changes — are performed through the REST API. When the CalDAV endpoint receives a write (e.g. a `PUT` or `DELETE` from Apple Calendar), schedio translates it into the corresponding REST/domain operation on the PostgreSQL store, rather than writing directly to a CalDAV-native store. This ensures PostgreSQL is always authoritative and the REST API (documented in OpenAPI) is the canonical interface for all operations.

**Store backend selection** — The active store backend is selected at startup via the environment variable `STORE_BACKEND`:

- `postgres` (default when `POSTGRES_HOST` is set) — uses the PostgreSQL database defined by the `POSTGRES_*` env vars. PostgreSQL is the single source of truth; the CalDAV endpoint is a read/write facade over it. This is the supported backend for production deployments.
- `memory` (default when no PostgreSQL is configured) — uses the in-process `CalendarStore` (memory store). The CalDAV endpoint is the primary persistence layer in this mode. All data is lost on restart. Intended for local development and debugging only; must not be used in production.
