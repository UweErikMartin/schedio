# schedio – UI Component Specifications

This folder contains one Markdown file per Web Component used in the **schedio**
appointment-booking application. Each file is **self-contained**: it carries
everything a code-generation tool (e.g. GitHub Copilot in a separate project)
needs to produce a complete, working Custom Element implementation.

## How to use

1. Copy the relevant `*.md` file(s) into a new project that holds only the
   component implementations.
2. Tell Copilot / your AI assistant:
   > "Implement the Web Component described in this spec file. Produce a single
   > ES-module `.js` file. Use no external dependencies."
3. Copy the generated `.js` file back into this project under the path shown in
   the **Output file** line at the top of each spec.

## File map

### Shared reference

| File | Purpose |
| --- | --- |
| [`_conventions.md`](_conventions.md) | Technology rules, design-token table, CSS import pattern |
| [`_data-shapes.md`](_data-shapes.md) | JSON/JS object shapes used across multiple components |

### Shared utility components (`web/js/shared/`)

| File | Element | Responsibility |
| --- | --- | --- |
| [`x-spinner.md`](x-spinner.md) | `<x-spinner>` | Loading indicator |
| [`x-toast.md`](x-toast.md) | `<x-toast>` | Transient notification |
| [`x-dialog.md`](x-dialog.md) | `<x-dialog>` | Accessible modal dialog |
| [`x-error-banner.md`](x-error-banner.md) | `<x-error-banner>` | Inline error strip |

### Customer booking SPA (`web/js/booking/`)

| File | Element | Responsibility |
| --- | --- | --- |
| [`x-booking-app.md`](x-booking-app.md) | `<x-booking-app>` | Top-level booking orchestrator |
| [`x-stepper.md`](x-stepper.md) | `<x-stepper>` | Progress indicator |
| [`x-service-picker.md`](x-service-picker.md) | `<x-service-picker>` | Service list + detail |
| [`x-date-time-picker.md`](x-date-time-picker.md) | `<x-date-time-picker>` | Calendar + time-slot picker |
| [`x-booking-line.md`](x-booking-line.md) | `<x-booking-line>` | Single booking line |
| [`x-booking-list.md`](x-booking-list.md) | `<x-booking-list>` | Ordered list of booking lines |
| [`x-contact-form.md`](x-contact-form.md) | `<x-contact-form>` | Contact data entry |
| [`x-session-summary.md`](x-session-summary.md) | `<x-session-summary>` | Read-only session summary |
| [`x-tandc-accept.md`](x-tandc-accept.md) | `<x-tandc-accept>` | T&C acceptance checkbox |
| [`x-booking-success.md`](x-booking-success.md) | `<x-booking-success>` | Post-submission success view |

### Customer booking management (`web/js/manage/`)

| File | Element | Responsibility |
| --- | --- | --- |
| [`x-booking-manager.md`](x-booking-manager.md) | `<x-booking-manager>` | Management view orchestrator |
| [`x-booking-card.md`](x-booking-card.md) | `<x-booking-card>` | Single booking row |
| [`x-reschedule-picker.md`](x-reschedule-picker.md) | `<x-reschedule-picker>` | Inline rescheduling |
| [`x-cancel-confirm.md`](x-cancel-confirm.md) | `<x-cancel-confirm>` | Cancellation confirmation |

### Admin SPA (`web/js/admin/`)

| File | Element | Responsibility |
| --- | --- | --- |
| [`x-admin-app.md`](x-admin-app.md) | `<x-admin-app>` | Admin top-level orchestrator |
| [`x-admin-nav.md`](x-admin-nav.md) | `<x-admin-nav>` | Side-navigation |
| [`x-login-form.md`](x-login-form.md) | `<x-login-form>` | Admin login form |
| [`x-admin-dashboard.md`](x-admin-dashboard.md) | `<x-admin-dashboard>` | Dashboard shell |
| [`x-dashboard-today.md`](x-dashboard-today.md) | `<x-dashboard-today>` | Today's bookings panel |
| [`x-dashboard-pending.md`](x-dashboard-pending.md) | `<x-dashboard-pending>` | Pending sessions panel |
| [`x-service-list.md`](x-service-list.md) | `<x-service-list>` | Service catalogue list |
| [`x-service-form.md`](x-service-form.md) | `<x-service-form>` | Create/edit service |
| [`x-session-review.md`](x-session-review.md) | `<x-session-review>` | Confirm/reject bookings |
| [`x-settings-form.md`](x-settings-form.md) | `<x-settings-form>` | Global settings editor |
| [`x-tandc-upload.md`](x-tandc-upload.md) | `<x-tandc-upload>` | T&C PDF upload |
| [`x-secret-manager.md`](x-secret-manager.md) | `<x-secret-manager>` | HMAC secret rotation |
| [`x-retention-list.md`](x-retention-list.md) | `<x-retention-list>` | Data-retention pending list |
