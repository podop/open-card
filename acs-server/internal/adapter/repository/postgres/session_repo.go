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

// SessionRepo implements repository.SessionRepository using PostgreSQL.
type SessionRepo struct {
	db *sqlx.DB
}

// NewSessionRepo creates a new SessionRepo.
func NewSessionRepo(db *sqlx.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

// Save inserts a new ACSSession row.
func (r *SessionRepo) Save(ctx context.Context, s *entity.ACSSession) error {
	const q = `
		INSERT INTO acs_sessions (
			id, transaction_id, session_type, status,
			notif_url, device_data, expires_at, created_at
		) VALUES (
			:id, :transaction_id, :session_type, :status,
			:notif_url, :device_data, :expires_at, :created_at
		)`
	_, err := r.db.NamedExecContext(ctx, q, s)
	if err != nil {
		return fmt.Errorf("SessionRepo.Save: %w", err)
	}
	return nil
}

// FindByID retrieves an ACSSession by primary key.
func (r *SessionRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.ACSSession, error) {
	var s entity.ACSSession
	const q = `SELECT * FROM acs_sessions WHERE id = $1 LIMIT 1`
	err := r.db.GetContext(ctx, &s, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("SessionRepo.FindByID: %w", err)
	}
	return &s, nil
}

// FindByTransactionID retrieves the most recent active session for a transaction.
func (r *SessionRepo) FindByTransactionID(ctx context.Context, txID uuid.UUID, sessionType string) (*entity.ACSSession, error) {
	var s entity.ACSSession
	const q = `
		SELECT * FROM acs_sessions
		WHERE transaction_id = $1
		  AND session_type   = $2
		  AND status         = 'pending'
		ORDER BY created_at DESC
		LIMIT 1`
	err := r.db.GetContext(ctx, &s, q, txID, sessionType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("SessionRepo.FindByTransactionID: %w", err)
	}
	return &s, nil
}

// UpdateStatus changes the lifecycle status of an ACSSession.
func (r *SessionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.SessionStatus) error {
	const q = `UPDATE acs_sessions SET status = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, q, status, id)
	if err != nil {
		return fmt.Errorf("SessionRepo.UpdateStatus: %w", err)
	}
	return nil
}
