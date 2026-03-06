# schedio â€” User Interface Specification

## 1. Overview

This document describes every web page served by schedio and decomposes each
page into its constituent **pure-JavaScript Web Components** (Custom Elements
API, no framework). It is the primary input artefact for both the web-component
designer and the test-case author.

For every component the following information is given:

- **Element name** â€” the HTML tag registered via `customElements.define`.
- **Responsibility** â€” single-sentence statement of purpose.
- **Observed attributes** â€” HTML attributes reflected in JS properties; all are
  lower-case strings in the DOM.
- **JS-only properties** â€” data that flows in via JS assignment (not parseable
  from HTML attributes due to complexity, e.g. arrays / objects).
- **Custom events** â€” events dispatched with `new CustomEvent(â€¦, { bubbles: true,
  composed: true })`, including the shape of `event.detail`.
- **Internal states** â€” named visual/logic states the component can be in.
- **Testable behaviours** â€” numbered list; each item is independently testable
  in a browser unit-test harness (e.g. using `@web/test-runner` + Chai) or as a
  Playwright e2e test scenario.

All components use a Shadow DOM with `mode: 'open'` unless stated otherwise.
Styles are applied inside the shadow root; the shared design-token stylesheet
documented in Â§3 is imported via `@import` inside each shadow root.

---

## 2. Technology Stack

| Concern | Technology |
| --- | --- |
| Component model | Custom Elements v1 (`HTMLElement` subclass, `customElements.define`) |
| Styling | Plain CSS with CSS custom properties (design tokens) |
| Shadow DOM | `mode: 'open'` on all components |
| Templates | `<template>` element cloned in `connectedCallback` |
| Module system | ES modules (`type="module"`), bundled only for production |
| HTTP client | `fetch` API with `async`/`await` |
| Routing (SPA) | `history.pushState` / `popstate` â€” managed by top-level app component |
| Form validation | Constraint Validation API + custom error display |
| Internationalisation | `Intl.DateTimeFormat`, `Intl.NumberFormat` using page locale |

No npm dependencies are allowed in component source files. Build tooling
(bundler, minifier) is outside the scope of this document.

---

## 3. Design Tokens and Global Reset

A single CSS file (`web/css/tokens.css`) defines all design tokens as CSS custom
properties on `:root`. All components import this file inside their shadow roots.

### 3.1 Colour palette

| Token | Default | Use |
| --- | --- | --- |
| `--color-bg` | `#f9fafb` | Page background |
| `--color-surface` | `#ffffff` | Card / elevated surface |
| `--color-border` | `#d1d5db` | Input borders |
| `--color-text` | `#1f2937` | Body text |
| `--color-muted` | `#6b7280` | Secondary / placeholder text |
| `--color-primary` | `#0f62fe` | Primary action |
| `--color-primary-hover` | `#0353d9` | Primary action â€” hover |
| `--color-danger` | `#dc2626` | Error state, delete action |
| `--color-warning` | `#f59e0b` | Warning / no-show state |
| `--color-success` | `#16a34a` | Success / confirmed state |
| `--color-tentative` | `#7c3aed` | Reserved / tentative state |

### 3.2 Typography

| Token | Default |
| --- | --- |
| `--font-family` | `Inter, "Segoe UI", system-ui, -apple-system, sans-serif` |
| `--font-size-sm` | `0.875rem` |
| `--font-size-base` | `1rem` |
| `--font-size-lg` | `1.125rem` |
| `--font-size-xl` | `1.25rem` |
| `--font-weight-normal` | `400` |
| `--font-weight-medium` | `500` |
| `--font-weight-bold` | `600` |

### 3.3 Spacing and layout

| Token | Default |
| --- | --- |
| `--space-xs` | `0.25rem` |
| `--space-sm` | `0.5rem` |
| `--space-md` | `1rem` |
| `--space-lg` | `1.5rem` |
| `--space-xl` | `2rem` |
| `--radius-sm` | `6px` |
| `--radius-md` | `10px` |
| `--radius-lg` | `16px` |
| `--shadow-card` | `0 1px 4px rgba(0,0,0,.08)` |

### 3.4 Breakpoints (media queries)

| Name | Min-width | Applies to |
| --- | --- | --- |
| `sm` | `480px` | Two-column contact form |
| `md` | `768px` | Wider card, inline stepper |
| `lg` | `1024px` | Admin two-column layout |

---

## 4. Page Map

| URL pattern | HTML served | SPA mode | Auth |
| --- | --- | --- | --- |
| `/` | `web/html/index.html` | Booking flow (default) | None |
| `/?token=&id=` | `web/html/index.html` | Booking management | Token |
| `/auth/login` | `web/html/admin.html` | Admin login | None |
| `/admin/` | `web/html/admin.html` | Admin dashboard | Cookie |
| `/admin/services/` | `web/html/admin.html` | Service list | Cookie |
| `/admin/session/{id}` | `web/html/admin.html` | Session review | Cookie |
| `/admin/settings/` | `web/html/admin.html` | General settings | Cookie |

Both HTML files are embedded in the Go binary via `//go:embed`. The top-level
app component reads `window.location` to determine the active view. Navigation
stays client-side using `history.pushState` for admin views and `replaceState`
for booking-flow step transitions.

Management links sent to customers include **both** `id` and `token` query
parameters: `/?id=<bookingID>&token=<signedToken>`. The `token` contains the
cryptographic HMAC-SHA256 signature over the booking ID (requirements §8 —
*Link protection*). The `id` parameter is provided separately so the frontend
can construct the `GET /api/v1/bookings/{id}` URL without decoding the token
client-side. This is compatible with the requirement that "the token encodes
the booking ID" — the token still contains the ID internally for signature
verification; the `?id=` parameter is an additional routing convenience.

---

## 5. Customer Booking SPA (`/`)

### 5.1 Page shell (`web/html/index.html`)

Minimal HTML shell:

