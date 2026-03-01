# Component spec: `<x-booking-success>`

> **Output file:** `web/js/booking/x-booking-success.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md)

---

## Element name

`x-booking-success`

## Responsibility

Read-only post-submission view shown in booking flow step 5. Displays the
session reference number, a summary of all reserved bookings, and a "reserved —
not yet confirmed" notice. Makes **no API calls** — all data is passed by the
parent via JS properties.

---

## Observed attributes

None.

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `sessionId` | `string` | Session reference shown prominently to the customer (e.g. as a labelled code). |
| `bookings` | `BookingLine[]` | Array of `{ service, start, end }` objects for the summary list. See `_data-shapes.md`. |
| `contact` | `Contact` | Customer contact; used to display the e-mail address the confirmation will be sent to. |

---

## Custom events

None.

---

## Testable behaviours

1. Renders the `sessionId` prominently, e.g. in a `<code>` element with a "Referenznummer:" label.
2. Renders one summary row per entry in `bookings` showing: service name, formatted date, and start time (using `Intl.DateTimeFormat` with `navigator.language`).
3. Renders a notice explaining that the bookings are *reserved but not yet confirmed*, and that a session result e-mail will be sent once the administrator reviews them.
4. Renders the customer e-mail address from `contact.email` with a "Bestätigung wird gesendet an:" label.
5. No interactive elements are present except a "Weiteren Termin buchen" link pointing to `/`.
6. Does not issue any `fetch` call; all data is received via JS properties from `<x-booking-app>`.
7. When `bookings` is an empty array, shows a "Keine Termine in dieser Buchung" notice.

---

## Implementation hints

- Format dates with `new Intl.DateTimeFormat(navigator.language, { dateStyle: 'full', timeStyle: 'short' })`.
- The "Weiteren Termin buchen" link should be a plain `<a href="/">` to keep navigation simple.

---

## Minimal usage example

```js
const el = document.createElement('x-booking-success');
el.sessionId = 'SES-20260315-00042';
el.bookings = [{ service: { name: 'Beratung' }, start: '2026-03-20T10:00:00Z', end: '2026-03-20T10:30:00Z' }];
el.contact = { first_name: 'Max', last_name: 'Mustermann', email: 'max@example.de', phone: '' };
document.body.appendChild(el);
```
