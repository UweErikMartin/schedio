# Component spec: `<x-booking-app>`

> **Output file:** `web/js/booking/x-booking-app.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md),
> [`x-stepper.md`](x-stepper.md), [`x-service-picker.md`](x-service-picker.md),
> [`x-date-time-picker.md`](x-date-time-picker.md), [`x-booking-line.md`](x-booking-line.md),
> [`x-booking-list.md`](x-booking-list.md), [`x-contact-form.md`](x-contact-form.md),
> [`x-session-summary.md`](x-session-summary.md), [`x-tandc-accept.md`](x-tandc-accept.md),
> [`x-booking-success.md`](x-booking-success.md), [`x-booking-manager.md`](x-booking-manager.md),
> [`x-error-banner.md`](x-error-banner.md), [`x-spinner.md`](x-spinner.md)

---

## Element name

`x-booking-app`

## Responsibility

Top-level orchestrator for the customer-facing single-page application at `/`.
Manages the multi-step booking flow state machine, creates and submits the
session via the API, and switches to management mode when the URL contains
`?id=` and `?token=`.

---

## Observed attributes

None. All state is internal.

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `step` | `number` (1–5) | Active step; triggers re-render. Managed internally. |
| `sessionId` | `string \| null` | Created after step 1; used in all subsequent API calls. |
| `selectedService` | `Service \| null` | Set when `service-selected` event fires. |
| `bookingLines` | `BookingLine[]` | Current booking lines state (step 2). |
| `contact` | `Contact \| null` | Contact data (step 3). |

---

## Internal states / modes

| State | Description |
| --- | --- |
| `booking-flow` | Default; renders `<x-stepper>` + active step component. |
| `management-mode` | URL has `?id=` and `?token=`; renders `<x-booking-manager>` without the stepper. |
| `loading` | Global spinner; interactions disabled (e.g. during session creation). |

---

## Step components

| Step | Component rendered |
| --- | --- |
| 1 | `<x-service-picker>` |
| 2 | `<x-booking-list>` |
| 3 | `<x-contact-form>` |
| 4 | `<x-session-summary>` + `<x-tandc-accept>` + submit button |
| 5 | `<x-booking-success>` |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `POST` | `/api/v1/sessions` | User selects a service (step 1 → 2) |
| `POST` | `/api/v1/sessions/{id}/bookings` | User adds a booking line (step 2) |
| `DELETE` | `/api/v1/sessions/{id}/bookings/{lineId}` | User removes a booking line (step 2) |
| `PATCH` | `/api/v1/sessions/{id}/bookings/{lineId}` | User changes a booking line's time (step 2) |
| `PUT` | `/api/v1/sessions/{id}/contact` | User confirms contact data (step 3 → 4) |
| `POST` | `/api/v1/sessions/{id}/submit` | User confirms T&C and submits (step 4 → 5) |

---

## Testable behaviours

1. On mount without `?id=` and `?token=` in the URL, renders `<x-stepper>` and `<x-service-picker>` (step 1).
2. On mount with both `?id=` and `?token=` present, renders `<x-booking-manager>` and does **not** render the stepper.
3. When `service-selected` fires, calls `POST /api/v1/sessions`, sets `sessionId`, and transitions to step 2.
4. When `timeslots-confirmed` fires (from a submit button on step 2), transitions to step 3.
5. When `contact-confirmed` fires, calls `PUT /api/v1/sessions/{id}/contact` and transitions to step 4.
6. When the submit button on step 4 is clicked (T&C accepted), calls `POST /api/v1/sessions/{id}/submit`; on success transitions to step 5.
7. When the session-submit call fails (non-2xx), stays on step 4 and renders `<x-error-banner>`.
8. Clicking a completed step in `<x-stepper>` (`step-clicked` event) navigates back to that step, preserving all collected data.
9. If the user navigates back to step 1 and selects a **different** service, the previous session is abandoned, a new session is created for the new service, and steps 2–4 data is reset.
10. Navigating back from step 3 to step 2 preserves the existing booking lines.
11. The submit button on step 4 is disabled until `acceptance-changed` emits `{ accepted: true }`.
12. After each step transition `window.scrollTo(0, 0)` is called.

---

## Minimal usage example

```html
<!-- web/html/index.html -->
<x-booking-app></x-booking-app>
```
