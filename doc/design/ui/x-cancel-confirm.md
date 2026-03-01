# Component spec: `<x-cancel-confirm>`

> **Output file:** `web/js/manage/cancel-confirm.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`x-dialog.md`](x-dialog.md),
> [`x-spinner.md`](x-spinner.md), [`x-error-banner.md`](x-error-banner.md)

---

## Element name

`x-cancel-confirm`

## Responsibility

Modal confirmation dialog that asks the customer to confirm their cancellation
intent and then calls the cancellation API endpoint.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `booking-id` | `string` | `""` | ID of the booking to cancel. |
| `token` | `string` | `""` | HMAC management token authorising the operation. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `cancellation-confirmed` | `{}` | The cancellation API call succeeded. |
| `cancellation-cancelled` | `{}` | User clicked "Abbrechen". |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `DELETE` | `/api/v1/bookings/{booking-id}?token={token}` | User clicks "Stornieren bestätigen" |

---

## Testable behaviours

1. Renders inside an `<x-dialog>` opened automatically.
2. Displays a warning message asking for cancellation confirmation with the booking reference visible.
3. "Stornieren bestätigen" button calls `DELETE` on the cancellation endpoint.
4. During the API call, the confirm button is replaced by an `<x-spinner>` and both buttons are disabled.
5. On success, dispatches `cancellation-confirmed` and closes the dialog.
6. On HTTP 4xx or 5xx, shows an `<x-error-banner>` inside the dialog and re-enables both buttons.
7. Clicking "Abbrechen" dispatches `cancellation-cancelled` and closes the dialog without calling the API.

---

## Minimal usage example

```html
<x-cancel-confirm booking-id="bk-abc123" token="hmac-signed-value"></x-cancel-confirm>
```
