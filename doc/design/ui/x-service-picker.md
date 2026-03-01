# Component spec: `<x-service-picker>`

> **Output file:** `web/js/booking/x-service-picker.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md), [`x-spinner.md`](x-spinner.md), [`x-error-banner.md`](x-error-banner.md)

---

## Element name

`x-service-picker`

## Responsibility

Displays the list of bookable services fetched from the public API. Allows the
customer to browse service details and select one to proceed with booking.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `services-endpoint` | `string` | `/api/v1/services` | URL to fetch the service list from. |
| `currency` | `string` | `EUR` | ISO 4217 code used when formatting prices. |

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `services` | `Service[]` | When set externally, the component skips the fetch and renders this list directly. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `service-selected` | `{ service: Service }` | User clicks "Auswählen" in a service detail view. |

---

## Internal states

| State | Description |
| --- | --- |
| `loading` | Fetching the service list; `<x-spinner>` shown. |
| `error` | Fetch failed; error message with retry button shown. |
| `empty` | API returned `[]`; "Keine Dienste verfügbar" message shown. |
| `list` | Service cards rendered; no service selected for detail. |
| `detail` | One service card is expanded to full detail. |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `GET` | value of `services-endpoint` attribute | On `connectedCallback` (unless `services` property is pre-set) |

Expected response: `Service[]` (see `_data-shapes.md`).

---

## Testable behaviours

1. On `connectedCallback`, fetches `services-endpoint`; shows `<x-spinner>` during fetch.
2. On HTTP error or network failure, transitions to `error` state and shows a retry button. Clicking retry re-fetches.
3. When API returns an empty array `[]`, shows "Keine Dienste verfügbar" in `empty` state.
4. In `list` state, renders one card per service showing: name, summary, price (formatted with `Intl.NumberFormat`), and duration in minutes.
5. Clicking a service card transitions to `detail` state showing: name, full description, price, duration, a "Auswählen" button, and a back button.
6. Clicking "Auswählen" dispatches `service-selected` with `{ service }`.
7. Clicking back returns to `list` state.
8. When `services` JS property is set before or after connect, the fetch is skipped and the property value is rendered.

---

## Implementation hints

- Use `Intl.NumberFormat(navigator.language, { style: 'currency', currency: this.currency })` for price.
- Fetch once and cache in an instance variable; re-fetch only on retry.

---

## Minimal usage example

```html
<x-service-picker services-endpoint="/api/v1/services" currency="EUR"></x-service-picker>
```
