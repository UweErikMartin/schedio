---
applyTo: "web/js/**"
---

# schedio – Web Component Authoring Instructions

These instructions apply to every file under `web/js/`. They tell Copilot the
full technology contract for creating or editing schedio web components.

---

## Technology stack – hard rules

| Concern | Rule |
|---|---|
| Language | Vanilla JavaScript (ES2022+). No TypeScript. No npm packages. |
| Component model | Custom Elements v1 — `class XFoo extends HTMLElement`, `customElements.define('x-foo', XFoo)`. |
| Shadow DOM | Always `this.attachShadow({ mode: 'open' })` in the constructor. Template is rendered into `this.shadowRoot.innerHTML` (or by cloning a `<template>`). |
| Module system | ES modules. One component per file. Export the class as a named export **and** call `customElements.define(…)` at the bottom of the file. |
| HTTP | `fetch` + `async/await` only. Never use `XMLHttpRequest`. |
| Global state | Forbidden. Pass data via JS properties or custom events only. |
| Routing | `history.pushState` / `popstate` only in top-level `*-app.js` components. |
| Frameworks | React, Vue, Lit, Svelte, Angular — all forbidden. |

> **Legacy note:** `x-date-time-picker.js` and `x-service-picker.js` use
> `this.innerHTML` (light DOM) instead of Shadow DOM. New components must use
> Shadow DOM. When editing those legacy files, preserve their existing DOM model
> to avoid breaking their callers.

---

## File placement

```
web/js/
  booking/        ← public booking SPA components
  manage/         ← public booking-management view components
  shared/         ← utility components used everywhere (spinner, toast, dialog, …)
  admin/          ← admin SPA components
web/css/
  tokens.css      ← design tokens (must be imported inside every shadow root)
```

---

## Standard component skeleton

```js
const STYLES = `
  @import '/css/tokens.css';

  :host {
    display: block;
  }

  /* component styles here — use CSS custom properties from tokens.css */
`;

/**
 * XFoo — one-line description of responsibility.
 *
 * Attributes:  some-attr (string)
 * Properties:  value (string)
 * Events:      foo-selected { value: string }
 */
export class XFoo extends HTMLElement {
  // Private fields for all internal state
  #value = null;

  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
  }

  static get observedAttributes() {
    return ['some-attr'];
  }

  connectedCallback() {
    this.#render();
    this.#setup();
  }

  attributeChangedCallback(name, oldValue, newValue) {
    if (oldValue === newValue) return;
    // react to attribute changes after initial render
  }

  // ── Public JS property ────────────────────────────────────────────────────

  get value() { return this.#value; }
  set value(v) {
    this.#value = v;
    // update DOM if already rendered
  }

  // ── Private rendering ─────────────────────────────────────────────────────

  #render() {
    this.shadowRoot.innerHTML = `<style>${STYLES}</style>
      <!-- markup here -->
    `;
  }

  #setup() {
    // wire event listeners on this.shadowRoot.querySelector(…)
  }

  // ── Private helpers ───────────────────────────────────────────────────────

  #dispatch(eventName, detail = {}) {
    this.dispatchEvent(new CustomEvent(eventName, {
      bubbles: true,
      composed: true,
      detail,
    }));
  }
}

customElements.define('x-foo', XFoo);
```

---

## CSS and design tokens

Import `tokens.css` as the first rule inside every shadow root style block:

```css
@import '/css/tokens.css';
```

Use only the tokens below for colours, typography, spacing, and elevation.
Hard-coded hex values are allowed only for one-off overrides with a comment.

### Colour tokens

| Token | Value | Use |
|---|---|---|
| `--color-bg` | `#f9fafb` | Page background |
| `--color-surface` | `#ffffff` | Cards, inputs |
| `--color-border` | `#d1d5db` | Borders |
| `--color-text` | `#1f2937` | Body text |
| `--color-muted` | `#6b7280` | Placeholder / secondary |
| `--color-primary` | `#0f62fe` | Primary action |
| `--color-primary-hover` | `#0353d9` | Primary hover |
| `--color-danger` | `#dc2626` | Error / destructive |
| `--color-warning` | `#f59e0b` | Warning |
| `--color-success` | `#16a34a` | Confirmed / success |
| `--color-tentative` | `#7c3aed` | Reserved / tentative |

### Typography tokens

| Token | Value |
|---|---|
| `--font-family` | `Inter, "Segoe UI", system-ui, -apple-system, sans-serif` |
| `--font-size-sm` | `0.875rem` |
| `--font-size-base` | `1rem` |
| `--font-size-lg` | `1.125rem` |
| `--font-size-xl` | `1.25rem` |
| `--font-weight-normal` | `400` |
| `--font-weight-medium` | `500` |
| `--font-weight-bold` | `600` |

### Spacing / layout tokens

| Token | Value |
|---|---|
| `--space-xs` | `0.25rem` |
| `--space-sm` | `0.5rem` |
| `--space-md` | `1rem` |
| `--space-lg` | `1.5rem` |
| `--space-xl` | `2rem` |
| `--radius-sm` | `6px` |
| `--radius-md` | `10px` |
| `--radius-lg` | `16px` |
| `--shadow-card` | `0 1px 4px rgba(0,0,0,.08)` |