```html
<!doctype html>
<html lang="de">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Termin buchen</title>
  <link rel="stylesheet" href="/css/tokens.css" />
  <script type="module" src="/js/booking-app.js"></script>
</head>
<body>
  <x-booking-app></x-booking-app>
</body>
</html>
```

`booking-app.js` is the ES-module entry point. It imports and registers all
customer-facing components.

### 5.2 Booking flow step machine

```text
Step 1 â”€ Service selection
  â”‚  event: service-selected
  â–¼
Step 2 â”€ Booking selection (one or more booking lines)
  â”‚  event: booking-confirmed
  â–¼
Step 3 â”€ Contact data entry
  â”‚  event: contact-confirmed
  â–¼
Step 4 â”€ Confirmation (summary + T&C acceptance)
  â”‚  event: session-submitted
  â–¼
Step 5 â”€ Success (session reference + instructions)
```

**Management mode** (`?token=` present in URL): steps 1â€“5 are replaced by a
single management view handled by `<x-booking-manager>` (Â§6).

---

### 5.3 Component: `<x-booking-app>`

**Responsibility** Top-level orchestrator for the customer-facing SPA. Manages
step state, session ID, and communication between child components.

**Observed attributes** â€” none (no meaningful HTML attribute)

**JS properties** — JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `step` | `number` (1â€“5) | Active step; triggers re-render |
| `sessionId` | `string \| null` | Created after step 1 |
| `selectedService` | `object \| null` | Service entity from API |
| `bookingLines` | `array` | Current booking lines (step 2 state) |
| `contact` | `object \| null` | Contact data (step 3 state) |

**Internal states** — Named visual/logic states the component can be in.

| State | Description |
| --- | --- |
| `booking-flow` | Default; renders stepper + active step component |
| `management-mode` | URL has `?token=`; renders `<x-booking-manager>` |
| `loading` | Spinner visible; no interactions |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. On mount without `?token=`, renders step 1 (`<x-service-picker>`).
2. On mount with `?token=&id=`, renders `<x-booking-manager>` and does not render the stepper.
3. When `service-selected` event is received from `<x-service-picker>`, transitions to step 2 and creates a session via `POST /api/v1/sessions`.
4. When `booking-confirmed` is received, transitions to step 3.
5. When `contact-confirmed` is received, transitions to step 4.
6. When `session-submitted` is received, calls `POST /api/v1/sessions/{id}/submit`, on success transitions to step 5.
7. When `session-submitted` API call returns a non-2xx status, stays on step 4 and renders a global error banner.
8. Clicking the step indicator for a completed step navigates back to that step, preserving all data entered in subsequent steps.
9. If the user navigates back to step 1 and selects a **different** service, the current session and all its booking lines are discarded, a new session is created for the new service, and steps 2–4 data is reset to initial state.
10. Navigating back from step 3 to step 2 preserves the existing booking lines.
11. On step transition the window is scrolled to the top.

---

### 5.4 Component: `<x-stepper>`

**Responsibility** Horizontal progress indicator showing the current step of the
booking flow.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `steps` | `string` (JSON array of labels) | Step labels |
| `current` | `number` | 1-based index of the active step |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `step-clicked` | `{ step: number }` | User clicks a completed step |

**Internal states**: `pending`, `active`, `completed` per step item.

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders one labelled indicator per entry in `steps`.
2. The indicator at index `current` has `aria-current="step"`.
3. Indicators for steps before `current` are visually marked as completed and are interactive.
4. Indicators for steps after `current` are visually muted and are not interactive (no pointer cursor, `aria-disabled="true"`).
5. Clicking a completed step indicator dispatches `step-clicked` with the correct step number.
6. Clicking the active or future step does not dispatch any event.
7. On narrow viewports (`< 480px`) the labels are hidden; only icons / numbers remain visible.

---

### 5.5 Component: `<x-service-picker>`

**Responsibility** Displays the list of available services fetched from
`GET /api/v1/services`. Supports browsing service details and selecting one.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `services-endpoint` | `string` | URL to fetch services from; default `/api/v1/services` |
| `currency` | `string` | ISO 4217 code for price display; default `EUR` |

**JS properties** — JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `services` | `Service[]` | Injected externally or fetched; triggers render |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `service-selected` | `{ service: Service }` | User selects a service |

**Internal states**: `loading`, `error`, `empty`, `list`, `detail`.

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. On `connectedCallback`, fetches `services-endpoint`; shows spinner during fetch.
2. On HTTP error or network failure, renders error state with retry button.
3. When the services list is empty (API returns `[]`), renders an "Keine Dienste verfügbar" message.
4. Renders one card per service showing: name, summary, price (formatted with `Intl.NumberFormat`), and duration in minutes.
5. Clicking a service card opens the detail view for that service.
6. Detail view shows name, summary, full description, price, and duration; has a "Auswählen" button and a back button.
7. Clicking "Auswählen" dispatches `service-selected` with the full service object.
8. Only one service may be selected; selecting a second service dispatches `service-selected` again (the parent discards the old session).
9. When `services` property is set externally, fetch is skipped.

---

### 5.6 Component: `<x-date-time-picker>`

**Responsibility** Inline calendar + time-slot picker for one booking line. Only
dates and times returned by `GET /api/v1/availability` are selectable.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `availability-endpoint` | `string` | URL prefix for availability queries |
| `service-id` | `string` | Service ID to include in availability queries |
| `min-date` | `string` | ISO-8601 date; no dates before this date are selectable |
| `disabled` | `boolean` | Renders in read-only mode when present |

**JS properties** — JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `value` | `string \| null` | Selected ISO-8601 datetime (`YYYY-MM-DDTHH:mm`); settable |
| `availableDays` | `string[]` | Cached list of dates with at least one slot |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `date-time-selected` | `{ value: string }` | User selects a time slot |
| `date-time-cleared` | `{}` | User clears the selection |

