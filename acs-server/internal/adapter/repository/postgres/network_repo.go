package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/your-org/acs-server/internal/domain/entity"
)

// NetworkRepo implements persistence for the networks table.
type NetworkRepo struct {
	db *sqlx.DB
}

// NewNetworkRepo creates a new NetworkRepo.
func NewNetworkRepo(db *sqlx.DB) *NetworkRepo {
	return &NetworkRepo{db: db}
}

// ListAll returns all networks ordered by name.
func (r *NetworkRepo) ListAll(ctx context.Context) ([]*entity.NetworkEntry, error) {
	var rows []*entity.NetworkEntry
	const q = `SELECT * FROM networks ORDER BY name`
	if err := r.db.SelectContext(ctx, &rows, q); err != nil {
		return nil, fmt.Errorf("NetworkRepo.ListAll: %w", err)
	}
	return rows, nil
}

// ListActive returns only active networks ordered by name.
func (r *NetworkRepo) ListActive(ctx context.Context) ([]*entity.NetworkEntry, error) {
	var rows []*entity.NetworkEntry
	const q = `SELECT * FROM networks WHERE active = true ORDER BY name`
	if err := r.db.SelectContext(ctx, &rows, q); err != nil {
		return nil, fmt.Errorf("NetworkRepo.ListActive: %w", err)
	}
	return rows, nil
}

// GetByName returns a network by its slug name, or nil if not found.
func (r *NetworkRepo) GetByName(ctx context.Context, name string) (*entity.NetworkEntry, error) {
	var row entity.NetworkEntry
	const q = `SELECT * FROM networks WHERE name = $1`
	if err := r.db.GetContext(ctx, &row, q, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("NetworkRepo.GetByName: %w", err)
	}
	return &row, nil
}

// Create inserts a new network.
func (r *NetworkRepo) Create(ctx context.Context, n *entity.NetworkEntry) error {
	const q = `
		INSERT INTO networks (name, display_name, active, color, eci_full_auth, eci_attempt, eci_not_auth)
		VALUES (:name, :display_name, :active, :color, :eci_full_auth, :eci_attempt, :eci_not_auth)
		RETURNING id, created_at, updated_at`
	rows, err := r.db.NamedQueryContext(ctx, q, n)
	if err != nil {
		return fmt.Errorf("NetworkRepo.Create: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&n.ID, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return fmt.Errorf("NetworkRepo.Create scan: %w", err)
		}
	}
	return nil
}

// Update modifies an existing network by name.
func (r *NetworkRepo) Update(ctx context.Context, n *entity.NetworkEntry) error {
	const q = `
		UPDATE networks SET
			display_name  = :display_name,
			active        = :active,
			color         = :color,
			eci_full_auth = :eci_full_auth,
			eci_attempt   = :eci_attempt,
			eci_not_auth  = :eci_not_auth
		WHERE name = :name`
	res, err := r.db.NamedExecContext(ctx, q, n)
	if err != nil {
		return fmt.Errorf("NetworkRepo.Update: %w", err)
	}
	if cnt, _ := res.RowsAffected(); cnt == 0 {
		return fmt.Errorf("NetworkRepo.Update: network %q not found", n.Name)
	}
	return nil
}

// Delete removes a network by name.
func (r *NetworkRepo) Delete(ctx context.Context, name string) error {
	const q = `DELETE FROM networks WHERE name = $1`
	res, err := r.db.ExecContext(ctx, q, name)
	if err != nil {
		return fmt.Errorf("NetworkRepo.Delete: %w", err)
	}
	if cnt, _ := res.RowsAffected(); cnt == 0 {
		return fmt.Errorf("NetworkRepo.Delete: network %q not found", name)
	}
	return nil
}