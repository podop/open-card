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
    -o /identity-server \
    ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────
FROM alpine:3.20

RUN addgroup -S idp && adduser -S idp -G idp

WORKDIR /app

COPY --from=builder /identity-server .
COPY --chown=idp:idp migrations/ ./migrations/

USER idp

EXPOSE 8444

ENTRYPOINT ["./identity-server"]