**Internal states**: `collapsed`, `calendar-open`, `time-slots-open`, `loading-slots`, `no-slots`.

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. In `collapsed` state, shows the selected date and time (or a placeholder) and a button to open the calendar.
2. Clicking the open button transitions to `calendar-open`.
3. Calendar shows the month view for `min-date`'s month or the current month, whichever is later.
4. Navigation arrows change the displayed month; navigating before `min-date`'s month is blocked.
5. Only dates that are present in `availableDays` are enabled; all others have `aria-disabled="true"`.
6. Clicking an enabled date fetches `GET /api/v1/availability?service_id=&date=` and transitions to `time-slots-open`.
7. If the fetch returns an empty list, shows `no-slots` message with a prompt to pick another date.
8. Time slots are rendered as buttons labelled with the local time (using `Intl.DateTimeFormat`).
9. Selecting a time slot dispatches `date-time-selected` and collapses the picker to display the chosen value.
10. Setting `value` programmatically updates the collapsed display without re-fetching.
11. When `disabled` attribute is present, all interactive elements are inert and visually dimmed.
12. On narrow viewports the calendar renders as a full-width overlay (bottom sheet) above other content.

---

### 5.7 Component: `<x-booking-line>`

**Responsibility** Represents one booking line in the session: one
`<x-date-time-picker>` plus a remove button. Emits events when the time
changes or the line is removed.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `line-id` | `string` | Unique ID matching the server booking-line record |
| `service-id` | `string` | Propagated to the inner `<x-date-time-picker>` |
| `min-date` | `string` | Propagated to the inner `<x-date-time-picker>` |
| `removable` | `boolean` | Shows remove button when present |

**JS properties** — JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `value` | `string \| null` | ISO-8601 datetime of the selected slot |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `line-changed` | `{ lineId: string, value: string }` | Date/time selection changed |
| `line-remove-requested` | `{ lineId: string }` | User clicks remove |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders an `<x-date-time-picker>` with the correct `service-id` and `min-date` propagated.
2. When the inner picker dispatches `date-time-selected`, the component dispatches `line-changed` with its `line-id` and the new value.
3. Remove button is visible only when the `removable` attribute is present.
4. Clicking remove dispatches `line-remove-requested` with the correct `line-id`.
5. When `removable` is absent, no remove button is rendered (not just hidden).
6. The component has a visible heading: "Termin 1", "Termin 2", etc., derived from its position in the parent list.

---

### 5.8 Component: `<x-booking-list>`

**Responsibility** Container for one or more `<x-booking-line>` elements.
Manages the ordered list and provides an "Weiteren Termin hinzufÃ¼gen" button.

**JS properties** — JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `lines` | `BookingLine[]` | Array of `{ id, value, minDate }` objects |
| `serviceId` | `string` | Passed to each child `<x-booking-line>` |
| `canAddMore` | `boolean` | Controls visibility of "add" button |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `line-add-requested` | `{}` | User clicks "add another" |
| `line-remove-requested` | `{ lineId: string }` | Bubbled from child |
| `line-changed` | `{ lineId: string, value: string }` | Bubbled from child |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders one `<x-booking-line>` per entry in `lines`.
2. The remove button is shown on a line only when it is **not** the sole remaining line; when only one line exists its remove button is absent. All lines beyond the first are always `removable`.
3. "Add" button is visible only when `canAddMore` is `true`.
4. Clicking "add" dispatches `line-add-requested`.
5. When a child `line-remove-requested` bubbles up, the component re-dispatches it so the parent can remove the line.
6. When `lines` is updated, lines are re-rendered without destroying unchanged picker state.
7. Each line's `min-date` is the end time of the preceding selected slot; the first line uses today's date.
8. On initial render of step 2, the first `<x-booking-line>` has its `value` property pre-set to the earliest available slot returned by `GET /api/v1/availability` for the selected service and today's date, satisfying the requirements pre-selection rule.

---

### 5.9 Component: `<x-contact-form>`

**Responsibility** Input form for customer contact details: full name, e-mail
address, telephone number. Validates inputs before emitting the confirmed event.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `disabled` | `boolean` | Makes all inputs read-only (used on confirmation step) |

**JS properties** — JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `value` | `Contact \| null` | Pre-fills all inputs; used when user navigates back |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `contact-confirmed` | `{ contact: Contact }` | User submits valid form data |

**Internal states**: `idle`, `invalid` (at least one field fails validation).

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders labelled inputs for `name`, `email`, and `phone`.
2. All three fields are required; submitting with any empty field shows field-level error messages using the Constraint Validation API.
3. E-mail field uses `type="email"` pattern validation.
4. Phone field accepts international (`+49 123 â€¦`) and local formats; validated against a regexp.
5. Error messages are announced by screen readers (`role="alert"` or `aria-describedby`).
6. On successful validation the component dispatches `contact-confirmed` with the field values.
7. When `value` property is set, all fields are pre-populated.
8. When `disabled` attribute is present, all inputs have `readonly`; no submit button is visible.
9. Tab order follows DOM order: name â†’ email â†’ phone â†’ submit.

---

### 5.10 Component: `<x-session-summary>`

**Responsibility** Read-only summary of the complete session: selected service,
all booking lines with formatted date/time, and contact details. Used in step 4
(confirmation) and step 5 (success).

**JS properties** — JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `service` | `Service \| null` | Selected service |
| `lines` | `BookingLine[]` | Array of `{ id, value }` |
| `contact` | `Contact \| null` | Customer contact |
| `currency` | `string` | ISO 4217 code |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Service name, price (formatted), and duration are rendered.
2. Each booking line renders the date and start time using `Intl.DateTimeFormat` with the user's locale.
3. Contact first name, last name, e-mail, and phone are rendered.
4. No interactive elements are rendered (read-only).
5. When `lines` is empty, a clear "Keine Termine ausgewÃ¤hlt" message is shown.
6. The component gracefully handles `null` for `service` or `contact` by showing skeleton placeholders.

