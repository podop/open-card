# ── Build stage ───────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /secrets-manager \
    ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────
FROM alpine:3.20

RUN addgroup -S sm && adduser -S sm -G sm

WORKDIR /app

COPY --from=builder /secrets-manager .
COPY --chown=sm:sm migrations/ ./migrations/

USER sm

EXPOSE 8445

ENTRYPOINT ["./secrets-manager"]
