package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/your-org/acs-server/internal/domain/entity"
)

// MFAProviderRepo implements persistence for MFA providers.
type MFAProviderRepo struct {
	db *sqlx.DB
}

func NewMFAProviderRepo(db *sqlx.DB) *MFAProviderRepo {
	return &MFAProviderRepo{db: db}
}

func (r *MFAProviderRepo) List(ctx context.Context) ([]*entity.MFAProvider, error) {
	var rows []*entity.MFAProvider
	const q = `SELECT * FROM mfa_providers ORDER BY id`
	if err := r.db.SelectContext(ctx, &rows, q); err != nil {
		return nil, fmt.Errorf("MFAProviderRepo.List: %w", err)
	}
	return rows, nil
}

func (r *MFAProviderRepo) GetByID(ctx context.Context, id int64) (*entity.MFAProvider, error) {
	var row entity.MFAProvider
	const q = `SELECT * FROM mfa_providers WHERE id = $1`
	if err := r.db.GetContext(ctx, &row, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("MFAProviderRepo.GetByID: %w", err)
	}
	return &row, nil
}

func (r *MFAProviderRepo) GetByName(ctx context.Context, name string) (*entity.MFAProvider, error) {
	var row entity.MFAProvider
	const q = `SELECT * FROM mfa_providers WHERE name = $1`
	if err := r.db.GetContext(ctx, &row, q, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("MFAProviderRepo.GetByName: %w", err)
	}
	return &row, nil
}

func (r *MFAProviderRepo) Create(ctx context.Context, p *entity.MFAProvider) error {
	const q = `
		INSERT INTO mfa_providers (name, display_name, type, factor_type, active, config, description)
		VALUES (:name, :display_name, :type, :factor_type, :active, :config, :description)
		RETURNING id`
	rows, err := r.db.NamedQueryContext(ctx, q, p)
	if err != nil {
		return fmt.Errorf("MFAProviderRepo.Create: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		rows.Scan(&p.ID)
	}
	return nil
}

func (r *MFAProviderRepo) Update(ctx context.Context, p *entity.MFAProvider) error {
	const q = `
		UPDATE mfa_providers SET
			display_name = :display_name,
			type = :type,
			factor_type = :factor_type,
			active = :active,
			config = :config,
			description = :description
		WHERE id = :id`
	_, err := r.db.NamedExecContext(ctx, q, p)
	if err != nil {
		return fmt.Errorf("MFAProviderRepo.Update: %w", err)
	}
	return nil
}

func (r *MFAProviderRepo) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM mfa_providers WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("MFAProviderRepo.Delete: %w", err)
	}
	return nil
}

func (r *MFAProviderRepo) GetActiveByID(ctx context.Context, id int64) (*entity.MFAProvider, error) {
	var row entity.MFAProvider
	const q = `SELECT * FROM mfa_providers WHERE id = $1 AND active = true`
	if err := r.db.GetContext(ctx, &row, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("MFAProviderRepo.GetActiveByID: %w", err)
	}
	return &row, nil
}
