#!/bin/sh
# Bring the schema up to date and load development data, in that order. Used by the
# one-shot "migrate" service in docker compose.
set -eu

echo "applying migrations..."
/app/migrate -command up -path /app/migrations

if [ "${APP_ENV:-development}" = "production" ]; then
    echo "APP_ENV is production, skipping seed"
    exit 0
fi

echo "loading seed data..."
/app/seed
