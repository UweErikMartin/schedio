# Component spec: `<x-reschedule-picker>`

> **Output file:** `web/js/manage/reschedule-picker.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md),
> [`x-date-time-picker.md`](x-date-time-picker.md), [`x-spinner.md`](x-spinner.md),
> [`x-error-banner.md`](x-error-banner.md)

---

## Element name

`x-reschedule-picker`

## Responsibility

Inline date-time picker for rescheduling a booking. Calls the reschedule API
endpoint on confirmation.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `booking-id` | `string` | `""` | ID of the booking to reschedule. |
| `token` | `string` | `""` | HMAC management token authorising the operation. |
| `service-id` | `string` | `""` | Forwarded to `<x-date-time-picker>` as `service-id`. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `reschedule-confirmed` | `{ newSlot: string }` | The reschedule API call succeeded; `newSlot` is the ISO-8601 datetime. |
| `reschedule-cancelled` | `{}` | User clicks "Abbrechen". |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `POST` | `/api/v1/bookings/{booking-id}/reschedule?token={token}` | User clicks "Bestätigen" after selecting a slot |

Request body: `{ "new_slot": "<ISO-8601 datetime>" }`

---

## Testable behaviours

1. Renders an `<x-date-time-picker>` seeded with `service-id` and `min-date` = today.
2. The "Bestätigen" button is disabled until the picker emits `date-time-selected`.
3. Clicking "Bestätigen" calls the reschedule endpoint.
4. On success, dispatches `reschedule-confirmed` with `{ newSlot }`.
5. On HTTP 4xx, shows an `<x-error-banner>` with the server message.
6. On HTTP 5xx or network failure, shows an `<x-error-banner>` with a generic message.
7. Clicking "Abbrechen" dispatches `reschedule-cancelled` without calling the API.

---

## Minimal usage example

```html
<x-reschedule-picker
  booking-id="bk-abc123"
  token="hmac-signed-value"
  service-id="svc-001">
</x-reschedule-picker>
```
