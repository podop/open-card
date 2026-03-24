# ── Build stage ───────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /ds-server \
    ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────
FROM alpine:3.20

RUN addgroup -S ds && adduser -S ds -G ds

WORKDIR /app

COPY --from=builder /ds-server .

USER ds

EXPOSE 9443

ENTRYPOINT ["./ds-server"]