# Component spec: `<x-booking-manager>`

> **Output file:** `web/js/manage/booking-manager.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md),
> [`x-booking-card.md`](x-booking-card.md), [`x-reschedule-picker.md`](x-reschedule-picker.md),
> [`x-cancel-confirm.md`](x-cancel-confirm.md), [`x-error-banner.md`](x-error-banner.md),
> [`x-spinner.md`](x-spinner.md)

---

## Element name

`x-booking-manager`

## Responsibility

Orchestrator for the customer-facing booking-management page, reached via the
management link (`/?id=<bookingID>&token=<signedToken>`). Fetches the booking,
hosts the reschedule and cancellation sub-flows.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `booking-id` | `string` | `""` | Booking ID extracted from `?id=` URL parameter. |
| `token` | `string` | `""` | HMAC-signed management token from `?token=` URL parameter. |

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `booking` | `Booking \| null` | Currently loaded booking data. Read-only from outside. |

---

## Internal states

| State | Description |
| --- | --- |
| `loading` | Fetching the booking; spinner displayed. |
| `error` | Fetch failed or token is invalid (HTTP 401/403/404); error banner shown with no retry option (link tampered). |
| `view` | Booking data displayed via `<x-booking-card>`. |
| `reschedule` | `<x-reschedule-picker>` displayed beneath the card. |
| `cancel-confirm` | `<x-cancel-confirm>` dialog is open. |
| `cancelled` | Booking was cancelled successfully; confirmation banner shown. |
| `success` | Rescheduling succeeded; confirmation banner shown. |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `GET` | `/api/v1/bookings/{booking-id}?token={token}` | On `connectedCallback` |

---

## Testable behaviours

1. On `connectedCallback` with valid `booking-id` and `token`, fetches the booking and transitions to `view`.
2. On HTTP 401/403/404, transitions to `error` with "Ungültiger oder abgelaufener Link" message.
3. On network failure, transitions to `error` with a retry button; clicking retry re-fetches.
4. In `view` state, renders `<x-booking-card>` with the loaded booking.
5. When `<x-booking-card>` emits `reschedule-requested`, transitions to `reschedule` and renders `<x-reschedule-picker>`.
6. When `<x-booking-card>` emits `cancel-requested`, transitions to `cancel-confirm` and opens `<x-cancel-confirm>`.
7. When `<x-reschedule-picker>` emits `reschedule-confirmed`, transitions to `success` banner.
8. When `<x-reschedule-picker>` emits `reschedule-cancelled`, returns to `view`.
9. When `<x-cancel-confirm>` emits `cancellation-confirmed`, transitions to `cancelled` banner.
10. When `<x-cancel-confirm>` emits `cancellation-cancelled`, returns to `view`.

---

## Minimal usage example

```html
<x-booking-manager booking-id="bk-abc123" token="hmac-signed-value"></x-booking-manager>
```
