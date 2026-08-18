# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Cache module dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source tree
COPY . .

# Build statically linked API server binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -extldflags '-static'" \
    -o /app/bin/attendance-api \
    ./cmd/api/

# Build statically linked Migration binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -extldflags '-static'" \
    -o /app/bin/attendance-migrate \
    ./cmd/migrate/

# Build statically linked Seeding binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -extldflags '-static'" \
    -o /app/bin/attendance-seed \
    ./cmd/seed/

# ── Stage 2: Minimal Runtime ──────────────────────────────────────────────────
FROM alpine:3.19 AS runtime

WORKDIR /app

# Install SSL certificates, timezone data, curl, and postgresql-client
RUN apk add --no-cache ca-certificates tzdata curl postgresql-client && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup

# Copy statically compiled binaries and migration SQL assets
COPY --from=builder /app/bin/attendance-api /app/attendance-api
COPY --from=builder /app/bin/attendance-migrate /app/attendance-migrate
COPY --from=builder /app/bin/attendance-seed /app/attendance-seed
COPY --from=builder /app/internal/db/migrations /app/internal/db/migrations

# Set file permissions for non-root execution
RUN chown -R appuser:appgroup /app

USER appuser:appgroup

EXPOSE 8080

# Built-in Healthcheck targeting /healthz
HEALTHCHECK --interval=60s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/attendance-api"]
