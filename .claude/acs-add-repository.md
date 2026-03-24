---
name: acs-add-repository
description: >
  Use this skill whenever a developer on the acs-server project needs to add a new database
  entity, a new repository, or extend an existing one. Trigger this skill for requests like
  "add a CardHolder table", "store issuer config in the DB", "add a new repo for X",
  "add a field to the Transaction entity", "create a new aggregate", "persist Y to postgres",
  "add a new DB table", "extend the schema", "track Z in the database", or any request
  involving new data persistence in the ACS. Also use this when someone asks to add caching,
  swap a repo implementation, or add a Redis layer on top of an existing PostgreSQL repo.
---

# ACS Add Repository Skill

This skill walks you through adding a new persistent entity to `acs-server` without breaking
Clean Architecture boundaries or introducing inconsistency with the existing codebase patterns.

## The Repository Pattern in This Project

```
domain/entity/<name>.go          ← the aggregate (pure Go, zero external deps)
domain/repository/interfaces.go  ← the port (interface only)
adapter/repository/postgres/<name>_repo.go  ← the adapter (sqlx implementation)
migrations/000N_<description>.sql           ← Goose migration
```

The domain owns the shape of data. The adapter owns how it's stored.
`main.go` constructs the adapter and injects it wherever the interface is needed.

---

## Step 1: Define the Entity

Create `internal/domain/entity/<name>.go`. Rules:

- Zero imports from `adapter/`, `infrastructure/`, `gin`, `sqlx`, `pgx`
- Allowed imports: `time`, `github.com/google/uuid`, other `entity` types
- Use typed string constants for status/enum fields (see `TransStatus`, `SessionStatus`)
- Include a factory function `New<Name>(...)` that sets `ID = uuid.New()` and timestamps
- Add behaviour as methods, not raw field access: `IsExpired()`, `Complete()`, `IsChallengeable()`
- Use pointer types (`*string`, `*time.Time`) only for genuinely optional fields

**Template:**
```go
package entity

import (
    "time"
    "github.com/google/uuid"
)

type <Name>Status string

const (
    <Name>StatusActive  <Name>Status = "active"
    <Name>StatusExpired <Name>Status = "expired"
)

type <Name> struct {
    ID        uuid.UUID    `db:"id"`
    // ... your fields
    Status    <Name>Status `db:"status"`
    CreatedAt time.Time    `db:"created_at"`
    UpdatedAt time.Time    `db:"updated_at"`
}

func New<Name>(...) *<Name> {
    now := time.Now().UTC()
    return &<Name>{
        ID:        uuid.New(),
        Status:    <Name>StatusActive,
        CreatedAt: now,
        UpdatedAt: now,
    }
}

func (n *<Name>) IsExpired() bool {
    // domain logic here
}
```

## Step 2: Define the Repository Interface

Add to `internal/domain/repository/interfaces.go` (do not create a new file):

```go
// <Name>Repository defines persistence operations for the <Name> aggregate.
type <Name>Repository interface {
    Save(ctx context.Context, n *entity.<Name>) error
    FindByID(ctx context.Context, id uuid.UUID) (*entity.<Name>, error)
    // add only the methods your use cases actually need
    // avoid speculative methods — YAGNI
}
```

Keep each interface minimal. If only one use case calls `FindByStatus`, add it.
If no use case needs `Delete`, don't add it.

## Step 3: Write the Migration

Create `migrations/000N_<description>.sql` (increment N from the last migration file).

**Always follow this exact Goose format:**
```sql
-- +goose Up
-- +goose StatementBegin

CREATE TABLE <name>s (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- your columns
    status      VARCHAR(20)  NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Index every column used in WHERE clauses
CREATE INDEX idx_<name>s_<col> ON <name>s (<col>);

-- Add the updated_at trigger if the table needs it
CREATE TRIGGER trg_<name>s_updated_at
    BEFORE UPDATE ON <name>s
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_<name>s_updated_at ON <name>s;
DROP TABLE IF EXISTS <name>s;
-- +goose StatementEnd
```

Column conventions matching the existing schema:
- UUIDs: `UUID NOT NULL` (primary keys use `DEFAULT gen_random_uuid()`)
- Short enums: `VARCHAR(20) NOT NULL DEFAULT '...'`
- Encrypted sensitive data: `TEXT NOT NULL` (AES-GCM hex-encoded by the repo layer)
- Optional fields: nullable without a default
- All timestamps: `TIMESTAMPTZ NOT NULL DEFAULT NOW()`

## Step 4: Implement the PostgreSQL Adapter

