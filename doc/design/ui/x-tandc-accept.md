# Component spec: `<x-tandc-accept>`

> **Output file:** `web/js/booking/x-tandc-accept.js`
> **Dependencies:** [`_conventions.md`](_conventions.md)

---

## Element name

`x-tandc-accept`

## Responsibility

Shows a link to download the Terms and Conditions PDF and a mandatory acceptance
checkbox. The parent component (`<x-booking-app>`) listens to the emitted event
to enable or disable the booking submission button.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `tandc-url` | `string` | `""` | URL of the T&C PDF. When empty, the download link is replaced by a fallback notice. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `acceptance-changed` | `{ accepted: boolean }` | The checkbox is checked or unchecked. |

---

## Testable behaviours

1. When `tandc-url` is non-empty, renders an `<a>` element opening `tandc-url` in a new tab (`target="_blank"`, `rel="noopener"`).
2. When `tandc-url` is empty or absent, the link is not rendered; instead a "T&C nicht verfügbar" fallback notice is shown.
3. Renders a checkbox labelled "Ich akzeptiere die Allgemeinen Geschäftsbedingungen".
4. The checkbox is initially unchecked.
5. Checking the checkbox dispatches `acceptance-changed` with `{ accepted: true }`.
6. Unchecking the checkbox dispatches `acceptance-changed` with `{ accepted: false }`.
7. The checkbox label is associated with the input via `<label>` and `id`/`for` (or wrapping `<label>`).

---

## Implementation hints

- No fetch calls; all data comes via observed attributes.
- The component never enables or disables itself — it only reports checkbox state.

---

## Minimal usage example

```html
<x-tandc-accept tandc-url="/api/v1/settings/tandc"></x-tandc-accept>
```
