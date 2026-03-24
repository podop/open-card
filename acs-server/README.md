# ACS Server — EMV 3DS Access Control Server (Go)

A production-ready scaffold implementing the core ACS functions defined by EMVCo's
**EMV 3-D Secure Protocol and Core Functions Specification v2.2.0 and v2.3.1**.

---

## Architecture

Clean Architecture with four concentric layers:

```
cmd/server/              ← Entry point & dependency wiring
internal/
  domain/                ← Layer 1: Entities, Repository interfaces, Domain services
    entity/              ← Transaction, CardRange, ACSSession aggregates
    repository/          ← Port interfaces (TransactionRepository, etc.)
    service/             ← RiskEngine, OTPService, AuditLogger, AuthValueService
  usecase/               ← Layer 2: Application logic (AReq, 3DS Method, Challenge)
  adapter/               ← Layer 3: HTTP handlers/DTOs, PostgreSQL repo implementations
    http/
    repository/postgres/
  infrastructure/        ← Layer 4: Config, DB pool, Logger, Crypto
pkg/
  errors/                ← Domain error types
  validator/             ← Input validation helpers
migrations/              ← Goose SQL migrations
```

### Dependency rule
Inner layers have **zero** knowledge of outer layers.
The domain never imports `gin`, `sqlx`, or any infrastructure package.

---

## EMV 3DS Flows Implemented

### 1. Frictionless Flow  (AReq → ARes `Y`)
```
3DS Server ──POST /v1/auth──▶ ACS
           ◀── ARes(Y) ───────
```
The risk engine silently evaluates device + transaction data.
On low risk, an Authentication Value (CAVV) and ECI are generated immediately.

### 2. Challenge Flow  (AReq → ARes `C` → CReq/CRes)
```
3DS Server ──POST /v1/auth──────▶ ACS
           ◀── ARes(C, url) ──────
Browser    ──GET  /v1/challenge/:id▶ ACS  (OTP page)
           ──POST /v1/challenge/submit▶ ACS
           ◀── CRes(Y or N) ─────
```
OTPs are bcrypt-hashed before storage. Max 3 attempts; each failure rotates the OTP.

### 3. 3DS Method  (device fingerprinting, browser-based only)
```
Browser iframe ──POST /v1/method──────▶ ACS  (threeDSMethodData)
               ◀── self-posting HTML ──      (posts to notif URL)
               ──POST /v1/method/notification/:id▶ ACS
```
Runs silently before AReq. Collects IP, User-Agent, Accept headers.

---

## Quick Start

### Prerequisites
- Go 1.22+
- Docker & Docker Compose
- `make`

### 1. Start PostgreSQL + ACS
```bash
make docker-up
```

### 2. Run locally (without Docker)
```bash
# Generate dev keys
make generate-keys
# Edit config.yaml with the generated keys

# Start PostgreSQL only
docker compose up postgres -d

# Run ACS
make run
```

### 3. Run tests
```bash
make test
```

### 4. Apply migrations manually
```bash
make migrate-up
```

---

## Configuration

All settings can be provided via `config.yaml` **or** environment variables
(prefix `ACS_`, dots become underscores):

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `server.port` | `ACS_SERVER_PORT` | `8443` | HTTP listen port |
| `server.tls_cert_file` | `ACS_SERVER_TLS_CERT_FILE` | — | TLS cert path |
| `database.dsn` | `ACS_DATABASE_DSN` | — | PostgreSQL DSN |
| `acs.base_url` | `ACS_ACS_BASE_URL` | — | Public ACS URL |
| `acs.reference_number` | `ACS_ACS_REFERENCE_NUMBER` | — | EMVCo Letter of Approval number |
| `crypto.master_key_hex` | `ACS_CRYPTO_MASTER_KEY_HEX` | — | 32-byte AES key (hex) for CAVV |
| `crypto.pan_encryption_key_hex` | `ACS_CRYPTO_PAN_ENCRYPTION_KEY_HEX` | — | 32-byte AES key for PAN at rest |

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/auth` | AReq → ARes (called by DS) |
| `POST` | `/v1/method` | 3DS Method device fingerprint |
| `POST` | `/v1/method/notification/:sessionID` | 3DS Method completion callback |
| `GET`  | `/v1/challenge/:acsTransID` | Render challenge page |
| `POST` | `/v1/challenge/submit` | Submit OTP |
| `GET`  | `/health` | Health check |

---

## Key Extension Points

| Interface | Location | Purpose |
|-----------|----------|---------|
| `service.RiskEngine` | `internal/domain/service/risk_engine.go` | Plug in ML/rules engine |
| `service.AuthValueService` | `internal/domain/service/risk_engine.go` | CAVV algorithm (replace HMAC stub with card-scheme algorithm) |
| `service.AuditLogger` | `internal/domain/service/audit.go` | Swap PostgreSQL for event stream |
| `repository.TransactionRepository` | `internal/domain/repository/interfaces.go` | Add Redis caching layer |

---

## Production Checklist

- [ ] Replace `DefaultRiskEngine` with production risk scoring
- [ ] Replace `HMACAuthValueService` with card-scheme-mandated CAVV algorithm
- [ ] Configure TLS (mutual TLS with DS strongly recommended)
- [ ] Load crypto keys from HSM or secrets manager (never from config files)
- [ ] Enable PostgreSQL SSL (`sslmode=require`)
- [ ] Set up `expire_stale_sessions()` as a scheduled job (pg_cron or cron)
- [ ] Complete EMVCo functional compliance testing
- [ ] Complete PCI 3DS security assessment
- [ ] Obtain EMVCo Letter of Approval before connecting to production DS

---

## License
Replace with your organization's license.
