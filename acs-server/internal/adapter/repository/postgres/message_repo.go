package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/your-org/acs-server/internal/domain/entity"
)

// MessageRepo implements repository.MessageRepository using PostgreSQL.
type MessageRepo struct {
	db *sqlx.DB
}

// NewMessageRepo creates a new MessageRepo.
func NewMessageRepo(db *sqlx.DB) *MessageRepo {
	return &MessageRepo{db: db}
}

// Save inserts a raw 3DS message record.
func (r *MessageRepo) Save(ctx context.Context, msg *entity.Message) error {
	const q = `
		INSERT INTO messages (acs_transaction_id, message_type, direction, content, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, q,
		msg.ACSTransactionID,
		msg.MessageType,
		msg.Direction,
		msg.Content,
		msg.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("MessageRepo.Save: %w", err)
	}
	return nil
}

// ListByTransaction returns all messages for a given ACS transaction, ordered chronologically.
func (r *MessageRepo) ListByTransaction(ctx context.Context, acsTransID uuid.UUID) ([]*entity.Message, error) {
	var rows []*entity.Message
	err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM messages WHERE acs_transaction_id = $1 ORDER BY created_at`, acsTransID)
	if err != nil {
		return nil, fmt.Errorf("MessageRepo.ListByTransaction: %w", err)
	}
	return rows, nil
}

// ListOperationalMessages returns paginated OReq/ORes messages (those with NULL acs_transaction_id).
func (r *MessageRepo) ListOperationalMessages(ctx context.Context, page, perPage int) ([]*entity.Message, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	var total int64
	err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM messages WHERE acs_transaction_id IS NULL`)
	if err != nil {
		return nil, 0, fmt.Errorf("MessageRepo.ListOperationalMessages count: %w", err)
	}

	var rows []*entity.Message
	err = r.db.SelectContext(ctx, &rows,
		`SELECT * FROM messages WHERE acs_transaction_id IS NULL ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("MessageRepo.ListOperationalMessages: %w", err)
	}
	return rows, total, nil
}