---

### 5.11 Component: `<x-tandc-accept>`

**Responsibility** Shows the "Terms and Conditions" download link and a
mandatory acceptance checkbox. Emits an event when the checkbox state changes.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `tandc-url` | `string` | Download URL for the T&C PDF |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `acceptance-changed` | `{ accepted: boolean }` | Checkbox checked or unchecked |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders a link opening `tandc-url` in a new tab.
2. Renders a checkbox labelled "Ich akzeptiere die Allgemeinen GeschÃ¤ftsbedingungen".
3. The checkbox is initially unchecked.
4. Checking/unchecking the checkbox dispatches `acceptance-changed` with the current boolean state.
5. When `tandc-url` is empty or absent, the link is not rendered and a fallback "T&C nicht verfÃ¼gbar" notice is shown.

---

### 5.12 Component: `<x-booking-success>`

**Responsibility** Read-only post-submission view shown in step 5. Displays the
session reference, a summary of all bookings made, and a mandatory notice that
the bookings are reserved but not yet reviewed --- the customer will receive a
session result e-mail once the administrator completes the review.

**JS properties** --- JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `sessionId` | `string` | Session reference shown to the customer |
| `bookings` | `BookingLine[]` | List of `{ service, start, end }` for the summary |
| `contact` | `Contact` | Customer contact for the confirmation address |

**Testable behaviours** --- Independently verifiable assertions for unit and e2e tests.

1. Renders the session reference ID prominently (e.g. as a labelled code or reference number).
2. Renders a summary row per booking with: service name, formatted date, and start time.
3. Renders a "reserved -- not yet confirmed" notice explaining that a session result e-mail will follow once the administrator has reviewed all bookings.
4. Renders the customer e-mail address to confirm where the confirmation e-mail was sent.
5. No interactive elements other than a "New booking" link pointing to `/` are present.
6. The component does not issue any API calls; all data is passed via JS properties by `<x-booking-app>` after a successful `POST /api/v1/sessions/{id}/submit`.

---

## 6. Customer Booking Management View

Activated when the page URL contains both `?id=<bookingID>` and `?token=<signedToken>`
query parameters. The `<x-booking-app>` renders `<x-booking-manager>` instead of the
step flow. See §4 for the management link URL design rationale.

### 6.1 Component: `<x-booking-manager>`

**Responsibility** Top-level management view. Fetches the booking via
`GET /api/v1/bookings/{id}?token=` and dispatches to the correct sub-view.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `booking-id` | `string` | Extracted from the URL during boot |
| `token` | `string` | Extracted from the URL during boot |

**Internal states**: `loading`, `error`, `view`, `reschedule`, `cancel-confirm`, `cancelled`, `success`.

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. On mount, fetches `GET /api/v1/bookings/{booking-id}?token={token}`.
2. On `403`/`401` response, renders an "UngÃ¼ltiger oder abgelaufener Link" error state.
3. On success, renders `<x-booking-card>` in `view` state.
4. Clicking "Termin verschieben" transitions to `reschedule` state.
5. Clicking "Termin absagen" transitions to `cancel-confirm` state.
6. Clicking "Neuen Termin buchen" calls `POST /api/v1/bookings/{id}/new-session?token=` then redirects to `/` with the new session pre-selected.
7. After a successful reschedule API call, transitions to `view` state with updated data and shows a success toast.
8. After a successful cancellation API call, transitions to `cancelled` state.
9. In `cancelled` state all action buttons are hidden and a "Termin abgesagt" notice is shown.

---

### 6.2 Component: `<x-booking-card>`

**Responsibility** Displays the details of a single booking: service name,
date/time, status badge, and action buttons.

**JS properties** — JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `booking` | `Booking` | Full booking object from API |
| `currency` | `string` | ISO 4217 code |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `reschedule-requested` | `{ bookingId: string }` | User clicks "Verschieben" |
| `cancel-requested` | `{ bookingId: string }` | User clicks "Absagen" |
| `new-session-requested` | `{ bookingId: string }` | User clicks "Neuen Termin buchen" |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders service name, formatted start/end datetime, and location.
2. Renders a status badge using the booking state: `reserved` â†’ purple, `confirmed` â†’ green, `cancelled` â†’ grey.
3. For `reserved` and `confirmed` bookings, "Verschieben" and "Absagen" buttons are visible.
4. For `cancelled` bookings, action buttons are hidden.
5. All three Custom Events carry the correct `bookingId` in `detail`.
6. Date/time is formatted with `Intl.DateTimeFormat` using the user's locale and timezone.

---

### 6.3 Component: `<x-reschedule-picker>`

**Responsibility** Wraps `<x-date-time-picker>` to let the customer pick a new
slot for the same service. Includes "Speichern" and "Abbrechen" buttons.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `booking-id` | `string` | Booking being rescheduled |
| `token` | `string` | HMAC token for the management link |
| `service-id` | `string` | Required for availability queries |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `reschedule-confirmed` | `{ bookingId: string, newStart: string }` | User saves |
| `reschedule-cancelled` | `{}` | User cancels |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders an `<x-date-time-picker>` with `service-id` set correctly.
2. "Speichern" button is disabled until a time slot is selected.
3. Clicking "Speichern" calls `POST /api/v1/bookings/{booking-id}/reschedule?token=` with `{ new_start }` body.
4. On API success, dispatches `reschedule-confirmed` with the new start time.
5. On API error, shows an inline error message and keeps the form open.
6. Clicking "Abbrechen" dispatches `reschedule-cancelled` without making any API call.

---

### 6.4 Component: `<x-cancel-confirm>`

