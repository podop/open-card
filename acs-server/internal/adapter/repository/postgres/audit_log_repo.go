package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/repository"
)

// AuditLogRepo adds list/query access to the audit_log table.
// It is separate from AuditRepo (which implements service.AuditLogger for writes).
type AuditLogRepo struct {
	db *sqlx.DB
}

// NewAuditLogRepo creates a new AuditLogRepo.
func NewAuditLogRepo(db *sqlx.DB) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

// List returns paginated audit log rows with an optional event_type filter.
func (r *AuditLogRepo) List(ctx context.Context, f repository.AuditLogFilter) ([]*entity.AuditLogRow, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 || f.PerPage > 200 {
		f.PerPage = 20
	}
	offset := (f.Page - 1) * f.PerPage

	where := "WHERE 1=1"
	args := []interface{}{}
	n := 1
	if f.EventType != "" {
		where += fmt.Sprintf(" AND event_type = $%d", n)
		args = append(args, f.EventType)
		n++
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		where += fmt.Sprintf(
			" AND (acs_transaction_id::text ILIKE $%d OR event_type ILIKE $%d OR trans_status ILIKE $%d OR card_bin ILIKE $%d OR device_channel ILIKE $%d)",
			n, n, n, n, n)
		args = append(args, like)
		n++
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("AuditLogRepo.List count: %w", err)
	}

	q := fmt.Sprintf("SELECT * FROM audit_log %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", where, n, n+1)
	args = append(args, f.PerPage, offset)

	var rows []*entity.AuditLogRow
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, 0, fmt.Errorf("AuditLogRepo.List: %w", err)
	}
	return rows, total, nil
}

// ListByTransaction returns all audit log entries for a given ACS transaction, ordered chronologically.
func (r *AuditLogRepo) ListByTransaction(ctx context.Context, acsTransID uuid.UUID) ([]*entity.AuditLogRow, error) {
	var rows []*entity.AuditLogRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM audit_log WHERE acs_transaction_id = $1 ORDER BY created_at`, acsTransID)
	if err != nil {
		return nil, fmt.Errorf("AuditLogRepo.ListByTransaction: %w", err)
	}
	return rows, nil
}