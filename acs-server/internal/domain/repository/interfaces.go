package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/your-org/acs-server/internal/domain/entity"
)

// TransactionRepository defines persistence operations for the Transaction aggregate.
type TransactionRepository interface {
	// Save persists a new transaction (INSERT).
	Save(ctx context.Context, tx *entity.Transaction) error

	// Update persists changes to an existing transaction (UPDATE).
	Update(ctx context.Context, tx *entity.Transaction) error

	// FindByACSTransactionID retrieves a transaction by ACS-assigned ID.
	FindByACSTransactionID(ctx context.Context, id uuid.UUID) (*entity.Transaction, error)

	// FindByServerTransactionID retrieves a transaction by 3DS Server ID.
	FindByServerTransactionID(ctx context.Context, id uuid.UUID) (*entity.Transaction, error)

	// List returns a paginated, filtered slice of transactions and total count.
	List(ctx context.Context, f TransactionFilter) ([]*entity.Transaction, int64, error)
}

// TransactionFilter controls pagination and optional filtering for List queries.
type TransactionFilter struct {
	Status  string // e.g. "Y", "C", "N" — empty means all
	Network string // e.g. "visa" — empty means all
	Channel string // e.g. "02" — empty means all
	Search  string // free-text search across key columns
	Page    int    // 1-based
	PerPage int    // rows per page
}

// AuditLogRepository provides read access to stored audit events.
type AuditLogRepository interface {
	List(ctx context.Context, f AuditLogFilter) ([]*entity.AuditLogRow, int64, error)
}

// AuditLogFilter controls pagination and optional filtering for audit log queries.
type AuditLogFilter struct {
	EventType string // e.g. "challenge_success" — empty means all
	Search    string // free-text search across key columns
	Page      int
	PerPage   int
}

// CardRangeRepository defines operations for BIN range lookups.
type CardRangeRepository interface {
	// FindByPAN returns the CardRange matching the given PAN, or nil if not enrolled.
	FindByPAN(ctx context.Context, pan string) (*entity.CardRange, error)

	// Upsert inserts or updates a card range (used during DS PRes processing).
	Upsert(ctx context.Context, cr *entity.CardRange) error

	// DeleteByRange removes a card range (action_ind = "D").
	DeleteByRange(ctx context.Context, start, end string) error

	// ListAll returns all card ranges ordered by start_range.
	ListAll(ctx context.Context) ([]*entity.CardRange, error)
}

// MessageRepository persists raw 3DS protocol messages for compliance and debugging.
type MessageRepository interface {
	Save(ctx context.Context, msg *entity.Message) error
	ListOperationalMessages(ctx context.Context, page, perPage int) ([]*entity.Message, int64, error)
}

// SessionRepository defines persistence operations for ACS sessions.
type SessionRepository interface {
	// Save persists a new session.
	Save(ctx context.Context, s *entity.ACSSession) error

	// FindByID retrieves a session by its primary key.
	FindByID(ctx context.Context, id uuid.UUID) (*entity.ACSSession, error)

	// FindByTransactionID retrieves the active session for a transaction.
	FindByTransactionID(ctx context.Context, txID uuid.UUID, sessionType string) (*entity.ACSSession, error)

	// UpdateStatus updates the session lifecycle status.
	UpdateStatus(ctx context.Context, id uuid.UUID, status entity.SessionStatus) error
}