### Focus ring – never remove without replacement

```css
:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
```

### Responsive breakpoints

| Name | Min-width |
|---|---|
| `sm` | `480px` |
| `md` | `768px` |
| `lg` | `1024px` |

---

## Attribute and property conventions

- Observed attributes are always **lowercase kebab-case** in the DOM.
- Boolean attributes: presence = `true`, absence = `false`.
  - Read with `this.hasAttribute('name')`.
  - Write with `this.toggleAttribute('name', bool)`.
- Complex values (arrays, objects) are passed via **JS properties only**, never serialised into attributes.
- Every observed attribute has a matching JS getter/setter that reflects the DOM attribute.

---

## Custom event conventions

- Use `new CustomEvent(name, { bubbles: true, composed: true, detail: { … } })`.
- `detail` is always a plain object — never a primitive.
- Event names are `kebab-case`.
- Fire events from the component element (not from shadow-root children) so that `composed: true` propagates them through the shadow boundary.

---

## Accessibility baseline (WCAG 2.1 AA)

- Every interactive element must be keyboard-reachable (`tabindex` if needed) and operable with Enter/Space.
- Minimum touch target: 44 × 44 px.
- Error messages must use `role="alert"` or be associated via `aria-describedby`.
- When a dialog or overlay closes, return focus to the element that opened it.
- All text/background colour pairs must meet 4.5 : 1 contrast ratio.
- Disabled states use `aria-disabled="true"` (not just `disabled`) on elements that still need to be reachable for screen readers.

---

## UI language

All user-visible labels, button texts, and messages are written in **German**.
Use `Intl.DateTimeFormat(navigator.language, …)` and `Intl.NumberFormat(…)` for formatted output so the component adapts to the user's locale while keeping static labels in German.

---

## API reference

The base URL for all fetch calls is relative (no hard-coded hostname).
Use the `availability-endpoint` attribute for overridable endpoint prefixes.

### Public endpoints consumed by components

| Method | Path | Consumer component |
|---|---|---|
| `GET` | `/api/v1/services` | `<x-service-picker>` |
| `GET` | `/api/v1/availability?service_id={uuid}&period={YYYY-MM}` | Parent of `<x-date-time-picker>` (month scope) |
| `GET` | `/api/v1/availability?service_id={uuid}&period={YYYY-MM-DD}` | `<x-date-time-picker>` (day scope) |
| `POST` | `/api/v1/sessions` | `<x-booking-app>` |
| `POST` | `/api/v1/sessions/{id}/bookings` | `<x-booking-app>` |
| `PUT` | `/api/v1/sessions/{id}/bookings/{bookingId}` | `<x-booking-app>` |
| `DELETE` | `/api/v1/sessions/{id}/bookings/{bookingId}` | `<x-booking-app>` |
| `POST` | `/api/v1/sessions/{id}/submit` | `<x-booking-app>` |
| `GET` | `/api/v1/bookings/{id}?token={hmac}` | `<x-booking-manager>` |
| `PATCH` | `/api/v1/bookings/{id}/reschedule?token={hmac}` | `<x-reschedule-picker>` |
| `POST` | `/api/v1/bookings/{id}/cancel?token={hmac}` | `<x-cancel-confirm>` |

### Key JSON shapes

**Service** (from `GET /api/v1/services`):
```json
{ "id": "uuid", "name": "string", "summary": "string",
  "description": "string", "price": 49.50, "duration_minutes": 60 }
```

**AvailabilityResponse** (from `GET /api/v1/availability`):
```json
{
  "months": {
    "2026-03": {
      "2026-03-15": ["2026-03-15T09:00:00Z", "2026-03-15T09:15:00Z"],
      "2026-03-16": ["2026-03-16T11:00:00Z"]
    }
  }
}
```
All timestamps are RFC 3339 UTC. Convert to local time for display; submit the original UTC string to the API.

**Contact**:
```json
{ "first_name": "string", "last_name": "string",
  "email": "user@example.de", "phone": "+49 123 4567890" }
```

**Booking** (persisted):
```json
{
  "id": "uuid", "session_id": "uuid",
  "service": { }, "contact": { },
  "start_at": "2026-03-15T10:00:00Z",
  "end_at":   "2026-03-15T11:00:00Z",
  "state": "reserved|confirmed|cancelled",
  "cancel_reason": "customer|admin|noshow|null",
  "location": "string"
}
```

State → colour token mapping:

| state | token |
|---|---|
| `reserved` | `--color-tentative` |
| `confirmed` | `--color-success` |
| `cancelled` | `--color-muted` |

---

## `<x-date-time-picker>` – element interface contract

Defined in `web/js/x-date-time-picker.js`. Full API contract: `api/openapi-x-date-time-picker.yaml`.

### Observed attributes

