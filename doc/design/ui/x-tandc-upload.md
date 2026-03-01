# Component spec: `<x-tandc-upload>`

> **Output file:** `web/js/admin/tandc-upload.js`
> **Dependencies:** [`_conventions.md`](_conventions.md), [`x-spinner.md`](x-spinner.md),
> [`x-error-banner.md`](x-error-banner.md), [`x-toast.md`](x-toast.md)

---

## Element name

`x-tandc-upload`

## Responsibility

Admin interface for uploading and replacing the Terms & Conditions PDF that
customers must accept before confirming a booking.

---

## Observed attributes

None.

---

## API calls

| Method | URL | When |
| --- | --- | --- |
| `GET` | `/api/v1/admin/tandc` | On mount (check whether a T&C file already exists) |
| `PUT` | `/api/v1/admin/tandc` | Admin submits a new PDF file (multipart/form-data) |

---

## Internal states

| State | Description |
| --- | --- |
| `loading` | Fetching current T&C metadata. |
| `no-file` | No T&C PDF uploaded yet; upload area shown. |
| `has-file` | Existing file metadata shown (filename, upload date); replace button shown. |
| `uploading` | Upload in flight; progress bar or spinner. |
| `error` | Fetch or upload failed. |

---

## Testable behaviours

1. On mount, checks for an existing T&C file and transitions to `has-file` or `no-file`.
2. File input accepts only `application/pdf`.
3. Selecting a non-PDF file shows a validation error and clears the selection.
4. Drag-and-drop onto the upload area populates the file input.
5. Upload limit: 10 MB; files larger than this show an error before calling the API.
6. During upload, shows progress (either spinner or `<progress>` element).
7. On successful upload, transitions to `has-file` and shows a `<x-toast>` "AGB erfolgreich hochgeladen".
8. On upload error, shows `<x-error-banner>` with the server message.
9. In `has-file`, a "Vorschau" link opens the current PDF in a new tab via `GET /api/v1/tandc`.

---

## Minimal usage example

```html
<x-tandc-upload></x-tandc-upload>
```
