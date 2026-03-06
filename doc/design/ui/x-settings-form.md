# Component spec: `<x-settings-form>`

> **Output file:** `web/js/admin/settings-form.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`x-spinner.md`](x-spinner.md),
> [`x-error-banner.md`](x-error-banner.md), [`x-toast.md`](x-toast.md)

---

## Element name

`x-settings-form`

## Responsibility

Form for editing global application settings (business name, contact e-mail,
slot buffer, etc.). Fetches current values on mount and persists changes via
the admin API.

---

## Observed attributes

None.

---

## Form fields (representative; exact set comes from API schema)

| Field | API key | Type | Description |
| --- | --- | --- | --- |
| Kalender-URL | `calendar_url` | `text` | Hostname or URL of the CalDAV server (e.g. `caldav.example.com`). Seeded from `--calendarUrl` startup flag; admin UI value overrides immediately. |
| Kalenderbezeichnung | `default_calendar_name` | `text` | Display name of the default CalDAV calendar in CalDAV clients. Falls back to "Booking-Calendar" when empty. |
| Absender-Name | `sender_name` | `text` | Display name in the `From:` header of all customer-facing e-mails. |
| Terminsort | `appointment_location` | `text` | Physical address or video-call URL included as the ICS `LOCATION` field. |
| Währung | `currency` | `text` | ISO 4217 currency code, e.g. `EUR`. |
| No-Show-Frist (Stunden) | `no_show_deadline_hours` | `number` | Hours after appointment start after which a cancellation counts as a no-show. |
| Datenspeicherung (Tage) | `retention_period_days` | `number` | Days to retain contact and booking records after the last appointment. |
| Erinnerung im Voraus (Tage) | `reminder_lead_time_days` | `number` | Days before an appointment at which the reminder e-mail is sent. |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `GET` | `/api/v1/admin/settings` | On `connectedCallback` |
| `PUT` | `/api/v1/admin/settings` | Admin submits the form |

---

## Testable behaviours

1. Fetches current settings on mount and populates all form fields.
2. Shows `<x-spinner>` during the initial fetch.
3. On fetch error, shows `<x-error-banner>`.
4. Submitting with invalid fields shows HTML5 validation messages without calling the API.
5. On successful save, shows a `<x-toast>` with "Einstellungen gespeichert".
6. On save error, shows `<x-error-banner>`.
7. During save, the submit button is disabled and shows a spinner.

---

## Minimal usage example

```html
<x-settings-form></x-settings-form>
```
