# Component spec: `<x-admin-app>`

> **Output file:** `web/js/admin/admin-app.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md),
> [`x-admin-nav.md`](x-admin-nav.md), [`x-login-form.md`](x-login-form.md),
> [`x-admin-dashboard.md`](x-admin-dashboard.md), [`x-service-list.md`](x-service-list.md),
> [`x-service-form.md`](x-service-form.md), [`x-settings-form.md`](x-settings-form.md),
> [`x-tandc-upload.md`](x-tandc-upload.md), [`x-secret-manager.md`](x-secret-manager.md),
> [`x-spinner.md`](x-spinner.md), [`x-toast.md`](x-toast.md)

---

## Element name

`x-admin-app`

## Responsibility

Top-level orchestrator for the admin single-page application at `/admin`. Manages
authentication state, client-side routing, and renders the appropriate admin panel
component for each route.

---

## Observed attributes

None. All state is internal.

---

## JS properties

| Property | Type | Description |
| --- | --- | --- |
| `route` | `string` | Current virtual route (read-only). |
| `user` | `{ email: string, role: string } \| null` | Authenticated user; `null` when logged out. |

---

## Internal states

| State | Description |
| --- | --- |
| `checking-auth` | Verifying cookie session on load; full-screen spinner shown. |
| `logged-out` | No valid session; `<x-login-form>` shown. |
| `logged-in` | Valid session; `<x-admin-nav>` + routed panel shown. |

---

## Routes and panels

| Route | Panel component |
| --- | --- |
| `/admin` or `/admin/dashboard` | `<x-admin-dashboard>` |
| `/admin/services` | `<x-service-list>` |
| `/admin/services/new` | `<x-service-form>` (create mode) |
| `/admin/services/:id` | `<x-service-form>` (edit mode) |
| `/admin/settings` | `<x-settings-form>` |
| `/admin/settings/tandc` | `<x-tandc-upload>` |
| `/admin/settings/secret` | `<x-secret-manager>` |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `GET` | `/auth/me` | On `connectedCallback` to verify existing session |
| `POST` | `/auth/logout` | User clicks logout in `<x-admin-nav>` |

---

## Testable behaviours

1. On `connectedCallback`, calls `GET /auth/me`; shows spinner while waiting.
2. If `/auth/me` returns 401, transitions to `logged-out` state.
3. If `/auth/me` returns 200, transitions to `logged-in` and renders the nav + the panel matching the current URL path.
4. When `<x-login-form>` emits `login-success`, transitions to `logged-in` using the returned user data, then routing to `/admin/dashboard`.
5. When `<x-admin-nav>` emits `nav-logout`, calls `POST /auth/logout` and transitions to `logged-out`.
6. Uses `history.pushState` for navigation; listens to `popstate` to sync the `route` property.
7. On unknown routes, renders the dashboard.
8. When `<x-service-form>` emits `service-saved` or `service-cancelled`, navigates back to `/admin/services`.

---

## Minimal usage example

```html
<!-- Served at /admin -->
<x-admin-app></x-admin-app>
```
