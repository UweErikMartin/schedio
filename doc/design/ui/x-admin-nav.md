# Component spec: `<x-admin-nav>`

> **Output file:** `web/js/admin/admin-nav.js`
> **Dependencies:** [`_conventions.md`](_conventions.md)

---

## Element name

`x-admin-nav`

## Responsibility

Side-navigation bar for the admin SPA. Highlights the active route and provides
a logout button.

---

## Observed attributes

| Attribute | Type | Default | Description |
| --- | --- | --- | --- |
| `active-route` | `string` | `""` | Current route path (e.g. `/admin/services`). Sets the active link. |
| `user-email` | `string` | `""` | Displayed under the logo as the signed-in user's e-mail. |

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `nav-clicked` | `{ href: string }` | User clicks a navigation link. |
| `nav-logout` | `{}` | User clicks "Abmelden". |

---

## Navigation items

| Label | `href` |
| --- | --- |
| Dashboard | `/admin/dashboard` |
| Dienste | `/admin/services` |
| Einstellungen | `/admin/settings` |

---

## Testable behaviours

1. Renders a logo/brand mark at the top of the sidebar.
2. Renders navigation links for Dashboard, Dienste, and Einstellungen.
3. The link whose `href` matches `active-route` has `aria-current="page"` and visible active styling.
4. Clicking a nav link dispatches `nav-clicked` with `{ href }` and does **not** trigger a full page navigation.
5. `user-email` is displayed below the logo.
6. "Abmelden" button dispatches `nav-logout`.
7. On mobile (< 768 px) the sidebar collapses to icon-only; a hamburger toggle button reveals the full labels.

---

## Minimal usage example

```html
<x-admin-nav active-route="/admin/dashboard" user-email="admin@example.de"></x-admin-nav>
```
