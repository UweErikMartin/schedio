# Component spec: `<x-dialog>`

> **Output file:** `web/js/shared/x-dialog.js`
> **Dependencies:** [`_conventions.md`](_conventions.md)

---

## Element name

`x-dialog`

## Responsibility

Accessible modal dialog wrapper using the native `<dialog>` element. Exposes
`open()` / `close()` methods. Body content is placed in the default slot
so the caller controls what appears inside the dialog.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `dialog-title` | `string` | `""` | Visible heading rendered inside the dialog as `<h2>`. |
| `close-label` | `string` | `Schließen` | `aria-label` for the × close button. |

---

## JS properties

Reflected from the observed attributes (no additional properties).

---

## JS methods

| Method | Signature | Description |
| --- | --- | --- |
| `open(triggerElement?)` | `open(el?: HTMLElement): void` | Calls `showModal()` on the inner `<dialog>`. Traps focus. Optionally records the trigger element to restore focus on close. |
| `close()` | `close(): void` | Closes the dialog, returns focus to the trigger element, and dispatches `dialog-closed`. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `dialog-closed` | `{}` | Dialog is hidden by any means (close button, Escape key, or backdrop click). |

---

## Slots

| Slot | Description |
| --- | --- |
| *(default)* | Content rendered inside the dialog body, below the heading. |

---

## Testable behaviours

1. Before `open()` is called, the dialog is not visible.
2. Calling `open()` calls `showModal()` and the dialog becomes visible with a backdrop.
3. Focus moves to the first focusable element inside the dialog after `open()`.
4. Tab key cycles focus within the dialog; focus does not escape to the page behind. *(Focus trap)*
5. Pressing Escape calls `close()` and dispatches `dialog-closed`.
6. Clicking the backdrop (outside the dialog content box) calls `close()`.
7. Clicking the × button calls `close()` and dispatches `dialog-closed`.
8. After `close()`, focus returns to the element passed to `open(triggerElement)`.
9. The value of `dialog-title` is rendered as an `<h2>` inside the dialog.
10. `aria-modal="true"` is set on the `<dialog>` element.

---

## Implementation hints

- Use the native `<dialog>` element; it provides correct ARIA role automatically.
- For focus trapping: listen to `keydown` and intercept Tab/Shift+Tab to cycle between focusable elements.
- Intercept the native `cancel` event (Escape key) to route through the component's `close()` method so `dialog-closed` is always dispatched.
- Backdrop click: compare `event.target === dialogElement` (clicking the padding area).

---

## Minimal usage example

```html
<x-dialog dialog-title="Termin absagen">
  <p>Soll der Termin wirklich abgesagt werden?</p>
  <button id="confirm">Ja, absagen</button>
  <button id="cancel">Abbrechen</button>
</x-dialog>
```

```js
const dlg = document.querySelector('x-dialog');
document.querySelector('#open-btn').addEventListener('click', e => dlg.open(e.currentTarget));
document.querySelector('#cancel').addEventListener('click', () => dlg.close());
```
