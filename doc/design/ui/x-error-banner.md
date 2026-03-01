# Component spec: `<x-error-banner>`

> **Output file:** `web/js/shared/x-error-banner.js`
> **Dependencies:** [`_conventions.md`](_conventions.md)

---

## Element name

`x-error-banner`

## Responsibility

Inline error message strip for page-level or section-level errors. Persists
until explicitly dismissed or `message` is cleared. Not animated (unlike
`<x-toast>`).

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `message` | `string` | `""` | Error text to display. Empty string hides the component. |
| `dismissible` | `boolean` | `false` | When present, renders a × dismiss button. |

---

## JS properties

Reflected from the observed attributes.

---

## Custom events

None.

---

## Internal states

| State | Description |
| --- | --- |
| `hidden` | `message` is empty; `display:none` or equivalent. |
| `visible` | `message` is non-empty; banner rendered in full. |

---

## Testable behaviours

1. When `message` attribute is non-empty, the component is visible.
2. When `message` attribute is empty (or absent), the component is hidden.
3. When `dismissible` attribute is present, a × button is rendered.
4. Clicking the × button sets `message` to `""` and hides the component.
5. When `dismissible` is absent, no × button is rendered.
6. The component has `role="alert"` so screen readers announce the message.
7. Changing `message` at runtime updates the displayed text immediately.

---

## Implementation hints

- Apply `role="alert"` and `aria-live="assertive"` on the root element inside the shadow root.
- Use `display: none` (toggled via a CSS class) to hide the banner rather than removing it from the DOM, so the `message` attribute can be set before it is connected.
- Use `--color-danger` as the accent / background tint colour.

---

## Minimal usage example

```html
<x-error-banner message="" dismissible></x-error-banner>
```

```js
banner.setAttribute('message', 'Netzwerkfehler – bitte erneut versuchen.');
```