**Responsibility** Confirmation dialog content asking the customer to confirm
cancellation. Not a `<dialog>` element itself; hosted inside `<x-dialog>`.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `booking-id` | `string` | Booking being cancelled |
| `token` | `string` | HMAC token |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `cancellation-confirmed` | `{ bookingId: string }` | User confirms cancellation |
| `cancellation-cancelled` | `{}` | User aborts |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders a warning message explaining the consequences including the no-show deadline.
2. "Termin absagen" button calls `DELETE /api/v1/bookings/{booking-id}?token=`.
3. On API success, dispatches `cancellation-confirmed`.
4. On API error, shows an error notice.
5. "Abbrechen" dispatches `cancellation-cancelled` without API calls.

---

## 7. Shared Utility Components

### 7.1 Component: `<x-spinner>`

**Responsibility** Animated loading indicator.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `size` | `string` (`sm`, `md`, `lg`) | Controls diameter |
| `label` | `string` | `aria-label` for screen readers; default "Ladeâ€¦" |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders an SVG or CSS-animated circle.
2. Has `role="status"` and `aria-label` set to the `label` attribute value.
3. `size` attribute maps to CSS class controlling the diameter.

---

### 7.2 Component: `<x-toast>`

**Responsibility** Transient notification shown at the top or bottom of the
viewport. Auto-dismisses after a configurable timeout.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `variant` | `string` (`success`, `error`, `info`) | Controls colour |
| `duration` | `number` | Milliseconds before auto-dismiss; default `4000` |

**JS properties** — JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `message` | `string` | Text to display; setting triggers show animation |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `toast-dismissed` | `{}` | Component hides (auto or manual) |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Setting `message` makes the component visible.
2. A close button manually dismisses the toast and dispatches `toast-dismissed`.
3. After `duration` milliseconds the toast auto-dismisses.
4. `variant` applies the corresponding colour token.
5. The toast has `role="alert"` so screen readers announce it.
6. Multiple `message` assignments reset the dismiss timer.

---

### 7.3 Component: `<x-dialog>`

**Responsibility** Accessible modal dialog wrapper using the native `<dialog>`
element. Exposes `open()` / `close()` methods. Slotted content is the body.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `dialog-title` | `string` | Visible heading inside the dialog |
| `close-label` | `string` | `aria-label` for the Ã— button; default "SchlieÃŸen" |

**JS methods** — Public JavaScript methods callable on the element instance.

| Method | Description |
| --- | --- |
| `open()` | Shows the dialog (`showModal()`) and traps focus |
| `close()` | Hides the dialog and restores focus to the trigger element |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `dialog-closed` | `{}` | Dialog is closed by any means |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Before `open()` the dialog is not visible (`display: none`).
2. Calling `open()` shows the dialog; backdrop is rendered.
3. Focus is moved to the first focusable element inside the dialog.
4. Tab key cycles focus within the dialog; focus does not escape.
5. Pressing Escape calls `close()` and dispatches `dialog-closed`.
6. Clicking the backdrop calls `close()`.
7. After `close()`, focus returns to the element that triggered `open()`.
8. The heading is rendered as `<h2>` with the value of `dialog-title`.

---

### 7.4 Component: `<x-error-banner>`

**Responsibility** Inline error message strip for page-level or section-level
errors.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `message` | `string` | Error text; empty string hides the component |
| `dismissible` | `boolean` | Shows dismiss button when present |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. When `message` is non-empty the component is visible.
2. When `message` is empty the component is hidden.
3. When `dismissible` is present and the user clicks "Ã—", `message` is cleared and the component hides.
4. The component has `role="alert"` for screen reader announcements.

---

## 8. Admin SPA (`/admin/`)

### 8.1 Page shell (`web/html/admin.html`)

```html
<!doctype html>
<html lang="de">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>schedio Admin</title>
  <link rel="stylesheet" href="/css/tokens.css" />
  <script type="module" src="/js/admin-app.js"></script>
</head>
<body>
  <x-admin-app></x-admin-app>
</body>
</html>
```

### 8.2 Component: `<x-admin-app>`

**Responsibility** Top-level orchestrator for the admin SPA. Determines the
active view from `window.location.pathname`. Manages the session auth cookie
presence check and renders the login view when unauthenticated.

**Internal states / views** — Named view states the component renders.

| State | Activation condition |
| --- | --- |
| `login` | Path is `/auth/login` or any `401` response from admin API |
| `dashboard` | Path is `/admin/` |
| `services` | Path starts with `/admin/services/` |
| `session-review` | Path matches `/admin/session/{id}` |
| `settings` | Path starts with `/admin/settings/` |
| `retention` | Path starts with `/admin/retention/` |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. On mount with path `/auth/login`, renders `<x-login-form>` without the nav.
2. On mount with path `/admin/`, fetches dashboard data and renders `<x-admin-nav>` + dashboard view.
3. When any API call returns `401`, transitions to `login` state.
4. `history.pushState` navigation between admin routes re-renders the correct view without a full page reload.
5. The `<x-admin-nav>` highlights the active route.

---

### 8.3 Component: `<x-admin-nav>`

**Responsibility** Persistent navigation sidebar or top bar listing the four
admin areas. Renders as a top bar on narrow viewports and as a sidebar on wide
viewports.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `active` | `string` | One of `dashboard`, `services`, `settings`, `retention` |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `nav-navigate` | `{ path: string }` | User clicks a nav link |
| `nav-logout` | `{}` | User clicks "Abmelden" |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders links for "Dashboard", "Dienste", "Einstellungen", and "Datenlöschung".
2. The link matching `active` has `aria-current="page"` and a distinctive active style.
3. Clicking a link dispatches `nav-navigate` with the target path without performing a full navigation.
4. Clicking "Abmelden" sends `POST /auth/logout` and dispatches `nav-logout`.
5. On narrow viewports (`< 768px`) the nav collapses to a hamburger toggle.
6. Hamburger toggle opens/closes the nav panel; focus is trapped in the open panel.

---

### 8.4 Login view – Component: `<x-login-form>`