| Attribute | Type | Default | Description |
|---|---|---|---|
| `availability-endpoint` | string | `/api/v1/availability` | Base URL for availability queries. |
| `service-id` | string | `""` | UUID passed as `?service_id=` in all requests. |
| `min-date` | string | today | ISO-8601 date. No earlier dates are shown. |
| `disabled` | boolean | `false` | All controls inert and dimmed. |
| `available-dates` | string | `""` | JSON `{ month: "YYYY-MM", timeSlots: { "YYYY-MM-DD": ["…Z"] } }` — set by the parent with month-scope availability. |
| `selected-time` | string | `""` | ISO-8601 datetime to pre-select programmatically. |
| `locale` | string | auto | BCP 47 tag; falls back to `document.documentElement.lang` then `navigator.language`. |

### Custom events emitted

| Event | `detail` | When |
|---|---|---|
| `date-time-selected` | `{ value: "2026-03-15T09:00:00Z" }` | User picks a time slot. |
| `date-time-cleared` | `{}` | User clears the selection. |

---

## `<x-service-picker>` – element interface contract

Defined in `web/js/x-service-picker.js`.

### Custom events emitted

| Event | `detail` | When |
|---|---|---|
| `service-selected` | `{ id: string, name: string, duration_minutes: number }` | User selects a service. |
| `service-cleared` | `{}` | User removes the selection. |

---

## Component inventory

| Tag | File | Responsibility |
|---|---|---|
| `<x-booking-app>` | `booking/booking-app.js` | Booking SPA orchestrator + step state machine |
| `<x-service-picker>` | `x-service-picker.js` | Service list + detail, fetches `/api/v1/services` |
| `<x-date-time-picker>` | `x-date-time-picker.js` | Calendar + time-slot picker |
| `<x-booking-manager>` | `manage/booking-manager.js` | Token-protected self-service page |
| `<x-toast>` | `x-toast.js` | Transient notification |

New components not yet implemented (target files in `web/js/`):

| Tag | Target file | Responsibility |
|---|---|---|
| `<x-stepper>` | `booking/stepper.js` | Horizontal progress indicator |
| `<x-booking-line>` | `booking/booking-line.js` | Single booking line (picker + remove button) |
| `<x-booking-list>` | `booking/booking-list.js` | Ordered list of booking lines + "add" button |
| `<x-contact-form>` | `booking/contact-form.js` | Name / email / phone with validation |
| `<x-session-summary>` | `booking/session-summary.js` | Read-only session summary |
| `<x-tandc-accept>` | `booking/tandc-accept.js` | T&C PDF link + mandatory acceptance checkbox |
| `<x-booking-success>` | `booking/booking-success.js` | Post-submission confirmation view |
| `<x-booking-card>` | `manage/booking-card.js` | Single booking row with reschedule/cancel |
| `<x-reschedule-picker>` | `manage/reschedule-picker.js` | Inline date-time picker for rescheduling |
| `<x-cancel-confirm>` | `manage/cancel-confirm.js` | Cancellation confirmation dialog |
| `<x-spinner>` | `shared/spinner.js` | Loading indicator |
| `<x-dialog>` | `shared/dialog.js` | Modal dialog wrapper |
| `<x-error-banner>` | `shared/error-banner.js` | Persistent error display |
| `<x-admin-app>` | `admin/admin-app.js` | Admin SPA orchestrator + routing |
| `<x-admin-nav>` | `admin/admin-nav.js` | Side-navigation |
| `<x-login-form>` | `admin/login-form.js` | Admin login form |
| `<x-admin-dashboard>` | `admin/admin-dashboard.js` | Dashboard shell |
| `<x-dashboard-today>` | `admin/dashboard-today.js` | Today's reserved + confirmed bookings |
| `<x-dashboard-pending>` | `admin/dashboard-pending.js` | Pending (unconfirmed) sessions |
| `<x-service-list>` | `admin/service-list.js` | Service catalogue list |
| `<x-service-form>` | `admin/service-form.js` | Create / edit service form |
| `<x-session-review>` | `admin/session-review.js` | Confirm / reject a booking session |
| `<x-settings-form>` | `admin/settings-form.js` | Global settings editor |
| `<x-tandc-upload>` | `admin/tandc-upload.js` | T&C PDF upload |
| `<x-secret-manager>` | `admin/secret-manager.js` | HMAC signing-secret rotation |

---

## Key design decisions – do not change without updating `doc/userinterface.md`

- **Management link format** — `/?id=<bookingID>&token=<HMAC-SHA256-signature>`. Two separate query parameters; never merge into one.
- **Remove button on first booking line** — hidden only when it is the sole remaining line; always visible for lines 2+.
- **First booking line pre-selection** — pre-filled with the earliest available slot on initial render of step 2.
- **`<x-booking-success>`** — receives all data via JS properties from `<x-booking-app>`; makes no API calls itself.
- **Admin dashboard today panel** — shows only `reserved` and `confirmed` bookings; never `cancelled` or `no-show`.
- **Pending sessions table** — must include the earliest requested booking date and time so the admin can prioritise.
