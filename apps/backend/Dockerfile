# syntax=docker/dockerfile:1

# ---- build ----------------------------------------------------------------
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Dependencies are copied first so a code change does not invalidate the module cache.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is off so the binaries run on a scratch-like base. Symbols are stripped to keep
# the image small.
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /out/api     ./cmd/api \
 && go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate \
 && go build -trimpath -ldflags="-s -w" -o /out/seed    ./cmd/seed

# ---- tools ----------------------------------------------------------------
# Runs migrations and the seed. It keeps a shell because the entrypoint sequences
# two binaries.
FROM alpine:3.21 AS tools

RUN adduser -D -u 10001 appuser
WORKDIR /app

COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/seed    /app/seed
COPY migrations                  /app/migrations
COPY entrypoint.sh               /app/entrypoint.sh

RUN chmod +x /app/entrypoint.sh && chown -R appuser:appuser /app
USER appuser

ENTRYPOINT ["/app/entrypoint.sh"]

# ---- runtime --------------------------------------------------------------
FROM alpine:3.21 AS runtime

# tzdata lets the API resolve Asia/Jakarta; ca-certificates is needed for TLS.
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 appuser

WORKDIR /app

COPY --from=builder /out/api     /app/api
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/seed    /app/seed
COPY migrations                  /app/migrations

USER appuser

EXPOSE 8080

# Matches the compose healthcheck: the binary probes its own /health endpoint.
HEALTHCHECK --interval=15s --timeout=5s --retries=5 --start-period=10s \
    CMD ["/app/api", "-healthcheck"]

ENTRYPOINT ["/app/api"]