**Responsibility** Renders the admin login page with username/password fields
and an "Anmelden" button plus a (normally disabled) "Anmelden mit Apple" button.
The Apple button is enabled dynamically after the username field loses focus
(`blur`) or the user presses Enter, if `GET /auth/apple/available?username=`
returns `{ "apple_enabled": true }`.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `action` | `string` | URL to POST credentials to; default `/auth/login` |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `login-success` | `{ role: string }` | Server confirmed valid credentials |
| `login-error` | `{ message: string }` | Login failed |

**Internal states**: `idle`, `checking-apple`, `ready`.

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders username input, password input, "Anmelden" button, and "Anmelden mit Apple" button.
2. "Anmelden mit Apple" button starts disabled (`disabled` attribute, `aria-disabled="true"`).
3. Does NOT query `/auth/apple/available` on every keystroke; queries on `blur` or Enter in the username field.
4. On `blur`/Enter with a non-empty username, calls `GET /auth/apple/available?username=<url-encoded-username>`.
5. When the response is `{ "apple_enabled": true }`, enables the "Anmelden mit Apple" button.
6. When the response is `{ "apple_enabled": false }` or a network error occurs, the button remains disabled.
7. Submitting empty fields shows validation errors; no API call is made.
8. Submitting valid credentials calls `POST /auth/login` with `application/x-www-form-urlencoded` body.
9. On `200` response, dispatches `login-success` with `{ role }` from the server response.
10. On `401` response, renders an inline error and dispatches `login-error`.
11. Clicking an enabled "Anmelden mit Apple" button navigates to `GET /auth/apple`.
12. Password field has `type="password"`; a show/hide toggle is provided.
13. During the credentials fetch, the "Anmelden" button is replaced by a spinner and cannot be clicked again.
14. Tab order: username -> password -> "Anmelden" -> "Anmelden mit Apple".

---

### 8.5 Dashboard view â€” Component: `<x-admin-dashboard>`

**Responsibility** Fetches and displays the two dashboard panels defined by
`GET /admin/api/v1/dashboard`: bookings of the day and pending confirmations.

**Internal states**: `loading`, `error`, `ready`.

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. On mount fetches `/admin/api/v1/dashboard`.
2. Until the response arrives, renders skeletons for both panels.
3. On error renders `<x-error-banner>` with a retry button.
4. Renders `<x-dashboard-today>` and `<x-dashboard-pending>` once data is available.
5. Auto-refreshes every 60 seconds without full re-render (only updated rows change).

---

### 8.5.1 Component: `<x-dashboard-today>`

**Responsibility** Shows a chronological list of today's bookings.

**JS properties** — JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `bookings` | `Booking[]` | List of today's bookings |
| `currency` | `string` | ISO 4217 code |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders a table row per booking with columns: time, service name, customer name, status badge.
2. Only bookings with state `reserved` or `confirmed` are shown; cancelled and no-show bookings are excluded.
3. When `bookings` is empty (or all bookings are excluded by the state filter), shows "Heute keine Termine" message.
4. Status badges use colours defined in Â§3.1.
5. Rows are sorted by `start_at` ascending.

---

### 8.5.2 Component: `<x-dashboard-pending>`

**Responsibility** Shows a list of booking sessions awaiting review, sorted by
submission time.

**JS properties** — JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `sessions` | `BookingSession[]` | Sessions with at least one `reserved` booking |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `session-open-requested` | `{ sessionId: string }` | Admin clicks "PrÃ¼fen" |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders one row per session with: customer name, service name, count of bookings, earliest requested booking date and time, and submission time (relative).
2. Each row has a "PrÃ¼fen" button that dispatches `session-open-requested`.
3. When `sessions` is empty, shows "Keine ausstehenden BestÃ¤tigungen" message.
4. Rows are sorted by `submitted_at` ascending (oldest first).

---

### 8.6 Services view

#### 8.6.1 Component: `<x-service-list>`

**Responsibility** Renders an admin table of all services with edit and delete
actions. Fetches from `GET /admin/api/v1/services`.

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `service-edit-requested` | `{ service: Service }` | Admin clicks "Bearbeiten" |
| `service-delete-requested` | `{ serviceId: string }` | Admin clicks "LÃ¶schen" |
| `service-create-requested` | `{}` | Admin clicks "Neuen Dienst anlegen" |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Fetches service list on mount; shows spinner.
2. Renders table columns: Name, Dauer, Preis, Tageslimit, Aktionen.
3. "Bearbeiten" dispatches `service-edit-requested` with the full service object.
4. "LÃ¶schen" shows an inline confirmation row before dispatching `service-delete-requested`.
5. A service with active bookings: the "LÃ¶schen" button is disabled with a tooltip explaining why.
6. "Neuen Dienst anlegen" button is rendered above the table.
7. Empty list shows "Noch keine Dienste angelegt" placeholder.

---

#### 8.6.2 Component: `<x-service-form>`

**Responsibility** Create / edit form for a service. Renders inline or inside
`<x-dialog>`.

**JS properties** — JavaScript properties set programmatically (too complex for HTML attributes).

| Property | Type | Description |
| --- | --- | --- |
| `service` | `Service \| null` | Pre-fills fields for edit; `null` for create |

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `service-saved` | `{ service: Service }` | API call succeeded |
| `service-form-cancelled` | `{}` | Admin cancels |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders inputs for: Name, Kurzfassung, Beschreibung, Preis, Dauer (Minuten), Tageslimit.
2. When `service` is non-null all fields are pre-populated.
3. All fields except Tageslimit (defaults to `0`) are required.
4. Price accepts only non-negative decimal numbers.
5. Duration and daily limit accept only positive integers.
6. Submitting a create form calls `POST /admin/api/v1/services`.
7. Submitting an edit form calls `PUT /admin/api/v1/services/{id}`.
8. On success, dispatches `service-saved`.
9. On API error, shows field-level or form-level error message.
10. Clicking "Abbrechen" dispatches `service-form-cancelled` without API calls.

