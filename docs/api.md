# API

Base URL: `http://localhost:8080/api/v1`

Swagger UI: `http://localhost:8080/swagger/index.html`

## Response envelope

Success:

```json
{ "success": true, "message": "Data retrieved successfully", "data": {} }
```

Error:

```json
{ "success": false, "message": "Validation failed", "errors": { "email": "Email is required" } }
```

Paginated `data`:

```json
{
  "items": [],
  "pagination": { "page": 1, "page_size": 10, "total_items": 125, "total_pages": 13 }
}
```

## Status codes

| Code | Used for |
| --- | --- |
| 200 | successful read, update, delete |
| 201 | resource created |
| 400 | malformed body or query |
| 401 | missing, invalid or expired token; wrong credentials |
| 403 | authenticated but not permitted |
| 404 | not found, or owned by another user |
| 409 | conflict: duplicate email, duplicate category, category still in use |
| 422 | validation failed |
| 429 | rate limit exceeded on auth endpoints |
| 500 | unexpected server error |

## Auth

| Method | Path | Auth | Body / Query |
| --- | --- | --- | --- |
| POST | `/auth/register` | no | `name`, `email`, `password` |
| POST | `/auth/login` | no | `email`, `password` |
| POST | `/auth/logout` | yes | — |
| GET | `/auth/me` | yes | — |

`POST /auth/register`

```json
{ "name": "John Doe", "email": "user@example.com", "password": "Admin123!" }
```

`POST /auth/login` → 200

```json
{
  "success": true,
  "message": "Login successful",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "token_type": "Bearer",
    "expires_at": "2026-09-26T12:00:00Z",
    "user": { "id": 1, "name": "John Doe", "email": "user@example.com" }
  }
}
```

Password policy: at least 8 characters, one upper case, one lower case, one digit.

Rate limit: 10 requests per minute per IP on `/auth/login` and `/auth/register`.

## Categories

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/categories` | query: `type`, `is_active`, `search`, `page`, `page_size`, `sort_by`, `sort_dir` |
| POST | `/categories` | `name`, `type`, `description` |
| GET | `/categories/{id}` | |
| PUT | `/categories/{id}` | `name`, `type`, `description` |
| PATCH | `/categories/{id}/status` | `is_active` |
| DELETE | `/categories/{id}` | 409 when transactions still reference it |

## Transactions

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/transactions` | filters below |
| POST | `/transactions` | |
| GET | `/transactions/{id}` | |
| PUT | `/transactions/{id}` | |
| DELETE | `/transactions/{id}` | |

List query parameters:

| Parameter | Example | Meaning |
| --- | --- | --- |
| `search` | `salary` | matches description or reference number |
| `date_from`, `date_to` | `2026-09-01` | inclusive range on `transaction_date` |
| `transaction_type` | `EXPENSE` | `INCOME`, `EXPENSE`, `TRANSFER` |
| `category_id` | `4` | |
| `page`, `page_size` | `1`, `10` | page size capped at 100 |
| `sort_by` | `transaction_date` | also `amount`, `created_at` |
| `sort_dir` | `desc` | `asc` or `desc` |

Create body:

```json
{
  "transaction_date": "2026-09-25",
  "transaction_type": "EXPENSE",
  "category_id": 5,
  "amount": "1150000.00",
  "description": "Cicilan Rumah September",
  "reference_number": "INV-0001"
}
```

`category_id` is required for `INCOME` and `EXPENSE`, and must be omitted or null for
`TRANSFER`. The category's `type` must match the transaction type.

## Dashboard

| Method | Path | Query |
| --- | --- | --- |
| GET | `/dashboard/summary` | `range` or `date_from` + `date_to` |

`range` accepts `today`, `week`, `month`, `year`, `custom`.

```json
{
  "success": true,
  "data": {
    "period": { "from": "2026-09-01", "to": "2026-09-30" },
    "total_income": 15000000,
    "total_expense": 8500000,
    "balance": 6500000,
    "transaction_count": 125,
    "income_vs_expense": [{ "label": "2026-09", "income": 15000000, "expense": 8500000 }],
    "expense_by_category": [{ "category_id": 5, "category_name": "Cicilan Rumah", "total": 1150000 }],
    "cash_flow_trend": [{ "date": "2026-09-25", "income": 15000000, "expense": 0, "balance": 15000000 }],
    "recent_transactions": []
  }
}
```

## Reports

| Method | Path | Returns |
| --- | --- | --- |
| GET | `/reports/cash-flow` | JSON rows with a running balance |
| GET | `/reports/cash-flow/excel` | `.xlsx` download |
| GET | `/reports/cash-flow/pdf` | `.pdf` download |
| GET | `/reports/summary` | totals for a period |
| GET | `/reports/expense-by-category` | totals grouped by category |
| GET | `/reports/monthly` | one row per month |

All report endpoints accept `date_from`, `date_to`, `category_id`, `transaction_type`.
`date_from` and `date_to` are required on `/reports/cash-flow*`.

`GET /reports/cash-flow` → 200

```json
{
  "success": true,
  "data": {
    "period": { "from": "2026-09-01", "to": "2026-09-30" },
    "total_income": 15000000,
    "total_expense": 8500000,
    "net_cash_flow": 6500000,
    "rows": [
      {
        "transaction_date": "2026-09-25",
        "transaction_type": "INCOME",
        "category_name": "Salary",
        "description": "Salary September",
        "income": 15000000,
        "expense": 0,
        "balance": 15000000
      }
    ]
  }
}
```

Export responses carry:

```
Content-Disposition: attachment; filename="cash-flow-2026-09-01-2026-09-30.xlsx"
```

The running `balance` is cumulative within the returned period and ordered by
`transaction_date`, then `id`, so the same filter always produces the same sequence.
