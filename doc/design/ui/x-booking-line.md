# Component spec: `<x-booking-line>`

> **Output file:** `web/js/booking/x-booking-line.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`x-date-time-picker.md`](x-date-time-picker.md)

---

## Element name

`x-booking-line`

## Responsibility

Represents one booking line within the session: wraps one `<x-date-time-picker>`
and, when removable, a remove button. Propagates picker events upward with its
own `line-id`.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `line-id` | `string` | `""` | Unique identifier matching the server booking-line record. Included in all emitted events. |
| `service-id` | `string` | `""` | Propagated verbatim to the inner `<x-date-time-picker>`. |
| `min-date` | `string` | `""` | ISO-8601 date (e.g. `2026-03-15`). Propagated to the inner picker. |
| `removable` | `boolean` | `false` | When present, the remove button is rendered. When absent, **no** remove button exists in the DOM. |

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `value` | `string \| null` | ISO-8601 datetime of the currently selected slot. Setting this propagates to the inner picker's `value` property. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `line-changed` | `{ lineId: string, value: string }` | The inner picker dispatched `date-time-selected`. |
| `line-remove-requested` | `{ lineId: string }` | User clicks the remove button. |

---

## Testable behaviours

1. Renders an `<x-date-time-picker>` with `service-id` and `min-date` propagated correctly from this component's attributes.
2. When the inner `<x-date-time-picker>` dispatches `date-time-selected`, this component dispatches `line-changed` with `{ lineId, value }`.
3. The remove button is rendered **only** when the `removable` attribute is present; when absent the button does not exist in the DOM at all.
4. Clicking the remove button dispatches `line-remove-requested` with `{ lineId: this.getAttribute('line-id') }`.
5. Setting the `value` JS property updates the inner picker's `value` property.
6. The component renders a visible heading (e.g. "Termin 1") derived from its ordinal position; this heading is provided by the parent `<x-booking-list>` updating an attribute or slot — the component renders whatever heading text it receives.

---

## Implementation hints

- Listen to `date-time-selected` and `date-time-cleared` on the inner picker element (not via event bubbling through the shadow boundary).
- `removable` is a boolean attribute: use `this.hasAttribute('removable')` to read it.

---

## Minimal usage example

```html
<x-booking-line
  line-id="line-abc"
  service-id="svc-001"
  min-date="2026-03-15"
  removable>
</x-booking-line>
```
