package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/your-org/acs-server/internal/domain/entity"
)

// CardRangeRepo implements repository.CardRangeRepository using PostgreSQL.
type CardRangeRepo struct {
	db *sqlx.DB
}

// NewCardRangeRepo creates a new CardRangeRepo.
func NewCardRangeRepo(db *sqlx.DB) *CardRangeRepo {
	return &CardRangeRepo{db: db}
}

// FindByPAN returns the active CardRange whose start_range..end_range contains the PAN.
func (r *CardRangeRepo) FindByPAN(ctx context.Context, pan string) (*entity.CardRange, error) {
	var cr entity.CardRange
	// Pad or trim PAN to match stored range width via LEFT() comparison
	const q = `
		SELECT *
		FROM card_ranges
		WHERE active = true
		  AND LEFT($1, LENGTH(start_range)) BETWEEN start_range AND end_range
		ORDER BY LENGTH(start_range) DESC
		LIMIT 1`
	err := r.db.GetContext(ctx, &cr, q, pan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("CardRangeRepo.FindByPAN: %w", err)
	}
	return &cr, nil
}

// Upsert inserts or updates a CardRange row (used when processing DS PRes).
func (r *CardRangeRepo) Upsert(ctx context.Context, cr *entity.CardRange) error {
	const q = `
		INSERT INTO card_ranges (
			start_range, end_range,
			network,
			acs_url_method, acs_url_challenge,
			min_protocol, max_protocol,
			action_ind, active
		) VALUES (
			:start_range, :end_range,
			:network,
			:acs_url_method, :acs_url_challenge,
			:min_protocol, :max_protocol,
			:action_ind, :active
		)
		ON CONFLICT (start_range, end_range) DO UPDATE SET
			network           = EXCLUDED.network,
			acs_url_method    = EXCLUDED.acs_url_method,
			acs_url_challenge = EXCLUDED.acs_url_challenge,
			min_protocol      = EXCLUDED.min_protocol,
			max_protocol      = EXCLUDED.max_protocol,
			action_ind        = EXCLUDED.action_ind,
			active            = EXCLUDED.active`
	_, err := r.db.NamedExecContext(ctx, q, cr)
	if err != nil {
		return fmt.Errorf("CardRangeRepo.Upsert: %w", err)
	}
	return nil
}

// DeleteByRange soft-deletes a card range (sets active = false).
func (r *CardRangeRepo) DeleteByRange(ctx context.Context, start, end string) error {
	const q = `UPDATE card_ranges SET active = false WHERE start_range = $1 AND end_range = $2`
	_, err := r.db.ExecContext(ctx, q, start, end)
	if err != nil {
		return fmt.Errorf("CardRangeRepo.DeleteByRange: %w", err)
	}
	return nil
}

// ListAll returns all card ranges ordered by start_range.
func (r *CardRangeRepo) ListAll(ctx context.Context) ([]*entity.CardRange, error) {
	var rows []*entity.CardRange
	const q = `SELECT * FROM card_ranges ORDER BY start_range`
	if err := r.db.SelectContext(ctx, &rows, q); err != nil {
		return nil, fmt.Errorf("CardRangeRepo.ListAll: %w", err)
	}
	return rows, nil
}
