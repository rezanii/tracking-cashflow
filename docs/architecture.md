# Architecture

## Overview

Monorepo with two deployable apps and one database.

```
┌──────────────┐        HTTPS/JSON        ┌──────────────┐       TDS       ┌────────────┐
│  Next.js 15  │ ───────────────────────► │   Go / chi   │ ──────────────► │ SQL Server │
│  App Router  │  Bearer <access_token>   │   REST API   │      GORM       │    2022    │
└──────────────┘                          └──────────────┘                 └────────────┘
```

The frontend never talks to the database. Every read and write goes through the API,
which is the only component holding database credentials.

## Request flow

```
HTTP Request
  → Router          (chi, route table + per-route middleware)
  → Middleware      (request id, logger, recoverer, CORS, secure headers,
                     rate limit on /auth/*, JWT auth on protected routes)
  → Handler         (decode, validate, map DTO, write response envelope)
  → Service         (business rules, money arithmetic, transaction orchestration)
  → Repository      (GORM queries, always scoped by user_id)
  → SQL Server
```

Rules enforced by this layering:

- Handlers hold no business logic and no SQL. They parse, validate, delegate, respond.
- Services own calculation and orchestration. A service that needs more than one write
  opens a database transaction and passes it down to repositories.
- Repositories own persistence only. They take a `*gorm.DB` so the caller decides
  whether the work joins an existing transaction.
- `context.Context` is threaded from handler to repository for cancellation and deadlines.

## Layer responsibilities

| Package | Responsibility |
| --- | --- |
| `internal/config` | Load and validate `.env` into a typed struct. Fails fast on missing required values. |
| `internal/model` | GORM entities, table names, column mapping. |
| `internal/dto` | Request and response shapes. Never expose `password_hash`. |
| `internal/validator` | Struct validation, shared error shape for field errors. |
| `internal/repository` | Data access interfaces and GORM implementations. |
| `internal/service` | Business logic: auth, transaction, category, report. |
| `internal/handler` | HTTP adapters. |
| `internal/middleware` | Cross-cutting HTTP concerns. |
| `internal/router` | Route table and dependency wiring. |
| `internal/utils` | Response envelope, money helpers, JWT, date helpers, paging. |

Dependencies point inwards: handler → service → repository. No package imports a layer above it.

## Money handling

Money never touches `float64`. The chain is:

- Database: `DECIMAL(18,2)`
- Go: `github.com/shopspring/decimal`
- JSON: string-free numeric via `decimal.Decimal` marshalling, so no precision is lost in transit

`Balance = Total Income − Total Expense`. `TRANSFER` rows are recorded but excluded from
both totals, because a transfer moves money between accounts and is not a gain or a loss.

## Ownership

Every query that reads or writes user-owned data carries `WHERE user_id = ?` taken from the
JWT subject, never from the request body or path. Fetching by primary key alone is treated
as a bug: repository methods take both the id and the owning user id, so a mismatched pair
returns "not found" rather than another user's row.

## Authentication

- `POST /auth/register` hashes the password with bcrypt (cost 12) and stores only the hash.
- `POST /auth/login` verifies the hash and issues a signed JWT (HS256) carrying `sub`, `email`,
  `iat`, `exp`.
- `POST /auth/logout` is server-side stateless. The token is revoked by the client dropping it;
  see Known limitations in the README for why a deny list was not added.
- `GET /auth/me` returns the caller resolved from the token.

## Error handling

Services return typed errors (`ErrNotFound`, `ErrConflict`, `ErrUnauthorized`,
`ErrValidation`). Handlers translate those into HTTP status codes and a fixed envelope.
Database errors are logged with full detail server-side and reported to the client as a
generic 500, so driver messages and schema details never reach the browser.

## Dependencies

Backend:

| Module | Why |
| --- | --- |
| `github.com/go-chi/chi/v5` | Router |
| `github.com/go-chi/cors` | CORS |
| `github.com/go-chi/httprate` | Rate limit on auth endpoints |
| `gorm.io/gorm`, `gorm.io/driver/sqlserver` | ORM and SQL Server driver |
| `github.com/golang-jwt/jwt/v5` | JWT |
| `golang.org/x/crypto/bcrypt` | Password hashing |
| `github.com/go-playground/validator/v10` | Request validation |
| `github.com/shopspring/decimal` | Exact money arithmetic |
| `github.com/golang-migrate/migrate/v4` | Schema migration |
| `github.com/xuri/excelize/v2` | Excel export |
| `github.com/go-pdf/fpdf` | PDF export (maintained fork of gofpdf) |
| `github.com/joho/godotenv` | `.env` loading |
| `github.com/swaggo/swag`, `github.com/swaggo/http-swagger` | Swagger UI |
| `log/slog` | Structured logging, standard library |

Frontend:

| Package | Why |
| --- | --- |
| `next`, `react`, `typescript` | Framework |
| `tailwindcss` | Styling |
| `react-hook-form`, `zod`, `@hookform/resolvers` | Forms and schema validation |
| `axios` | HTTP client behind a typed service layer |
| `recharts` | Charts |
| `date-fns`, `date-fns-tz` | Date formatting in Asia/Jakarta |

## Why these choices

- **chi over a full framework**: the app needs routing, middleware and nothing else. chi is
  stdlib-compatible, so handlers stay `http.HandlerFunc`.
- **golang-migrate with plain SQL files** over GORM `AutoMigrate`: the schema is reviewable,
  versioned and reversible, and production never runs implicit DDL.
- **decimal over int64 cents**: the report layer sums and subtracts across categories; a
  decimal type keeps the arithmetic readable and the scale explicit.
- **slog over a logging library**: structured output with no dependency.

## Telegram integration

The bot is an add-on, not a dependency: with `TELEGRAM_MODE=off` the API serves exactly as
before, and a failure to reach Telegram at startup is logged while the HTTP surface keeps
running.

Two transports, one handler:

- **polling** (`TELEGRAM_MODE=polling`) runs `getUpdates` in a goroutine started by `cmd/api`.
  It needs no public URL, so it works on a laptop or behind NAT. The offset only advances
  after an update has been handled, so a crash re-delivers rather than loses it.
- **webhook** (`TELEGRAM_MODE=webhook`) takes updates on `POST /api/v1/telegram/webhook`.

They are mutually exclusive — Telegram refuses `getUpdates` while a webhook is registered — so
startup reconciles the two: polling mode deletes any webhook, webhook mode registers one.
Both paths hand the update to the same `HandleUpdate`, which is why the command behaviour is
tested once, without a network.

One consequence worth knowing: **a bot token cannot be shared with another polling process.**
Two pollers on one token terminate each other's long poll and Telegram answers
`409 Conflict`. One bot, one consumer.

### Why chats are paired rather than allow-listed

A chat id proves nothing: anyone can find a bot and message it. So the API issues a
short-lived single-use code to an authenticated caller, and the bot links the chat that sends
it back. The code is spent by an `UPDATE ... WHERE used_at IS NULL`, which makes the claim and
the check one atomic step, so two concurrent `/start` commands cannot both succeed. Codes come
from `crypto/rand`, because a guessable one would hand over an account's finances.

Until a chat is linked the bot answers nothing but "not linked", and a wrong code and an
expired code get the same reply, so the bot cannot be used to probe which codes exist.

### Rendering

The report is built as data (`dto.DailyCashFlowReport`) and rendered separately, so the same
report serves the JSON endpoint, the bot and any future channel. The renderer targets
MarkdownV2, where an unescaped `.` or `-` inside an amount makes Telegram reject the whole
message: every value is escaped before the bold markers are added, and a test walks the
rendered output asserting no special character is left bare. Messages are split on section
boundaries to stay under the 4096-character limit.
