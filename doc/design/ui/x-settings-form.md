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

| Field | Type | Description |
| --- | --- | --- |
| Unternehmensname | `text` | Business name shown in e-mail templates |
| Kontakt-E-Mail | `email` | Reply-to address for outgoing e-mails |
| Puffer zwischen Terminen (min) | `number` | Minutes of buffer between consecutive bookings |
| Maximale Buchungen pro Tag | `number` | Cap on daily bookings (0 = unlimited) |
| Zeitzone | `select` | IANA timezone name |

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
