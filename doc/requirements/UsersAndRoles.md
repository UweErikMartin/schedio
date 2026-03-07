# Users and Roles

schedio has three roles. Two of them — **Staff** and **Administrator** — require a named user account; the third — **Public** — covers all unauthenticated visitors.

---

## Role Summary

| Role | Authentication | Primary responsibility |
| --- | --- | --- |
| **Public** | None | Books appointments; manages own bookings via HMAC-signed management links. |
| **Staff** | Password (+ optional Apple Sign-In) | Reviews and confirms/rejects booking sessions; manages availability via CalDAV; receives data-retention and billing notifications. |
| **Administrator** | Password (+ optional Apple Sign-In) | Manages the service catalogue and deployment-wide settings. Has no access to individual public user bookings. |

---

## Public User

Public users are persons requesting an appointment for a service — no account or login is needed at any point.

- Book appointments through the multi-step booking flow.
- Manage their own bookings (reschedule, cancel, add a new booking) via individual management links included in every public user e-mail.
- A management link encodes the booking ID and an HMAC-SHA256 signature. The server validates the signature on every request; no session or cookie is stored.

---

## Staff

Staff users handle day-to-day operations:

- **Session review** — Confirm or reject individual bookings in each public user session via the admin session review page, reached by clicking the URL embedded in a CalDAV event.
- **Availability management** — Maintain the Availability-Calendar in a CalDAV client (e.g. Apple Calendar) by creating, modifying, and deleting availability events.
- **Data retention** — Receive notification e-mails when public user data reaches the retention deadline; approve deletions via a signed confirmation link or from the pending-deletion list in the admin interface.
- **Billing** — Receive an invoice e-mail when a public user's last appointment completes.

### CalDAV access

The Staff user connects a CalDAV client (such as Apple Calendar) directly to the schedio `/caldav/` endpoint using HTTP Basic authentication with their e-mail address and password. Two calendars are visible:

| Calendar | Purpose |
| --- | --- |
| **Availability-Calendar** | Staff-managed availability windows (always named `"Availability-Calendar"`). |
| **Booking-Calendar** | Read-only view of active public user bookings (display name configurable in General Settings). |

---

## Administrator

Administrators configure schedio for the deployment:

- Manage the **service catalogue** (add, edit, delete services).
- Configure **global settings**: Terms & Conditions PDF, no-show deadline, currency, appointment location, data retention period, reminder lead time, sender name, CalDAV calendar names, CalDAV server URL, and the HMAC signing secret.
- No access to individual public user records or bookings.

---

## User Account Management

User accounts are **not** managed through the web interface. They are defined in a YAML configuration file loaded once at startup.

**Default path:** `/etc/schedio/users.yaml` (overridden via `USERS_CONFIG_FILE` environment variable). In Kubernetes the file is mounted from a ConfigMap or Secret.

### Required fields per user

| Field | Required | Description |
| --- | --- | --- |
| `email` | Yes | Login name and address for system-generated e-mails sent to this user. |
| `password_hash` | Yes | bcrypt-hashed password. Used for password login and HTTP Basic auth on the CalDAV endpoint (Staff only). |
| `role` | Yes | `staff` or `administrator`. |
| `apple_oauth_enabled` | No (default `false`) | Enables Apple Sign-In for this user when Apple Sign-In is globally configured. |
| `apple_subject` | When `apple_oauth_enabled: true` | Apple `sub` claim used to match the authenticated Apple identity to this user entry. |

### Example

```yaml
users:
  - email: staff@example.de
    password_hash: "$2a$12$..."
    role: staff
    apple_oauth_enabled: true
    apple_subject: "001234.abcdef..."
  - email: admin@example.de
    password_hash: "$2a$12$..."
    role: administrator
```

---

## Authentication Methods

### Password login

Available to both Staff and Administrator. The password is checked against the bcrypt hash stored in the config file.

### Apple Sign-In (optional)

Apple Sign-In is an optional, per-user addition to password login. It requires four environment variables to be set at deployment time:

| Variable | Description |
| --- | --- |
| `APPLE_CLIENT_ID` | Apple Services ID |
| `APPLE_TEAM_ID` | Apple Developer Team ID |
| `APPLE_KEY_ID` | Key identifier for the Sign-In private key |
| `APPLE_PRIVATE_KEY` | PEM-encoded ECDSA private key |

Apple Sign-In is only available to users who have `apple_oauth_enabled: true` in the config file. On successful Apple authentication the server matches the Apple `sub` claim against the user's `apple_subject` field.

### HTTP Basic (CalDAV endpoint)

Used exclusively by the Staff role when a CalDAV client connects to `/caldav/`. Credentials are the staff user's `email` and cleartext password, verified against the bcrypt hash in the config file.
