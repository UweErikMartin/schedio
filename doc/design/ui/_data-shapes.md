# schedio – Shared Data Shapes

> **Import this file alongside any component spec** when it references data
> shapes such as `Service`, `Booking`, `Contact`, etc.

---

## Service

Returned by `GET /api/v1/services` and `GET /admin/api/v1/services`.

```json
{
  "id": "uuid",
  "name": "string",
  "summary": "string",
  "description": "string",
  "price": 49.50,
  "duration_minutes": 60,
  "daily_limit": 0
}
```

`daily_limit` of `0` means unlimited. The public endpoint omits `daily_limit`.

---

## BookingLine (session state)

Individual time slot within a session (step 2 state, not a persisted booking).

```json
{
  "id": "uuid",
  "start": "2026-03-15T10:00:00Z",
  "end": "2026-03-15T11:00:00Z",
  "service_id": "uuid"
}
```

As a JS object passed between components:

```js
{ id: string, value: string|null, minDate: string }
// value: ISO-8601 datetime "YYYY-MM-DDTHH:mm" or null
// minDate: ISO-8601 date "YYYY-MM-DD"
```

---

## Contact

Customer personal details.

```json
{
  "first_name": "string",
  "last_name": "string",
  "email": "user@example.de",
  "phone": "+49 123 4567890"
}
```

---

## Booking

Persisted booking record returned by customer management endpoints.

```json
{
  "id": "uuid",
  "session_id": "uuid",
  "service": { },
  "contact": { },
  "start_at": "2026-03-15T10:00:00Z",
  "end_at": "2026-03-15T11:00:00Z",
  "state": "reserved|confirmed|cancelled",
  "cancel_reason": "customer|admin|noshow|null",
  "location": "string"
}
```

`state` values and their display colours:

| state | colour token |
| --- | --- |
| `reserved` | `--color-tentative` (purple) |
| `confirmed` | `--color-success` (green) |
| `cancelled` | `--color-muted` (grey) |

---

## BookingSession (pending)

Returned inside `pending_sessions` of the dashboard response.

```json
{
  "id": "uuid",
  "service": { },
  "contact": { },
  "booking_count": 2,
  "earliest_booking_start": "2026-03-20T09:00:00Z",
  "submitted_at": "2026-03-14T08:30:00Z"
}
```

---

## Dashboard response

`GET /admin/api/v1/dashboard`

```json
{
  "bookings_today": [ ],
  "pending_sessions": [ ]
}
```

---

## Settings

`GET /admin/api/v1/settings` and `PUT /admin/api/v1/settings`

```json
{
  "no_show_deadline_hours": 24,
  "retention_period_days": 30,
  "currency": "EUR",
  "appointment_location": "Musterstraße 1, 10115 Berlin",
  "tandc_filename": "agb.pdf"
}
```

---

## Availability slot

Returned by `GET /api/v1/availability?service_id=&date=YYYY-MM-DD`

```json
[
  { "start": "2026-03-15T09:00:00Z", "end": "2026-03-15T10:00:00Z" },
  { "start": "2026-03-15T11:00:00Z", "end": "2026-03-15T12:00:00Z" }
]
```

---

## Auth response

`POST /auth/login` (JSON body `{ "email", "password" }`)

```json
{ "email": "admin@example.de", "role": "administrator" }
```

Roles: `staff`, `administrator`.

---

## Apple availability response

`GET /auth/apple/available?username=<email>`

```json
{ "apple_enabled": true }
```