Create `internal/adapter/repository/postgres/<name>_repo.go`.

**Pattern to follow exactly (mirrors `transaction_repo.go`):**

```go
package postgres

import (
    "context"
    "database/sql"
    "errors"
    "fmt"

    "github.com/google/uuid"
    "github.com/jmoiron/sqlx"
    "github.com/your-org/acs-server/internal/domain/entity"
)

type <Name>Repo struct {
    db *sqlx.DB
}

func New<Name>Repo(db *sqlx.DB) *<Name>Repo {
    return &<Name>Repo{db: db}
}

func (r *<Name>Repo) Save(ctx context.Context, n *entity.<Name>) error {
    const q = `INSERT INTO <name>s (...) VALUES (:...)`
    _, err := r.db.NamedExecContext(ctx, q, n)
    if err != nil {
        return fmt.Errorf("<Name>Repo.Save: %w", err)
    }
    return nil
}

func (r *<Name>Repo) FindByID(ctx context.Context, id uuid.UUID) (*entity.<Name>, error) {
    var n entity.<Name>
    const q = `SELECT * FROM <name>s WHERE id = $1 LIMIT 1`
    err := r.db.GetContext(ctx, &n, q, id)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, nil  // not found → nil, nil (not an error)
    }
    if err != nil {
        return nil, fmt.Errorf("<Name>Repo.FindByID: %w", err)
    }
    return &n, nil
}
```

Critical rules:
- `not found` must return `nil, nil` — not an error. Callers check for nil.
- Always wrap errors: `fmt.Errorf("<Name>Repo.<Method>: %w", err)`
- Use `NamedExecContext` for INSERT/UPDATE (maps struct fields via `db:` tags)
- Use `GetContext`/`SelectContext` for SELECT (maps columns back to struct)
- Encrypt sensitive fields (PAN, card numbers) before Save, decrypt after Find
  — do this in the repo, not in the use case

## Step 5: Wire into main.go

In `cmd/server/main.go`, after the existing repo instantiations:

```go
myRepo := postgres.New<Name>Repo(db)
```

Then pass `myRepo` to the use case(s) that need it:
```go
myUC := usecase.New<Flow>UseCase(txRepo, myRepo, auditRepo, log)
```

## Step 6: Write Tests for the Use Case

Use cases that depend on the new repo need mock implementations in their test files.
Follow the pattern in `authenticate_test.go` — define a minimal struct that satisfies
the interface, with just enough behaviour for the test scenario:

```go
type mock<Name>Repo struct {
    stored *entity.<Name>
}

func (m *mock<Name>Repo) Save(_ context.Context, n *entity.<Name>) error {
    m.stored = n
    return nil
}

func (m *mock<Name>Repo) FindByID(_ context.Context, id uuid.UUID) (*entity.<Name>, error) {
    if m.stored != nil && m.stored.ID == id {
        return m.stored, nil
    }
    return nil, nil
}
```

---

## Adding a Field to an Existing Entity

When adding a column to an existing table (e.g. `transactions`):

1. Add the field to the entity struct with a `db:` tag
2. Update the `Save` and `Update` SQL in the repo (add to INSERT column list and UPDATE SET)
3. Write a new migration: `ALTER TABLE transactions ADD COLUMN ...`
4. If nullable, use a pointer type in the struct; if not nullable, ensure existing rows get a default in the migration

---

## Caching Layer Pattern (optional)

If a repo is read-heavy and you want to add Redis caching without changing the domain:

Create `adapter/repository/cached_<name>_repo.go` that wraps the PostgreSQL repo:

```go
type Cached<Name>Repo struct {
    pg    repository.<Name>Repository  // the real repo
    cache *redis.Client
    ttl   time.Duration
}

// FindByID checks cache first, falls back to pg, populates cache on miss.
func (r *Cached<Name>Repo) FindByID(ctx context.Context, id uuid.UUID) (*entity.<Name>, error) {
    // try cache
    // on miss: call r.pg.FindByID, store result
}
```

The use case sees only the `repository.<Name>Repository` interface — it never knows
whether it's talking to Postgres or a cache. Swap at the `main.go` wiring point.

---

## Checklist

- [ ] Entity in `domain/entity/` — zero external imports
- [ ] Interface added to `domain/repository/interfaces.go`
- [ ] Migration file with both Up and Down sections, index on all WHERE columns
- [ ] Adapter in `adapter/repository/postgres/` — wraps errors, returns nil/nil for not-found
- [ ] Sensitive fields encrypted/decrypted in the repo layer
- [ ] `main.go` wired
- [ ] Mock implementation in use case test file
