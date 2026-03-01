# Component spec: `<x-admin-dashboard>`

> **Output file:** `web/js/admin/admin-dashboard.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`x-dashboard-today.md`](x-dashboard-today.md),
> [`x-dashboard-pending.md`](x-dashboard-pending.md), [`x-session-review.md`](x-session-review.md)

---

## Element name

`x-admin-dashboard`

## Responsibility

Dashboard shell that renders the "today's bookings" panel and the "pending
sessions" panel side-by-side. Hosts the session-review overlay when the admin
opens a pending session.

---

## Observed attributes

None.

---

## Internal states

| State | Description |
| --- | --- |
| `overview` | Both panels shown; no session detail open. |
| `reviewing` | `<x-session-review>` shown on top (modal or side panel). |

---

## Testable behaviours

1. Renders `<x-dashboard-today>` and `<x-dashboard-pending>` on mount.
2. When `<x-dashboard-pending>` emits `session-review-requested`, transitions to `reviewing` and renders `<x-session-review>` with the session data.
3. When `<x-session-review>` emits `review-done`, returns to `overview` and triggers a refresh of both panels.
4. On desktop (≥ 1024 px) the two panels appear side-by-side; on mobile they stack vertically.

---

## Minimal usage example

```html
<x-admin-dashboard></x-admin-dashboard>
```
