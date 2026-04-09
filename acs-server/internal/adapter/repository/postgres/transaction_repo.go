package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/repository"
)

// TransactionRepo implements repository.TransactionRepository using PostgreSQL.
type TransactionRepo struct {
	db *sqlx.DB
}

// NewTransactionRepo creates a new TransactionRepo.
func NewTransactionRepo(db *sqlx.DB) *TransactionRepo {
	return &TransactionRepo{db: db}
}

// DB exposes the underlying sqlx DB for read-heavy analytics queries.
func (r *TransactionRepo) DB() *sqlx.DB {
	return r.db
}

// Save inserts a new Transaction row.
func (r *TransactionRepo) Save(ctx context.Context, tx *entity.Transaction) error {
	const q = `
		INSERT INTO transactions (
			id, acs_transaction_id, ds_transaction_id, server_transaction_id, sdk_transaction_id,
			spec_version, message_category, device_channel,
			acct_number, card_bin, network,
			merchant_name, merchant_country, merchant_id,
			purchase_amount, purchase_currency, purchase_exponent,
			trans_status, auth_value, eci,
			challenge_attempts, otp_hash, otp_expiry,
			second_factor_required, pin_hash, pin_expiry, pin_attempts,
			created_at, updated_at, completed_at
		) VALUES (
			:id, :acs_transaction_id, :ds_transaction_id, :server_transaction_id, :sdk_transaction_id,
			:spec_version, :message_category, :device_channel,
			:acct_number, :card_bin, :network,
			:merchant_name, :merchant_country, :merchant_id,
			:purchase_amount, :purchase_currency, :purchase_exponent,
			:trans_status, :auth_value, :eci,
			:challenge_attempts, :otp_hash, :otp_expiry,
			:second_factor_required, :pin_hash, :pin_expiry, :pin_attempts,
			:created_at, :updated_at, :completed_at
		)`
	_, err := r.db.NamedExecContext(ctx, q, tx)
	if err != nil {
		return fmt.Errorf("TransactionRepo.Save: %w", err)
	}
	return nil
}

// Update persists changes to an existing Transaction row.
func (r *TransactionRepo) Update(ctx context.Context, tx *entity.Transaction) error {
	const q = `
		UPDATE transactions SET
			trans_status            = :trans_status,
			auth_value              = :auth_value,
			eci                     = :eci,
			challenge_attempts      = :challenge_attempts,
			otp_hash                = :otp_hash,
			otp_expiry              = :otp_expiry,
			second_factor_required  = :second_factor_required,
			pin_hash                = :pin_hash,
			pin_expiry              = :pin_expiry,
			pin_attempts            = :pin_attempts,
			updated_at              = :updated_at,
			completed_at            = :completed_at
		WHERE acs_transaction_id = :acs_transaction_id`
	_, err := r.db.NamedExecContext(ctx, q, tx)
	if err != nil {
		return fmt.Errorf("TransactionRepo.Update: %w", err)
	}
	return nil
}

// FindByACSTransactionID retrieves a transaction by ACS-assigned UUID.
func (r *TransactionRepo) FindByACSTransactionID(ctx context.Context, id uuid.UUID) (*entity.Transaction, error) {
	var tx entity.Transaction
	const q = `SELECT * FROM transactions WHERE acs_transaction_id = $1 LIMIT 1`
	err := r.db.GetContext(ctx, &tx, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("TransactionRepo.FindByACSTransactionID: %w", err)
	}
	return &tx, nil
}

// List returns paginated transactions with optional status/network/channel filters.
func (r *TransactionRepo) List(ctx context.Context, f repository.TransactionFilter) ([]*entity.Transaction, int64, error) {
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
	if f.Status != "" {
		where += fmt.Sprintf(" AND trans_status = $%d", n)
		args = append(args, f.Status)
		n++
	}
	if f.Network != "" {
		where += fmt.Sprintf(" AND network = $%d", n)
		args = append(args, f.Network)
		n++
	}
	if f.Channel != "" {
		where += fmt.Sprintf(" AND device_channel = $%d", n)
		args = append(args, f.Channel)
		n++
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		where += fmt.Sprintf(
			" AND (acs_transaction_id::text ILIKE $%d OR ds_transaction_id::text ILIKE $%d OR merchant_name ILIKE $%d OR network ILIKE $%d OR trans_status ILIKE $%d OR COALESCE(purchase_amount,'') ILIKE $%d)",
			n, n, n, n, n, n)
		args = append(args, like)
		n++
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM transactions "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("TransactionRepo.List count: %w", err)
	}

	q := fmt.Sprintf("SELECT * FROM transactions %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", where, n, n+1)
	args = append(args, f.PerPage, offset)

	var rows []*entity.Transaction
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, 0, fmt.Errorf("TransactionRepo.List: %w", err)
	}
	return rows, total, nil
}

// FindByServerTransactionID retrieves a transaction by 3DS Server UUID.
func (r *TransactionRepo) FindByServerTransactionID(ctx context.Context, id uuid.UUID) (*entity.Transaction, error) {
	var tx entity.Transaction
	const q = `SELECT * FROM transactions WHERE server_transaction_id = $1 LIMIT 1`
	err := r.db.GetContext(ctx, &tx, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("TransactionRepo.FindByServerTransactionID: %w", err)
	}
	return &tx, nil
}
