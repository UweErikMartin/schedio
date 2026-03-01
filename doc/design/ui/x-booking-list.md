# Component spec: `<x-booking-list>`

> **Output file:** `web/js/booking/x-booking-list.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`x-booking-line.md`](x-booking-line.md)

---

## Element name

`x-booking-list`

## Responsibility

Container for one or more `<x-booking-line>` elements in booking flow step 2.
Manages the ordered list, determines remove-button visibility, and provides a
"Weiteren Termin hinzufügen" button.

---

## Observed attributes

None.

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `lines` | `Array<{ id: string, value: string\|null, minDate: string }>` | Current booking lines. Each change re-renders the list. |
| `serviceId` | `string` | Passed to each child `<x-booking-line>` as `service-id`. |
| `canAddMore` | `boolean` | When `true`, the "add" button is visible. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `line-add-requested` | `{}` | User clicks "Weiteren Termin hinzufügen". |
| `line-remove-requested` | `{ lineId: string }` | Re-dispatched from a child `<x-booking-line>`. |
| `line-changed` | `{ lineId: string, value: string }` | Re-dispatched from a child `<x-booking-line>`. |

---

## Testable behaviours

1. Renders exactly one `<x-booking-line>` per entry in `lines`.
2. The remove button on a line is visible (`removable` attribute set) **only** when there is more than one line. The sole remaining line never shows a remove button.
3. "Weiteren Termin hinzufügen" button is visible only when `canAddMore` is `true`.
4. Clicking "add" dispatches `line-add-requested`.
5. When a child dispatches `line-remove-requested`, this component re-dispatches it with the same `detail`.
6. When a child dispatches `line-changed`, this component re-dispatches it with the same `detail`.
7. When `lines` is updated, existing `<x-booking-line>` instances whose `id` is unchanged are reused (not recreated) to preserve their internal picker state.
8. The `min-date` for each line is: today for the first line; the end time of the preceding line's selected slot for subsequent lines (or today if the preceding line has no selection yet).
9. On initial render of step 2, the first `<x-booking-line>`'s `value` property is pre-set to the earliest available slot (provided by the parent via the `lines` array).

---

## Implementation hints

- Diff the `lines` array by `id` when re-rendering to avoid destroying unchanged picker state.
- Pass `min-date` and `service-id` as attributes to each `<x-booking-line>`.
- Set the `removable` attribute on all lines except when `lines.length === 1`.
- Listen to `line-remove-requested` and `line-changed` on the container (event bubbles through `composed: true`).

---

## Minimal usage example

```js
const list = document.createElement('x-booking-list');
list.serviceId = 'svc-001';
list.canAddMore = true;
list.lines = [{ id: 'l1', value: null, minDate: '2026-03-15' }];
document.body.appendChild(list);
```
