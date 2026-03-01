# Component spec: `<x-dashboard-pending>`

> **Output file:** `web/js/admin/dashboard-pending.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md),
> [`x-spinner.md`](x-spinner.md), [`x-error-banner.md`](x-error-banner.md)

---

## Element name

`x-dashboard-pending`

## Responsibility

Fetches and lists booking sessions awaiting admin review (state `pending`).
Supports navigation to the session-review flow.

---

## Observed attributes

None.

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `session-review-requested` | `{ session: Session }` | Admin clicks "Prüfen" on a pending session row. |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `GET` | `/api/v1/admin/sessions?state=pending` | On mount and after a review completes (call `refresh()`) |

---

## Internal states

| State | Description |
| --- | --- |
| `loading` | Initial fetch; spinner shown. |
| `error` | Fetch failed; error banner with retry. |
| `empty` | No pending sessions; "Keine ausstehenden Anfragen" message. |
| `list` | Table of pending sessions. |

---

## Displayed columns

| Column | Source |
| --- | --- |
| Frühester Wunschtermin | Earliest slot across all booking lines, formatted as German date + time |
| Dienst | `session.service.name` |
| Kunde | Contact name |
| Eingegangen | `session.created_at` formatted as `dd.MM.yyyy HH:mm` |
| Aktionen | "Prüfen" button |

---

## Public methods

| Method | Description |
| --- | --- |
| `refresh()` | Triggers a new fetch to update the list after an external change. |

---

## Testable behaviours

1. On mount, fetches pending sessions and renders them sorted by earliest slot ascending.
2. Shows `<x-spinner>` during the initial fetch only.
3. Clicking "Prüfen" dispatches `session-review-requested` with the selected session object.
4. If the fetch returns `[]`, shows "Keine ausstehenden Anfragen".
5. `refresh()` triggers a new fetch and re-renders the list.
6. On fetch error, shows `<x-error-banner>` with retry.

---

## Minimal usage example

```html
<x-dashboard-pending></x-dashboard-pending>
```
