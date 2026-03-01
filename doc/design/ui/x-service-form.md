# Component spec: `<x-service-form>`

> **Output file:** `web/js/admin/service-form.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md),
> [`x-spinner.md`](x-spinner.md), [`x-error-banner.md`](x-error-banner.md)

---

## Element name

`x-service-form`

## Responsibility

Create / edit form for a single service. Operates in two modes: create (no
`service-id` attribute) and edit (with `service-id`).

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `service-id` | `string` | `""` | When set, fetches the existing service and populates the form for editing. When absent, the form is blank (create mode). |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `service-saved` | `{ service: Service }` | Service was successfully created or updated. |
| `service-cancelled` | `{}` | Admin clicked "Abbrechen". |

---

## Form fields

| Field | Type | Required |
| --- | --- | --- |
| Name | `text` | ✅ |
| Beschreibung (description) | `textarea` | ✅ |
| Kurztext (summary) | `text` | ✅ |
| Dauer (minutes) | `number` (min 1) | ✅ |
| Preis | `number` (min 0, step 0.01) | ✅ |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `GET` | `/api/v1/admin/services/{service-id}` | On `connectedCallback` when `service-id` is set (edit mode) |
| `POST` | `/api/v1/admin/services` | Saving in create mode |
| `PUT` | `/api/v1/admin/services/{service-id}` | Saving in edit mode |

---

## Testable behaviours

1. In create mode (no `service-id`), all fields are blank; title reads "Neuen Dienst erstellen".
2. In edit mode, fetches the service on mount, populates fields; title reads "Dienst bearbeiten".
3. Submitting with any required field empty shows validation errors and does not call the API.
4. On successful save, dispatches `service-saved` with the returned `Service`.
5. On API error, shows `<x-error-banner>` and leaves the form editable.
6. During save, the submit button is replaced by `<x-spinner>` and all inputs are disabled.
7. "Abbrechen" dispatches `service-cancelled` without calling the API.

---

## Minimal usage example

```html
<!-- create mode -->
<x-service-form></x-service-form>

<!-- edit mode -->
<x-service-form service-id="svc-001"></x-service-form>
```
