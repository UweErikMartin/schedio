# schedio – Web Component Conventions

> **Import this file alongside any component spec** when generating
> implementations, so the AI has the full technology contract.

---

## Technology rules

| Concern | Rule |
| --- | --- |
| Language | Vanilla JavaScript (ES2022+). No TypeScript, no npm packages. |
| Component model | Custom Elements v1 — `class Foo extends HTMLElement`, `customElements.define('x-foo', Foo)` |
| Shadow DOM | Always `this.attachShadow({ mode: 'open' })` in the constructor |
| Templates | Use a `<template>` element cloned with `content.cloneNode(true)` in `connectedCallback` |
| Module system | ES modules (`export default class …`). One component per file. |
| HTTP | `fetch` + `async`/`await`. Never use XMLHttpRequest. |
| Routing | `history.pushState` / `popstate` — only in top-level app components |
| i18n | `Intl.DateTimeFormat` and `Intl.NumberFormat` using `navigator.language` |
| CSS | Plain CSS inside the shadow root only. No inline style attributes. |
| No frameworks | React, Vue, Lit, Svelte, Angular — all forbidden |

---

## CSS design tokens

Every component imports the host application's token file inside its shadow
root via:

```js
const sheet = new CSSStyleSheet();
sheet.replaceSync(`@import '/css/tokens.css';`);
this.shadowRoot.adoptedStyleSheets = [sheet, componentSheet];
```

Or equivalently inside the `<template>` style block:

```html
<style>
  @import '/css/tokens.css';
  /* component styles here */
</style>
```

### Colour palette

| Token | Default value | Use |
| --- | --- | --- |
| `--color-bg` | `#f9fafb` | Page background |
| `--color-surface` | `#ffffff` | Card / elevated surface |
| `--color-border` | `#d1d5db` | Input borders |
| `--color-text` | `#1f2937` | Body text |
| `--color-muted` | `#6b7280` | Secondary / placeholder text |
| `--color-primary` | `#0f62fe` | Primary action colour |
| `--color-primary-hover` | `#0353d9` | Primary action — hover state |
| `--color-danger` | `#dc2626` | Error state, destructive action |
| `--color-warning` | `#f59e0b` | Warning / no-show state |
| `--color-success` | `#16a34a` | Success / confirmed state |
| `--color-tentative` | `#7c3aed` | Reserved / tentative state |

### Typography

| Token | Default value |
| --- | --- |
| `--font-family` | `Inter, "Segoe UI", system-ui, -apple-system, sans-serif` |
| `--font-size-sm` | `0.875rem` |
| `--font-size-base` | `1rem` |
| `--font-size-lg` | `1.125rem` |
| `--font-size-xl` | `1.25rem` |
| `--font-weight-normal` | `400` |
| `--font-weight-medium` | `500` |
| `--font-weight-bold` | `600` |

### Spacing and layout

| Token | Default value |
| --- | --- |
| `--space-xs` | `0.25rem` |
| `--space-sm` | `0.5rem` |
| `--space-md` | `1rem` |
| `--space-lg` | `1.5rem` |
| `--space-xl` | `2rem` |
| `--radius-sm` | `6px` |
| `--radius-md` | `10px` |
| `--radius-lg` | `16px` |
| `--shadow-card` | `0 1px 4px rgba(0,0,0,.08)` |

### Focus ring

Never remove the browser focus outline without replacement. Use:

```css
:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
```

---

## Attribute conventions

- Observed attributes are always **lowercase strings** in the DOM.
- Boolean attributes follow the HTML boolean convention: presence = `true`,
  absence = `false`. Use `this.hasAttribute('name')` to read, and
  `this.toggleAttribute('name', bool)` to write.
- Complex data (arrays, objects) is passed via **JS properties only**, never via
  attributes.
- Attribute↔property reflection: every observed attribute has a matching JS
  getter/setter.

## Event conventions

- All custom events use `new CustomEvent(name, { bubbles: true, composed: true, detail: … })`.
- `detail` is always a plain object (never a primitive).
- Event names are kebab-case.

## Accessibility baseline

- Every interactive element must be keyboard-reachable and operable.
- Minimum touch target: 44 × 44 px.
- Error messages: `role="alert"` or associated via `aria-describedby`.
- Focus must never be lost silently (e.g. when a dialog closes, return it to the trigger).
- Colour contrast: all text/background pairs ≥ 4.5:1 (WCAG 2.1 AA).

## UI language

All user-visible labels are in **German**.

---

## Responsive breakpoints

| Name | Min-width | Notes |
| --- | --- | --- |
| `sm` | `480px` | Two-column contact form |
| `md` | `768px` | Wider cards, inline stepper |
| `lg` | `1024px` | Admin two-column sidebar layout |

---

## File placement in the host project

```text
web/js/
  booking/        ← customer booking SPA components
  manage/         ← customer management view components
  shared/         ← utility components used everywhere
  admin/          ← admin SPA components
web/css/
  tokens.css      ← design tokens (imported inside every shadow root)
```
