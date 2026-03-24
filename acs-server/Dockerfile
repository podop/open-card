# ── Build stage ───────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /acs-server \
    ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────
FROM alpine:3.20

# Security: run as non-root
RUN addgroup -S acs && adduser -S acs -G acs

WORKDIR /app

# Copy binary and migrations
COPY --from=builder /acs-server .
COPY --chown=acs:acs migrations/ ./migrations/

USER acs

EXPOSE 8443

ENTRYPOINT ["./acs-server"]
CMD ["--config", "/app/config.yaml"]
