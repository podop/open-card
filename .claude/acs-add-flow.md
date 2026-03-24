---
name: acs-add-flow
description: >
  Use this skill whenever a developer working on the acs-server project wants to add, extend,
  or modify a 3DS authentication flow. This includes adding Decoupled Authentication (transStatus D),
  3DS Requestor-Initiated (3RI) flows, new challenge types (OOB, biometric, static password),
  new message types (PReq/PRes, RReq/RRes wiring), or any new EMV 3DS protocol behaviour.
  Trigger this skill whenever someone says things like "add decoupled auth", "implement 3RI",
  "add a new challenge method", "extend the auth flow", "add OOB challenge", "wire up RReq",
  "add a new transStatus path", or "implement [anything] in the ACS". Also trigger when
  someone asks to "support spec version X" or "add a new message type".
---

# ACS Add Flow Skill

This skill guides you through adding a new authentication flow to the `acs-server` codebase
while preserving the Clean Architecture layer boundaries and EMV 3DS spec correctness.

## Project Architecture Reminder

The four layers and their strict dependency rule (inner layers never import outer):

```
domain/entity      ← aggregates, value objects, no external deps
domain/repository  ← port interfaces only (no implementation)
domain/service     ← pure domain logic (RiskEngine, OTPService, AuditLogger)
usecase/           ← orchestration; imports domain only
adapter/http/      ← Gin handlers, DTOs; imports usecase only
adapter/postgres/  ← repo implementations; imports domain only
infrastructure/    ← config, crypto, db pool, logger
cmd/server/main.go ← wires everything via constructor injection
```

Never add infrastructure imports to `domain/` or `usecase/`. Never add `gin` to `usecase/`.

---

## Step-by-step: Adding a New Flow

### 1. Identify spec touchpoints

Before writing any code, answer these questions:

- Which EMV 3DS message(s) does this flow touch? (AReq/ARes, CReq/CRes, RReq/RRes, PReq/PRes)
- Which `transStatus` value(s) does it produce? (`Y`, `N`, `C`, `D`, `A`, `U`, `R`, `I`)
- Which `deviceChannel` values does it apply to? (`01`=App, `02`=Browser, `03`=3RI)
- Does it require a new session type, a new DB column, or a new migration?
- Which spec versions support it? (2.2.0, 2.3.1, or both)

Write a one-paragraph spec summary before touching code. This prevents mid-implementation rethinks.

### 2. Extend the domain (if needed)

**Only add to `internal/domain/` if the new flow introduces a genuinely new concept.**

If adding a new `TransStatus` value, add it to `entity/transaction.go`:
```go
TransStatusDecoupled TransStatus = "D"
```

If the flow needs new state on `Transaction` (e.g. a decoupled auth expiry), add the field there
and update the migration. Keep entity methods behaviour-rich:
```go
func (t *Transaction) IsDecoupledPending() bool {
    return t.TransStatus == TransStatusDecoupled && t.CompletedAt == nil
}
```

If it needs a brand new aggregate (rare), create a new file in `entity/` following the pattern
of `session.go`: factory function, TTL/expiry helpers, status transitions as methods.

If it needs a new repository port, add an interface to `domain/repository/interfaces.go`.
Keep each interface focused: one interface per aggregate.

### 3. Write the use case

Create `internal/usecase/<flow_name>.go`. The use case is the heart of the flow.

**Standard structure:**
```go
// Input carries everything the use case needs — no HTTP types, no DB types.
type <Flow>Input struct { ... }

// Output carries everything the caller (handler) needs — no DB types.
type <Flow>Output struct { ... }

// <Flow>UseCase holds injected dependencies.
type <Flow>UseCase struct {
    txRepo      repository.TransactionRepository
    sessionRepo repository.SessionRepository
    auditLog    service.AuditLogger
    log         *zap.Logger
    // add only what this use case actually needs
}

func New<Flow>UseCase(...) *<Flow>UseCase { ... }

func (uc *<Flow>UseCase) Execute(ctx context.Context, in *<Flow>Input) (*<Flow>Output, error) {
    // 1. Load aggregate(s) from repo
    // 2. Validate domain invariants
    // 3. Apply domain logic / call domain services
    // 4. Persist changes
    // 5. Emit audit event
    // 6. Return output
}
```

Emit an audit event for every terminal outcome (success, failure, timeout):
```go
_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEvent<YourEvent>, nil))
```

Add the new `AuditEventType` constant to `internal/domain/service/audit.go` first.

### 4. Add DTOs

Add request/response structs to `internal/adapter/http/dto/messages.go`. Name them after the
EMV 3DS message type: `<MessageType>DTO` (e.g. `RReqDTO`, `DecoupledChallengeDTO`).

