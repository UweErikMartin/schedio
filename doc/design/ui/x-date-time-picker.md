# Component spec: `<x-date-time-picker>`

> **Output file:** `web/js/booking/x-date-time-picker.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md), [`x-spinner.md`](x-spinner.md)

---

## Element name

`x-date-time-picker`

## Responsibility

Inline calendar + time-slot picker for one booking line. Only dates and time
slots returned by the availability API are selectable. All other dates are
disabled.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `availability-endpoint` | `string` | `/api/v1/availability` | URL prefix for availability queries. Query params `service_id` and `date` are appended automatically. |
| `service-id` | `string` | `""` | Included as `?service_id=` in all availability queries. |
| `min-date` | `string` | today | ISO-8601 date (e.g. `2026-03-15`). No dates before this are shown or selectable. |
| `disabled` | `boolean` | `false` | When present, all interactive elements are inert and visually dimmed. |

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `value` | `string \| null` | The currently selected ISO-8601 datetime (`YYYY-MM-DDTHH:mm`). Settable programmatically; updates the collapsed display. |
| `availableDays` | `string[]` | Cached list of ISO-8601 dates that have at least one available slot. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `date-time-selected` | `{ value: string }` | User selects a time slot; `value` is ISO-8601 datetime. |
| `date-time-cleared` | `{}` | User clears the current selection. |

---

## Internal states

| State | Description |
| --- | --- |
| `collapsed` | Shows the selected value (or placeholder) and an open button. |
| `calendar-open` | Month calendar is displayed; user picks a date. |
| `time-slots-open` | Time slot buttons for the chosen date are displayed. |
| `loading-slots` | Fetching slots for a chosen date; spinner shown. |
| `no-slots` | API returned empty list for the chosen date. |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `GET` | `{availability-endpoint}?service_id={service-id}&date={YYYY-MM-DD}` | User clicks an enabled calendar date |

Expected response: `AvailabilitySlot[]` (see `_data-shapes.md`).

---

## Testable behaviours

1. In `collapsed` state: shows the selected date/time (or a "Datum wählen" placeholder) and a button to open the calendar.
2. Clicking the open button transitions to `calendar-open`.
3. Calendar shows the month of `min-date` (or current month if `min-date` is not set / is in the past).
4. Navigation arrows change the displayed month; navigating to a month before `min-date`'s month is blocked.
5. Only dates present in `availableDays` are enabled; all others have `aria-disabled="true"` and are not clickable.
6. Clicking an enabled date transitions to `loading-slots`, then fetches the availability endpoint, then transitions to `time-slots-open` or `no-slots`.
7. When `no-slots`: shows a "Für diesen Tag sind keine Termine verfügbar" message with a prompt to choose another date.
8. Time slots render as buttons labelled with the local start time using `Intl.DateTimeFormat`.
9. Clicking a time-slot button dispatches `date-time-selected` with the ISO-8601 datetime and collapses to `collapsed` state showing the selected value.
10. Setting `value` programmatically updates the collapsed display without triggering a fetch.
11. When `disabled` attribute is present, all interactive elements are inert (`disabled` or `aria-disabled="true"`) and visually dimmed.
12. On narrow viewports (< 480px) the calendar renders as a full-width overlay (bottom sheet) above other content.

---

## Implementation hints

- Store the list of available days as an instance variable fetched separately (or derived from the parent providing `availableDays`).
- Use `Intl.DateTimeFormat(navigator.language, { weekday: 'short', day: 'numeric' })` for calendar cells.
- Use `Intl.DateTimeFormat(navigator.language, { timeStyle: 'short' })` for slot buttons.
- The bottom-sheet overlay on mobile can be a `position: fixed` container with a backdrop.

---

## Minimal usage example

```html
<x-date-time-picker
  service-id="svc-001"
  min-date="2026-03-15"
  availability-endpoint="/api/v1/availability">
</x-date-time-picker>
```
