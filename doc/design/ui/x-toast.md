# Component spec: `<x-toast>`

> **Output file:** `web/js/shared/x-toast.js`
> **Dependencies:** [`_conventions.md`](_conventions.md)

---

## Element name

`x-toast`

## Responsibility

Transient notification shown at the top or bottom of the viewport. Auto-dismisses
after a configurable timeout. Hidden when no message is set.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `variant` | `string` | `info` | Visual variant. Allowed: `success`, `error`, `info`. |
| `duration` | `number` | `4000` | Milliseconds before auto-dismiss. |

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `message` | `string` | Text to display. Setting a non-empty value triggers the show animation. Setting an empty string hides the toast immediately. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `toast-dismissed` | `{}` | The toast becomes hidden, either via auto-dismiss or the close button. |

---

## Internal states

| State | Description |
| --- | --- |
| `hidden` | Default; component has `display:none` or equivalent. |
| `visible` | A message is set; dismiss timer is running. |

---

## Testable behaviours

1. When `message` property is set to a non-empty string, the component becomes visible.
2. When `message` is set to an empty string, the component hides immediately without firing `toast-dismissed`.
3. A close (×) button is rendered; clicking it sets `message` to `""`, hides the toast, and dispatches `toast-dismissed`.
4. After `duration` milliseconds the toast auto-dismisses and dispatches `toast-dismissed`.
5. `variant="success"` applies `--color-success` as the accent colour.
6. `variant="error"` applies `--color-danger` as the accent colour.
7. `variant="info"` applies `--color-primary` as the accent colour.
8. The component has `role="alert"` so screen readers announce it without requiring focus.
9. Setting `message` again while the toast is visible resets the dismiss timer.

---

## Implementation hints

- Position the toast with `position: fixed` at the top-right corner (or top-center on mobile).
- Use a CSS transition on `opacity` and `transform: translateY(…)` for the show/hide animation.
- Store the dismiss timer ID in an instance variable so it can be cleared on reassignment.
- `role="alert"` with `aria-live="assertive"` ensures immediate announcement.

---

## Minimal usage example

```js
const toast = document.createElement('x-toast');
toast.setAttribute('variant', 'success');
document.body.appendChild(toast);
toast.message = 'Änderungen gespeichert.';
```
