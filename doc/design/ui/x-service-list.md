# Component spec: `<x-service-list>`

> **Output file:** `web/js/admin/service-list.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`_data-shapes.md`](_data-shapes.md),
> [`x-spinner.md`](x-spinner.md), [`x-error-banner.md`](x-error-banner.md)

---

## Element name

`x-service-list`

## Responsibility

Admin view listing all services in the catalogue. Provides buttons to create a
new service or edit/delete an existing one.

---

## Observed attributes

None.

---

## Custom events

| Event | `detail` | When dispatched |
| --- | --- | --- |
| `service-create-requested` | `{}` | Admin clicks "Neuen Dienst erstellen". |
| `service-edit-requested` | `{ serviceId: string }` | Admin clicks "Bearbeiten" on a service row. |

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `GET` | `/api/v1/admin/services` | On `connectedCallback` and after a delete |
| `DELETE` | `/api/v1/admin/services/{id}` | Admin confirms deletion via an inline confirmation |

---

## Internal states

| State | Description |
| --- | --- |
| `loading` | Fetching service list; spinner shown. |
| `error` | Fetch failed. |
| `empty` | No services; "Noch keine Dienste angelegt" with "Erstellen" button. |
| `list` | Table of services. |
| `deleting` | DELETE in flight for one row; that row shows a spinner; others are inert. |

---

## Displayed columns

| Column | Source |
| --- | --- |
| Name | `service.name` |
| Dauer | `service.duration_minutes` min |
| Preis | `service.price` formatted with `Intl.NumberFormat` |
| Aktionen | "Bearbeiten" button + inline delete with confirmation |

---

## Testable behaviours

1. Fetches and renders services on mount.
2. "Neuen Dienst erstellen" dispatches `service-create-requested`.
3. "Bearbeiten" dispatches `service-edit-requested` with `{ serviceId }`.
4. Clicking "Löschen" shows an inline "Wirklich löschen?" confirmation in the same row.
5. Confirming deletion calls `DELETE /api/v1/admin/services/{id}`; on success re-fetches; on error shows `<x-error-banner>`.
6. Cancelling the deletion restores the row to its normal state.
7. During a delete, all other action buttons are disabled.

---

## Minimal usage example

```html
<x-service-list></x-service-list>
```
