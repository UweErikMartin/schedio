# Component spec: `<x-stepper>`

> **Output file:** `web/js/booking/x-stepper.js`
> **Dependencies:** [`_conventions.md`](_conventions.md)

---

## Element name

`x-stepper`

## Responsibility

Horizontal progress indicator showing the numbered steps of the customer booking
flow. Completed steps are clickable; the active and future steps are not.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `steps` | `string` | `"[]"` | JSON-encoded array of step label strings, e.g. `'["Dienst","Termin","Kontakt","Bestätigung","Fertig"]'`. |
| `current` | `number` | `1` | 1-based index of the currently active step. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `step-clicked` | `{ step: number }` | User clicks a *completed* step indicator. |

---

## Internal states per step item

| State | Condition |
| --- | --- |
| `completed` | Step index < `current` |
| `active` | Step index === `current` |
| `pending` | Step index > `current` |

---

## Testable behaviours

1. Renders exactly one labelled indicator element per entry in `steps`.
2. The indicator at index `current` has `aria-current="step"`.
3. Indicators with index < `current` are visually marked as completed (e.g. check icon) and have pointer cursor.
4. Indicators with index > `current` are visually muted and have `aria-disabled="true"`; clicking them does **not** dispatch `step-clicked`.
5. Clicking a completed (index < `current`) step indicator dispatches `step-clicked` with `{ step: N }` where `N` is the 1-based step number.
6. Clicking the active step does not dispatch any event.
7. On narrow viewports (< 480px) step labels are hidden and only the step number / icon is shown.
8. Updating `current` attribute re-renders the indicator states without destroying the DOM structure.

---

## Implementation hints

- Render step indicators as a `<ol>` (ordered list) for implicit semantics.
- Use `tabindex="0"` and `role="button"` on completed step indicators; omit or set `tabindex="-1"` on pending ones.
- Separate step indicator styles (circle + label) with flexbox row layout.
- Connecting lines between indicators can be CSS `::after` pseudo-elements.

---

## Minimal usage example

```html
<x-stepper
  steps='["Dienst","Termin","Kontakt","Bestätigung","Fertig"]'
  current="2">
</x-stepper>
```
