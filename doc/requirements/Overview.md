# Overview: Application Functionality

schedio is a web-based appointment booking service deployed as a single container. It serves three groups of users — public users (public), staff, and administrators — each through a purpose-built interface.

---

## Public user Booking Flow

A public user opens the booking page in any browser (smartphone, tablet, or desktop) without creating an account.

### Step 1 — Choose a service

The public user picks a service from the catalogue. This starts a **booking session**, which groups all appointments made in one interaction. Selecting a different service discards the current session and starts a new one.

### Step 2 — Select date and time

The public user sees a list of booking lines. The first line is added automatically and pre-filled with the earliest available slot that fits the service duration. The public user can:

- Change the date and time of any line (only genuinely free slots are shown).
- Add more booking lines for additional appointments of the same service.
- Remove any line (except the last remaining one).

### Step 3 — Enter contact details

The public user provides name, e-mail address, and telephone number. All fields are required. These details are stored as a shared **Contact** entity and linked to every booking in the session.

### Step 4 — Review and confirm

A summary of the chosen service, all booking lines, and the entered contact details is shown. The public user must accept the Terms and Conditions (PDF link provided) before submitting. Navigating back is always possible to adjust any earlier step.

### Step 5 — Booking confirmed (reserved)

After submission the public user receives a confirmation e-mail containing:

- All booked appointments (service, date, time).
- An **individual management link** per booking for self-service changes.
- An `.ics` calendar attachment per booking.
- A notice that appointments are *reserved* but still await staff review.

---

## Public user Self-Service (Public user Dashboard)

Each management link in public user e-mails opens the **Public user Dashboard** for that specific booking — no login required. Access is protected by an HMAC-signed token in the URL:

```text
/?id=<bookingID>&token=<HMAC-SHA256-signature>
```

The dashboard shows: service, date and time, duration, location, current state, and the public user's contact details.

| Action | Available when booking state is … |
| --- | --- |
| **Reschedule** | `reserved` or `confirmed` |
| **Cancel** | `reserved` or `confirmed` |
| **Add another booking** | any state |

- **Reschedule** — Pick a new slot; receive an updated change-summary e-mail with a fresh `.ics` attachment.
- **Cancel** — Cancels the booking; if requested after the no-show deadline the state becomes `no-show` instead. A cancellation e-mail is sent in either case.
- **Add another booking** — Starts a new session for the same service with contact data pre-filled.

---

## Staff Operations

Staff users log in to the web interface and also connect their CalDAV client (e.g. Apple Calendar) directly to the schedio CalDAV endpoint.

### Session review and confirmation

New bookings appear as *tentative* events in Apple Calendar. The staff user clicks the URL in the event to open the session review page, where each booking in the session is confirmed or rejected individually. Once all bookings are decided, the public user receives a **session result e-mail** summarising which were confirmed and which were rejected.

### Availability management

The staff user maintains a dedicated **Availability-Calendar** in their CalDAV client:

- Create events (including recurring events) to define when appointments can be booked.
- Modify or delete events at any time.
- If a change conflicts with existing active bookings, schedio sends the staff user a notification e-mail listing all affected bookings and public user contacts.

Each availability event reflects its current occupancy in the calendar view: free windows appear as *transparent* ("Free"); booked windows show the public user name and service and appear *opaque*.

### Staff dashboard

- **Bookings of the Day** — Chronological list of all reserved or confirmed appointments for today.
- **Pending Confirmations** — Booking sessions awaiting review, oldest first.
- **Data Deletion** — Public user records that have reached the retention deadline and are pending manual deletion.

### Data retention

When the configured retention period (default 30 days) has passed since a public user's last appointment, schedio notifies all staff by e-mail and provides a signed deletion-confirmation link (valid 7 days). Clicking the link permanently deletes the public user's contact and booking records. If the link expires, the record moves to the pending-deletion list where staff can delete it manually on demand.

### Billing

When a public user's last appointment is completed, schedio automatically generates an invoice (all bookings with dates, services, and prices; total) and e-mails it to all staff. The invoice is also stored as a plain-text file in the configured data directory.

---

## Administrator Operations

Administrators log in to the web interface and manage configuration — they have no access to individual public user bookings.

### Service catalogue

- **Add / edit / delete services** — Each service has a name, summary, description, price, duration, and an optional daily booking limit. Services with active bookings cannot be deleted.

### General settings

| Setting | Purpose |
| --- | --- |
| Terms & Conditions PDF | Uploaded PDF public users must accept before confirming a booking. |
| No-show deadline | Hours before appointment after which a public user cancellation becomes a no-show (default 24 h). |
| Currency | ISO 4217 code used for price display (e.g. `EUR`). |
| Appointment Location | Address or URL included in `.ics` attachments. |
| Management Link Secret | HMAC secret for signing management links; rotating it invalidates all existing links. |
| Data Retention Period | Days to keep public user data after the last appointment (default 30). |
| Reminder Lead Time | Days before an appointment when a reminder e-mail is sent (default 1). |
| Sender Name | Display name in the `From:` header of public user e-mails. |
| Booking-Calendar display name | Name shown for the staff Booking-Calendar in CalDAV clients. |
| CalDAV Server URL | Hostname/URL advertised to CalDAV clients. |

---

## Automated Tasks

schedio runs the following tasks automatically once per day at a configurable time (`AUTOMATED_TASKS_RUN_AT`, default `08:00` local server time):

- **Reminder e-mails** — Sends a reminder to public users with a confirmed booking exactly *n* days in the future (where *n* is the Reminder Lead Time). Each booking receives at most one reminder.
- **Retention checks** — Identifies public users whose retention deadline has passed and triggers the notification / confirmation-link workflow described above.
