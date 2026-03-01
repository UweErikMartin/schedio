# Component spec: `<x-dashboard-today>`

> **Output file:** `web/js/admin/dashboard-today.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md),
> [`x-spinner.md`](x-spinner.md), [`x-error-banner.md`](x-error-banner.md)

---

## Element name

`x-dashboard-today`

## Responsibility

Fetches and lists today's bookings with state `reserved` or `confirmed`.
Auto-refreshes on a short polling interval.

---

## Observed attributes

None.

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `GET` | `/api/v1/admin/bookings?date=<today>&state=reserved,confirmed` | On mount and on each poll |

---

## Internal states

| State | Description |
| --- | --- |
| `loading` | Initial fetch; spinner shown. |
| `error` | Fetch failed; error banner with retry. |
| `empty` | No bookings for today; "Heute keine Termine" message. |
| `list` | One row per booking, sorted by start time ascending. |

---

## Displayed columns (table or card list)

| Column | Source |
| --- | --- |
| Zeit (time) | Earliest slot start, formatted as `HH:mm` |
| Kunde | `booking.contact.first_name + ' ' + booking.contact.last_name` |
| Dienst | `booking.service.name` |
| Status | `booking.state` badge |

---

## Testable behaviours

1. On mount, fetches today's bookings and renders them sorted by slot start time.
2. Shows `<x-spinner>` during the initial fetch only.
3. Polls at 60-second intervals while mounted; cancels polling on `disconnectedCallback`.
4. Only `reserved` and `confirmed` bookings are shown; `cancelled` and `no-show` are excluded.
5. If the fetch returns `[]`, shows "Heute keine Termine".
6. On fetch error, shows `<x-error-banner>` with retry; polling resumes after retry succeeds.

---

## Minimal usage example

```html
<x-dashboard-today></x-dashboard-today>
```
