# Component spec: `<x-secret-manager>`

> **Output file:** `web/js/admin/secret-manager.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`x-dialog.md`](x-dialog.md),
> [`x-spinner.md`](x-spinner.md), [`x-error-banner.md`](x-error-banner.md),
> [`x-toast.md`](x-toast.md)

---

## Element name

`x-secret-manager`

## Responsibility

Admin interface for rotating the HMAC signing secret used to generate and
verify customer management-link tokens. Displays the creation date of the
current secret and provides a guarded rotation action.

---

## Observed attributes

None.

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `GET` | `/api/v1/admin/settings/secret` | On `connectedCallback` (retrieves metadata only, never the raw secret) |
| `POST` | `/api/v1/admin/settings/secret/rotate` | Admin confirms rotation in the warning dialog |

---

## Internal states

| State | Description |
| --- | --- |
| `loading` | Fetching secret metadata. |
| `ready` | Metadata shown; rotation button available. |
| `confirm-dialog` | `<x-dialog>` open with rotation warning. |
| `rotating` | API call in flight; dialog spinner shown. |
| `error` | Fetch or rotate failed. |

---

## Testable behaviours

1. On mount, fetches secret metadata (e.g. creation timestamp) and displays it.
2. The raw secret value is **never** displayed or requested.
3. "Geheimnis rotieren" button opens a warning dialog (`<x-dialog>`) explaining that all existing management links will be invalidated.
4. Dialog has "Abbrechen" and "Jetzt rotieren" buttons.
5. "Abbrechen" closes the dialog without calling the API.
6. "Jetzt rotieren" calls `POST .../rotate`; on success shows a `<x-toast>` "Geheimnis wurde rotiert", closes the dialog, and refreshes the metadata.
7. On rotation error, shows `<x-error-banner>` inside the dialog and re-enables buttons.
8. During rotation, both dialog buttons are disabled.

---

## Minimal usage example

```html
<x-secret-manager></x-secret-manager>
```
