# ── Build stage ───────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

# Build both binaries
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /hsm-emulator \
    ./cmd/hsm-emulator

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /hsm-console \
    ./cmd/hsm-console

# ── Runtime stage ─────────────────────────────────────────────────
FROM alpine:3.20

RUN addgroup -S hsm && adduser -S hsm -G hsm

WORKDIR /app

COPY --from=builder /hsm-emulator .
COPY --from=builder /hsm-console .

# LMK and config are stored in /data (mount a volume for persistence)
RUN mkdir -p /data && chown hsm:hsm /data
VOLUME /data

USER hsm

# Host command port
EXPOSE 1500
# Console port
EXPOSE 2222
# Admin HTTP API port
EXPOSE 8086

# Default: start with test LMK, auto-authorize, header=2
ENTRYPOINT ["./hsm-emulator"]
CMD ["--test-lmk", "--auto-authorize", "--config", "/data/config.json", "--lmk", "/data/lmk.bin"]