---

### 8.7 Session review view â€” Component: `<x-session-review>`

**Responsibility** Loads a booking session and lets the admin confirm or reject
each individual booking.

**Observed attributes** — HTML attributes set in markup, reflected as JS properties.

| Attribute | Type | Description |
| --- | --- | --- |
| `session-id` | `string` | ID of the session to load |

**Internal states**: `loading`, `error`, `review`, `completed`.

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Fetches `GET /admin/api/v1/sessions/{session-id}` on mount.
2. Shows customer name, email, phone, and service details in a summary header.
3. Renders one row per booking with: date/time, status badge, "BestÃ¤tigen" and "Ablehnen" buttons.
4. Clicking "BestÃ¤tigen" calls `POST /admin/api/v1/sessions/{id}/bookings/{bookingId}/confirm`.
5. Clicking "Ablehnen" calls `POST /admin/api/v1/sessions/{id}/bookings/{bookingId}/reject`.
6. After each action the booking row's status badge updates immediately (optimistic update).
7. When all bookings in the session have been reviewed (none in `reserved` state), the component transitions to `completed` state.
8. In `completed` state a green confirmation message is shown; the action buttons are hidden.
9. "No-show" button is present for `confirmed` bookings and calls `POST /admin/api/v1/bookings/{id}/noshow`.
10. On API error an `<x-error-banner>` appears above the affected row.

---

### 8.8 Settings view

#### 8.8.1 Component: `<x-settings-form>`

**Responsibility** Edit form for general settings: CalDAV server URL, no-show deadline, reminder
lead time, sender name, currency, appointment location, default CalDAV calendar
name.

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Fetches `GET /admin/api/v1/settings` on mount.
1. Renders fields: Kalender-URL, Kalenderbezeichnung, Absender-Name, Terminsort, Währung, No-show-Frist (Stunden), Aufbewahrungsfrist (Tage), Erinnerungsfrist (Tage).
1. CalDAV server URL is a free-text input; labelled "Kalender-URL". Hint text states the expected format (e.g. `caldav.example.com`). Seeded from the `--calendarUrl` startup flag; changes take effect immediately without a server restart.
1. No-show deadline is a positive integer input.
1. Retention period is a positive integer input (default 30); labelled "Aufbewahrungsfrist (Tage)".
1. Reminder lead time is a positive integer input (default 1); labelled "Erinnerungsfrist (Tage)".
1. Sender name is a free-text input (default "Schedio Buchungssystem"); labelled "Absender-Name".
1. Currency is a free-text input; a dropdown of common ISO 4217 codes is offered as a datalist.
1. Default CalDAV calendar name is a free-text input; labelled "Kalenderbezeichnung". Hint text states the fallback is "Default Calendar" when left empty. When saved with a non-empty value the default staff CalDAV calendar is immediately renamed in CalDAV clients.
1. Saving calls `PUT /admin/api/v1/settings` with the updated values.
1. On success displays a success toast.
1. On error displays an inline error banner.

---

#### 8.8.2 Component: `<x-tandc-upload>`

**Responsibility** Upload widget for the Terms and Conditions PDF.

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `tandc-uploaded` | `{ filename: string }` | Upload succeeded |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. Renders a file input accepting `application/pdf` only.
2. Selected file size is shown; files larger than 10 MB are rejected client-side before upload.
3. "Hochladen" button calls `POST /admin/api/v1/settings/tandc` as `multipart/form-data`.
4. During upload a progress indicator is shown.
5. On success, dispatches `tandc-uploaded` and shows the new filename.
6. On error, shows an inline error message.
7. Current filename (if any) is fetched from the settings response and shown as a read-only label.

---

#### 8.8.3 Component: `<x-secret-manager>`

**Responsibility** Download/upload widget for the HMAC management-link secret.

**Custom events** — DOM CustomEvents dispatched by this component.

| Event | `detail` | When |
| --- | --- | --- |
| `secret-replaced` | `{}` | Upload succeeded |

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. "Geheimnis herunterladen" button triggers `GET /admin/api/v1/settings/secret` and forces a file download.
2. "Geheimnis ersetzen" shows a file picker accepting any binary file.
3. Clicking "Hochladen" calls `POST /admin/api/v1/settings/secret` with the file contents as `application/octet-stream`.
4. Before upload, a warning dialog (`<x-dialog>`) explains that uploading invalidates all existing management links.
5. On success, dispatches `secret-replaced` and shows a warning toast "Alle bestehenden Verwaltungslinks sind jetzt ungÃ¼ltig".
6. On error, shows an inline error banner.

---

### 8.9 Data Retention view — Component: `<x-retention-list>`

**Responsibility** Displays the list of customer contacts in `pending_deletion`
state — i.e., whose deletion-confirmation link expired without being clicked.
Allows Staff users to permanently delete individual records.

**Internal states**: `loading`, `empty`, `ready`, `error`.

**Testable behaviours** — Independently verifiable assertions for unit and e2e tests.

1. On mount, calls `GET /admin/api/v1/retention/pending` and transitions from `loading` to `ready` or `error`.
2. In `ready` state, renders a table with columns: Nachname, Vorname, E-Mail, Telefon, Frist erreicht am.
3. Dates in the "Frist erreicht am" column are formatted with `Intl.DateTimeFormat` using the user's locale.
4. In `empty` state (empty array response), shows a "Keine ausstehenden Löschvorgänge" message and no table.
5. Each row has a "Löschen" button.
6. Clicking "Löschen" opens a confirmation `<x-dialog>` asking "Kundendaten und alle Buchungen dauerhaft löschen?"
7. Confirming the dialog calls `DELETE /admin/api/v1/retention/pending/{id}`.
8. On successful delete, removes the row from the list and shows a success toast.
9. On error, closes dialog and shows an `<x-error-banner>`.
10. "Löschen" button shows a spinner while the DELETE request is in flight; the row is not removed until success.