Field names must match the EMV 3DS spec exactly (camelCase JSON names). Add `binding:"required"`
tags for mandatory fields. Use pointers (`*string`, `*int`) for conditional fields.

### 5. Add the HTTP handler

Create or extend a handler in `internal/adapter/http/handler/`. Follow the existing pattern:

```go
type <Flow>Handler struct {
    uc  *usecase.<Flow>UseCase
    log *zap.Logger
}

func (h *<Flow>Handler) Handle<Method>(c *gin.Context) {
    var req dto.<ReqDTO>
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, errorResponse("203", "A", "...", err.Error(), "<MsgType>"))
        return
    }
    // map DTO → use case input
    out, err := h.uc.Execute(c.Request.Context(), input)
    if err != nil { ... }
    // map use case output → response DTO
    c.JSON(http.StatusOK, resp)
}
```

Always use `errorResponse()` (defined in `auth_handler.go`) for error responses — it produces
a spec-compliant `Erro` message type.

### 6. Register the route

Add the route to `internal/adapter/http/router.go`:
```go
v1.POST("/your-endpoint", flowHandler.HandleMethod)
```

Follow the existing URL conventions:
- `/v1/auth` → DS-to-ACS calls (JSON body)
- `/v1/challenge/*` → cardholder-facing challenge flow
- `/v1/method/*` → 3DS Method device fingerprinting

### 7. Wire in main.go

In `cmd/server/main.go`, follow the existing DI pattern:
```go
// Use case
myFlowUC := usecase.New<Flow>UseCase(txRepo, sessionRepo, auditRepo, log)

// Handler
myFlowHandler := handler.New<Flow>Handler(myFlowUC, log)

// Router (pass to NewRouter)
router := acshttp.NewRouter(authH, methodH, challengeH, myFlowHandler, log)
```

Update `NewRouter`'s signature accordingly.

### 8. Write tests

Every new use case needs a `<flow>_test.go` alongside it. Follow `authenticate_test.go`:

- Define minimal mock structs for each repository interface used
- Write one test per meaningful decision branch:
  - Happy path (expected TransStatus)
  - Entity not found
  - Domain invariant violated (e.g. wrong current state)
  - Repo error
- Use `service.NoopAuditLogger{}` for the audit logger mock

For the handler, add tests in `handler/<flow>_handler_test.go` following `auth_handler_test.go`.

### 9. Add a migration if state changes

If the flow adds columns or tables, create `migrations/000N_<description>.sql` using Goose format:

```sql
-- +goose Up
-- +goose StatementBegin
ALTER TABLE transactions ADD COLUMN decoupled_expiry TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE transactions DROP COLUMN IF EXISTS decoupled_expiry;
-- +goose StatementEnd
```

Always include a `-- +goose Down` section.

---

## Common Flow Recipes

### Decoupled Authentication (transStatus = "D")

Key differences from challenge flow:
- ACS authenticates the cardholder out-of-band (push notification, bank app)
- ARes returns `transStatus=D` immediately; no challenge redirect URL
- ACS later sends an RReq to the DS when authentication is complete
- Add `decoupled_expiry` column to `transactions`; the cardholder has up to 3 minutes (or per DS rules)

New use case: `usecase/decoupled_auth.go` with `InitiateDecoupled` and `CompleteDecoupled` methods.
New handler endpoint: `POST /v1/decoupled/complete` (called by your push notification callback).

### 3DS Requestor-Initiated (3RI) — deviceChannel "03"

- No cardholder present; merchant initiates on behalf of a prior authenticated session
- AReq must carry `threeRIInd` and `priorAuthenticationData`
- Risk engine should auto-frictionless (no challenge possible without cardholder)
- Add `threeRIInd` and `prior_auth_ref` fields to `Transaction` entity and migration

### New Challenge Type (e.g. OOB)

- Add new `ChallengeType` value type in `entity/`
- Extend `OTPService` OR create a new `OOBService` in `domain/service/`
- Add new challenge session type `"oob"` to `ACSSession`
- Add new handler endpoint for OOB callback: `POST /v1/challenge/oob-callback`

---

## Checklist Before Opening a PR

- [ ] Domain layer has zero new imports from `adapter/` or `infrastructure/`
- [ ] Use case has zero imports from `gin` or `sqlx`
- [ ] Every new terminal outcome emits an `AuditEvent`
- [ ] All new repository methods have a matching interface entry in `domain/repository/interfaces.go`
- [ ] New migration has both `Up` and `Down` sections
- [ ] New handler uses `errorResponse()` for all error returns
- [ ] At least one test per use case decision branch
- [ ] `NewRouter` updated and `main.go` wired
