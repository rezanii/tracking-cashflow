---
title: Tracking Cashflow API
emoji: 💸
colorFrom: indigo
colorTo: blue
sdk: docker
app_port: 7860
pinned: false
short_description: Personal finance API with Telegram reporting
---

# Tracking Cashflow API

The Go API for [tracking-cashflow](https://github.com/rezanii/tracking-cashflow): personal
finance tracking with an accounts-based daily report, Excel and PDF exports, and a Telegram bot
that both reports and records.

This Space runs **only the API**. The web frontend is hosted separately on Netlify and reaches
this Space through a proxy, and the database is a managed Postgres (Neon) — no data is stored
in the Space.

## Endpoints

- `GET /health` — liveness
- `GET /swagger/index.html` — the full API reference, 26 paths
- `POST /api/v1/auth/login`, and everything else under `/api/v1`

## Configuration

Set these in **Settings → Variables and secrets**. Everything except `APP_PORT` is a secret.

| Name | Kind | Value |
| --- | --- | --- |
| `APP_PORT` | variable | `7860` — must match `app_port` above |
| `APP_ENV` | variable | `production` |
| `DATABASE_URL` | secret | the pooled Postgres URL, with `?sslmode=require` |
| `JWT_SECRET` | secret | 32+ random characters; production refuses anything shorter |
| `CORS_ALLOWED_ORIGINS` | variable | the frontend origin, e.g. `https://your-site.netlify.app` |
| `TELEGRAM_MODE` | variable | `off`, or `webhook` once the two below are set |
| `TELEGRAM_BOT_TOKEN` | secret | from @BotFather |
| `TELEGRAM_WEBHOOK_SECRET` | secret | a long random value |
| `TELEGRAM_WEBHOOK_URL` | variable | `https://<this-space>.hf.space/api/v1/telegram/webhook` |

`webhook` rather than polling, because a free Space sleeps when idle: a poller would simply
stop, while Telegram retries a webhook and the request wakes the Space.

Migrations are applied out of band against the database, not by this Space.