---

All JSON requests use `Content-Type: application/json`; responses are also JSON
unless stated otherwise. Auth token endpoints use
`application/x-www-form-urlencoded`.

### 9.1 Service shape

```json
{
  "id": "uuid",
  "name": "string",
  "description": "string",
  "price": 49.50,
  "duration_minutes": 60,
  "daily_limit": 0
}
```

### 9.2 BookingLine shape (session state)

```json
{
  "id": "uuid",
  "start": "2026-03-15T10:00:00Z",
  "end": "2026-03-15T11:00:00Z",
  "service_id": "uuid"
}
```

### 9.3 Contact shape

```json
{
  "first_name": "string",
  "last_name": "string",
  "email": "string",
  "phone": "string"
}
```

### 9.4 Booking shape

```json
{
  "id": "uuid",
  "session_id": "uuid",
  "service": { /* Service */ },
  "contact": { /* Contact */ },
  "start_at": "2026-03-15T10:00:00Z",
  "end_at": "2026-03-15T11:00:00Z",
  "state": "reserved|confirmed|cancelled",
  "cancel_reason": "customer|admin|noshow|null",
  "location": "string"
}
```

### 9.5 Dashboard response shape

```json
{
  "bookings_today": [ /* Booking[] */ ],
  "pending_sessions": [
    {
      "id": "uuid",
      "service": { /* Service */ },
      "contact": { /* Contact */ },
      "booking_count": 2,
      "submitted_at": "2026-03-14T08:30:00Z"
    }
  ]
}
```

### 9.6 Settings shape

```json
{
  "calendar_url": "caldav.example.com",
  "no_show_deadline_hours": 24,
  "retention_period_days": 30,
  "reminder_lead_time_days": 1,
  "sender_name": "Schedio Buchungssystem",
  "currency": "EUR",
  "appointment_location": "Musterstraße 1, 10115 Berlin",
  "tandc_filename": "agb.pdf"
}
```

---

## 10. Accessibility Requirements

All components must meet WCAG 2.1 Level AA. Key requirements:

1. **Keyboard navigability** â€” every interactive element is reachable and operable via keyboard alone.
2. **Focus visibility** â€” focus ring is clearly visible; the default browser outline must not be removed without an equivalent replacement using a `--focus-ring` token.
3. **Colour contrast** â€” all text/background pairs meet a contrast ratio â‰¥ 4.5:1; large text â‰¥ 3:1. The design tokens defined in Â§3.1 satisfy this constraint.
4. **ARIA landmarks** â€” each page has exactly one `<main>`, `<nav>` for the admin nav, and `<header>` for the page title.
5. **Form errors** â€” error messages are associated with their input via `aria-describedby` and are announced immediately via `role="alert"`.
6. **Dialog** â€” the `<x-dialog>` component uses the native `<dialog>` element which provides correct ARIA role and focus trapping without additional ARIA hacks.
7. **Status updates** â€” all `<x-toast>` elements use `role="alert"` to be announced by screen readers without requiring focus.
8. **Touch targets** â€” minimum touch target size 44Ã—44 px on all interactive elements.

---

## 11. Responsive Layout and Grid

The layout uses a single-column CSS grid on mobile scaling to multi-column on
wider viewports. All layout rules are pure CSS using `display: grid` and
`display: flex`; no external grid library.

### 11.1 Booking SPA layout

```text
< 768px                      â‰¥ 768px
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”     â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚  <x-stepper>         â”‚     â”‚  <x-stepper> (horizontal)â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚     â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚  Active step content â”‚     â”‚  Active step content      â”‚
â”‚  (full width)        â”‚     â”‚  (centred, max-width 640) â”‚
â”‚                      â”‚     â”‚                           â”‚
â”‚  [CTA button]        â”‚     â”‚  [CTA button]             â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜     â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

### 11.2 Admin SPA layout

```text
< 1024px                         â‰¥ 1024px
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”     â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚  [â˜°] schedio Admin       â”‚     â”‚  Nav   â”‚  Main content    â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚     â”‚  side  â”‚                  â”‚
â”‚  Main content (full w.)  â”‚     â”‚  bar   â”‚  (scrollable)    â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜     â””â”€â”€â”€â”€â”€â”€â”€â”€â”´â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

### 11.3 Contact form grid

The contact form uses a two-column grid on `â‰¥ 480px` viewports:

```text
< 480px          ≥ 480px
Vorname          Vorname     │  Nachname
Nachname         E-Mail      │
E-Mail           Telefon     │
Telefon
```

---

## 12. Component File Layout

Each component lives in its own ES-module file. The two entry points import
and register all components they need.

```text
web/js/
  booking-app.js          â† entry point; registers all customer components
  admin-app.js            â† entry point; registers all admin components
  components/
    x-booking-app.js
    x-stepper.js
    x-service-picker.js
    x-date-time-picker.js
    x-booking-line.js
    x-booking-list.js
    x-contact-form.js
    x-session-summary.js
    x-tandc-accept.js
    x-booking-manager.js
    x-booking-card.js
    x-reschedule-picker.js
    x-cancel-confirm.js
    x-spinner.js
    x-toast.js
    x-dialog.js
    x-error-banner.js
    x-admin-app.js
    x-admin-nav.js
    x-login-form.js
    x-admin-dashboard.js
    x-dashboard-today.js
    x-dashboard-pending.js
    x-service-list.js
    x-service-form.js
    x-session-review.js
    x-settings-form.js
    x-tandc-upload.js
    x-secret-manager.js
    x-retention-list.js
web/css/
  tokens.css              â† design tokens only; imported inside every shadow root
  reset.css               â† minimal global reset (box-sizing, margin, padding)
  global.css              â† page-level layout (body, main, error pages)
```
