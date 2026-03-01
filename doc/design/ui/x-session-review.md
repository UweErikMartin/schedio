# Component spec: `<x-session-review>`

> **Output file:** `web/js/admin/session-review.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md),
> [`x-session-summary.md`](x-session-summary.md), [`x-spinner.md`](x-spinner.md),
> [`x-error-banner.md`](x-error-banner.md)

---

## Element name

`x-session-review`

## Responsibility

Lets an admin review a pending booking session, inspect its details, and either
confirm or reject it.

---

## Observed attributes

None. Session data is set via JS property.

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `session` | `Session` | The session to review. Triggers a re-render when set. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `review-done` | `{ action: 'confirmed' \| 'rejected' }` | Admin confirmed or rejected the session; API call succeeded. |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `POST` | `/api/v1/admin/sessions/{id}/confirm` | Admin clicks "Bestätigen" |
| `POST` | `/api/v1/admin/sessions/{id}/reject` | Admin clicks "Ablehnen" |

---

## Internal states

| State | Description |
| --- | --- |
| `idle` | Session details shown; action buttons active. |
| `submitting` | API call in flight; buttons replaced by `<x-spinner>`. |
| `error` | Action failed; error banner shown; buttons re-enabled. |

---

## Testable behaviours

1. Renders `<x-session-summary>` with `session` data in read-only mode.
2. "Bestätigen" and "Ablehnen" buttons are visible and enabled in `idle` state.
3. Clicking "Bestätigen" calls `POST .../confirm`; on success dispatches `review-done` with `{ action: 'confirmed' }`.
4. Clicking "Ablehnen" calls `POST .../reject`; on success dispatches `review-done` with `{ action: 'rejected' }`.
5. During the call, both buttons are disabled and a spinner is shown.
6. On API error, shows `<x-error-banner>` and re-enables the buttons.
7. An optional `note` textarea allows the admin to add a free-text note sent in the request body `{ "note": string }`.

---

## Minimal usage example

```js
const rev = document.createElement('x-session-review');
rev.session = pendingSession;
document.body.appendChild(rev);
```
