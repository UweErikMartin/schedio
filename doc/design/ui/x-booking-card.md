# Component spec: `<x-booking-card>`

> **Output file:** `web/js/manage/booking-card.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md)

---

## Element name

`x-booking-card`

## Responsibility

Displays a single booking's summary and provides action buttons for
rescheduling or cancellation.

---

## Observed attributes

None.

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `booking` | `Booking` | Full booking object to display. See `_data-shapes.md`. |
| `currency` | `string` | ISO 4217 currency code for price formatting. Default `EUR`. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `reschedule-requested` | `{}` | User clicks "Termin verschieben". |
| `cancel-requested` | `{}` | User clicks "Termin stornieren". |

---

## Displayed information

| Field | Source |
| --- | --- |
| Service name | `booking.service.name` |
| Booked time slots | `booking.slots[]` formatted with `Intl.DateTimeFormat` |
| Customer name | `booking.contact.first_name + ' ' + booking.contact.last_name` |
| Price | `booking.totalPrice` formatted with `Intl.NumberFormat` |
| Status | `booking.state` rendered as a localised badge |

---

## Testable behaviours

1. Renders service name, all booked slots, customer name, price, and status badge.
2. When `booking.state` is `cancelled`, the "Termin verschieben" and "Termin stornieren" buttons are hidden.
3. When `booking.state` is `reserved` or `confirmed`, both action buttons are visible.
4. Clicking "Termin verschieben" dispatches `reschedule-requested`.
5. Clicking "Termin stornieren" dispatches `cancel-requested`.
6. Slot times are formatted using `Intl.DateTimeFormat(navigator.language, { dateStyle: 'medium', timeStyle: 'short' })`.
7. Price is formatted using `Intl.NumberFormat(navigator.language, { style: 'currency', currency: this.currency })`.

---

## Minimal usage example

```js
const card = document.createElement('x-booking-card');
card.booking = myBooking;
card.currency = 'EUR';
document.body.appendChild(card);
```
