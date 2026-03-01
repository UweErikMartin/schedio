# Component spec: `<x-login-form>`

> **Output file:** `web/js/admin/login-form.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`x-spinner.md`](x-spinner.md),
> [`x-error-banner.md`](x-error-banner.md)

---

## Element name

`x-login-form`

## Responsibility

E-mail + password login form for the admin SPA. Submits credentials to the auth
API and emits a success event so the parent can transition to the authenticated
state.

---

## Observed attributes

None.

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `login-success` | `{ user: { email: string, role: string } }` | Login API returned 200; session cookie is now set. |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `POST` | `/auth/login` | User submits the form |

Request body (JSON): `{ "email": string, "password": string }`

---

## Internal states

| State | Description |
| --- | --- |
| `idle` | Form displayed; no error. |
| `submitting` | API call in flight; submit button replaced by `<x-spinner>`. |
| `error-credentials` | Server returned 401; "Ungültige E-Mail-Adresse oder Passwort" shown. |
| `error-server` | Server returned 5xx or network failure; generic error shown. |

---

## Testable behaviours

1. Renders an e-mail input (`type="email"`, `autocomplete="email"`) and a password input (`type="password"`, `autocomplete="current-password"`).
2. Submitting with an empty e-mail or password shows HTML5 validation messages and does not call the API.
3. On submit, transitions to `submitting` and disables both inputs.
4. On HTTP 200, dispatches `login-success` with the user object from the response body.
5. On HTTP 401, transitions to `error-credentials` with the German error message.
6. On HTTP 5xx or network failure, transitions to `error-server`.
7. After any error, inputs are re-enabled for correction.
8. The password input has a show/hide toggle button with `aria-pressed` set appropriately.

---

## Minimal usage example

```html
<x-login-form></x-login-form>
```
