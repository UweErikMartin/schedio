# Component spec: `<x-spinner>`

> **Output file:** `web/js/shared/x-spinner.js`
> **Dependencies:** [`_conventions.md`](_conventions.md)

---

## Element name

`x-spinner`

## Responsibility

Animated loading indicator. Purely presentational — no API calls, no events.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `size` | `string` | `md` | Controls diameter. Allowed values: `sm`, `md`, `lg`. |
| `label` | `string` | `Lade…` | Value of `aria-label` for screen readers. |

---

## JS properties

None beyond the reflected attributes.

---

## Custom events

None.

---

## Internal states

The component has no named internal states; it is always visible when in the DOM.

---

## Testable behaviours

1. Renders an SVG or CSS-animated circle element inside the shadow root.
2. Has `role="status"` on the root element.
3. `aria-label` is set to the value of the `label` attribute (default `"Lade…"`).
4. When `size="sm"` the spinner diameter is smaller than for `size="md"`.
5. When `size="lg"` the spinner diameter is larger than for `size="md"`.
6. Changing the `size` attribute at runtime updates the layout without removing and re-adding the element.

---

## Implementation hints

- Use a CSS `@keyframes` animation (`transform: rotate(360deg)`) on an SVG `<circle>` with `stroke-dasharray` / `stroke-dashoffset` for a smooth arc spinner.
- Size values map to fixed diameter tokens, e.g. `sm` → 16px, `md` → 32px, `lg` → 48px.
- The component should impose no external margin; callers are responsible for layout spacing.

---

## Minimal usage example

```html
<x-spinner size="md" label="Dienste werden geladen…"></x-spinner>
```
