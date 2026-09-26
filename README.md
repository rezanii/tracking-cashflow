# Tracking Cashflow

Personal finance tracking: a Go API, a Next.js frontend, PostgreSQL, and a Telegram bot that
reports and records from a chat.

Documentation lives in [`docs/`](docs/):

| Document | Covers |
| --- | --- |
| [docs/README.md](docs/README.md) | everything: architecture, ERD, API, running it, tests, the daily report and the bot, deploying |
| [docs/architecture.md](docs/architecture.md) | layers, request flow, money handling, ownership, the Telegram integration |
| [docs/database.md](docs/database.md) | schema, constraints, indexes, account types, migrations |
| [docs/api.md](docs/api.md) | endpoints, response envelope, status codes, bot commands |
| [docs/PRD.md](docs/PRD.md) | the original requirements |

## Quick start

```bash
cp .env.example .env    # set DB_PASSWORD and JWT_SECRET
docker compose up -d
```

Frontend on `FRONTEND_HOST_PORT` (3000 by default), API on `APP_HOST_PORT` (8080), Swagger at
`/swagger/index.html`. The seed creates a development account; see
[docs/README.md](docs/README.md) for its credentials.

Deploying: Netlify for the frontend, Vercel or a Hugging Face Docker Space for the API, Neon for the
database — see [§14 Deploying](docs/README.md#14-deploying).
