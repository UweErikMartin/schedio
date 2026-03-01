# Component spec: `<x-session-summary>`

> **Output file:** `web/js/booking/x-session-summary.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md)

---

## Element name

`x-session-summary`

## Responsibility

Read-only summary of the complete booking session: selected service, all booking
lines with formatted date/time, and contact details. Used in booking flow
step 4 (confirmation before submission) and step 5 (success page). Makes no
API calls.

---

## Observed attributes

None.

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `service` | `Service \| null` | The selected service. See `_data-shapes.md`. |
| `lines` | `BookingLine[]` | Array of `{ id, value }` where `value` is ISO-8601 datetime or `null`. |
| `contact` | `Contact \| null` | Customer contact. See `_data-shapes.md`. |
| `currency` | `string` | ISO 4217 currency code, e.g. `"EUR"`. |

---

## Custom events

None.

---

## Testable behaviours

1. Renders the service name, price (formatted with `Intl.NumberFormat` and the `currency` property), and duration in minutes.
2. Renders one row per booking line showing the formatted date and start time using `Intl.DateTimeFormat` with `navigator.language`.
3. Renders the contact first name, last name, e-mail address, and phone number.
4. No interactive elements are rendered.
5. When `lines` is an empty array, shows a "Keine Termine ausgewählt" message instead of the booking list.
6. When `service` is `null`, renders a skeleton placeholder for the service section.
7. When `contact` is `null`, renders a skeleton placeholder for the contact section.
8. Setting any JS property triggers a re-render of the whole component.

---

## Implementation hints

- Format price: `new Intl.NumberFormat(navigator.language, { style: 'currency', currency: this.currency }).format(price)`.
- Format booking date/time: `new Intl.DateTimeFormat(navigator.language, { dateStyle: 'long', timeStyle: 'short' }).format(new Date(value))`.
- Skeleton placeholders can be `<div>` elements with a CSS animation (shimmer effect) matching the expected text dimensions.

---

## Minimal usage example

```js
const el = document.createElement('x-session-summary');
el.service = { id: '…', name: 'Beratung', price: 0, duration_minutes: 30 };
el.lines = [{ id: 'line-1', value: '2026-03-20T10:00' }];
el.contact = { first_name: 'Max', last_name: 'Mustermann', email: 'mm@example.de', phone: '+49 123 456' };
el.currency = 'EUR';
document.body.appendChild(el);
```
