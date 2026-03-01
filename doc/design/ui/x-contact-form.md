# Component spec: `<x-contact-form>`

> **Output file:** `web/js/booking/x-contact-form.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md)

---

## Element name

`x-contact-form`

## Responsibility

Input form for customer contact details in booking flow step 3. Validates all
fields client-side using the Constraint Validation API before emitting the
confirmed event.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `disabled` | `boolean` | `false` | When present, all inputs have `readonly` and the submit button is hidden. Used when showing the summary in step 4. |

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `value` | `Contact \| null` | When set, pre-fills all form fields. Used when the user navigates back to step 3. See `_data-shapes.md`. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `contact-confirmed` | `{ contact: Contact }` | User submits the form and all fields pass validation. |

---

## Internal states

| State | Description |
| --- | --- |
| `idle` | Form ready for input; no validation errors shown. |
| `invalid` | At least one field failed validation; field-level error messages displayed. |

---

## Form fields

| Field | HTML input type | Required | Validation |
| --- | --- | --- | --- |
| Vorname (first name) | `text` | ✅ | Non-empty |
| Nachname (last name) | `text` | ✅ | Non-empty |
| E-Mail | `email` | ✅ | `type="email"` pattern |
| Telefon | `tel` | ✅ | Regexp: `^\+?[\d\s\-().]{7,20}$` |

---

## Testable behaviours

1. Renders labelled inputs for Vorname, Nachname, E-Mail, and Telefon.
2. Attempting to submit with any empty required field shows field-level error messages and does **not** dispatch `contact-confirmed`.
3. E-mail field uses `type="email"`; invalid e-mail format shows a validation error.
4. Phone field is validated against a loose regexp allowing international (`+49 123 …`) and local formats.
5. Error messages are associated with their input via `aria-describedby` and use `role="alert"` for announcement.
6. On successful validation, dispatches `contact-confirmed` with `{ contact: { first_name, last_name, email, phone } }`.
7. When `value` JS property is set, all four fields are pre-populated.
8. When `disabled` attribute is present, all inputs have `readonly` attribute and the submit button is absent.
9. Tab order: Vorname → Nachname → E-Mail → Telefon → submit button.
10. On `≥ 480px` viewports, Vorname and Nachname appear side-by-side (two-column grid); E-Mail and Telefon fill the full width.

---

## Implementation hints

- Use `form.reportValidity()` to trigger native browser validation UI, then supplement with custom error rendering.
- For the phone regexp: `/^\+?[\d\s\-().]{7,20}$/` — set as `pattern` attribute on the `<input type="tel">`.
- Two-column grid: `display: grid; grid-template-columns: 1fr 1fr` with a media query at 480px.

---

## Minimal usage example

```html
<x-contact-form></x-contact-form>
```

```js
contactForm.addEventListener('contact-confirmed', e => {
  console.log(e.detail.contact); // { first_name, last_name, email, phone }
});
```
