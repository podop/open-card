package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/your-org/acs-server/internal/domain/entity"
)

// NetworkRuleRepo implements persistence for network rules and BIN exclusions.
type NetworkRuleRepo struct {
	db *sqlx.DB
}

// NewNetworkRuleRepo creates a new NetworkRuleRepo.
func NewNetworkRuleRepo(db *sqlx.DB) *NetworkRuleRepo {
	return &NetworkRuleRepo{db: db}
}

// ── Network Rules ────────────────────────────────────────────────────────────

// ListNetworkRules returns all network rules ordered by network name.
func (r *NetworkRuleRepo) ListNetworkRules(ctx context.Context) ([]*entity.NetworkRule, error) {
	var rows []*entity.NetworkRule
	const q = `SELECT * FROM network_rules ORDER BY network`
	if err := r.db.SelectContext(ctx, &rows, q); err != nil {
		return nil, fmt.Errorf("ListNetworkRules: %w", err)
	}
	return rows, nil
}

// GetNetworkRule returns the rule for a specific network.
func (r *NetworkRuleRepo) GetNetworkRule(ctx context.Context, network string) (*entity.NetworkRule, error) {
	var row entity.NetworkRule
	const q = `SELECT * FROM network_rules WHERE network = $1`
	if err := r.db.GetContext(ctx, &row, q, network); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("GetNetworkRule: %w", err)
	}
	return &row, nil
}

// UpsertNetworkRule inserts or updates a network rule.
func (r *NetworkRuleRepo) UpsertNetworkRule(ctx context.Context, rule *entity.NetworkRule) error {
	const q = `
		INSERT INTO network_rules (network, allow_frictionless, allow_otp, allow_oob, default_challenge_channel, second_factor_mode, mfa_provider_id)
		VALUES (:network, :allow_frictionless, :allow_otp, :allow_oob, :default_challenge_channel, :second_factor_mode, :mfa_provider_id)
		ON CONFLICT (network) DO UPDATE SET
			allow_frictionless = EXCLUDED.allow_frictionless,
			allow_otp = EXCLUDED.allow_otp,
			allow_oob = EXCLUDED.allow_oob,
			default_challenge_channel = EXCLUDED.default_challenge_channel,
			second_factor_mode = EXCLUDED.second_factor_mode,
			mfa_provider_id = EXCLUDED.mfa_provider_id`
	_, err := r.db.NamedExecContext(ctx, q, rule)
	if err != nil {
		return fmt.Errorf("UpsertNetworkRule: %w", err)
	}
	return nil
}

// ── BIN Exclusions ──────────────────────────────────────────────────────────

// ListBinExclusions returns all BIN exclusions, optionally filtered by network.
func (r *NetworkRuleRepo) ListBinExclusions(ctx context.Context, network string) ([]*entity.BinExclusion, error) {
	var rows []*entity.BinExclusion
	if network != "" {
		const q = `SELECT * FROM bin_exclusions WHERE network = $1 ORDER BY start_range`
		if err := r.db.SelectContext(ctx, &rows, q, network); err != nil {
			return nil, fmt.Errorf("ListBinExclusions: %w", err)
		}
	} else {
		const q = `SELECT * FROM bin_exclusions ORDER BY network, start_range`
		if err := r.db.SelectContext(ctx, &rows, q); err != nil {
			return nil, fmt.Errorf("ListBinExclusions: %w", err)
		}
	}
	return rows, nil
}

// GetBinExclusion returns a BIN exclusion by ID.
func (r *NetworkRuleRepo) GetBinExclusion(ctx context.Context, id int64) (*entity.BinExclusion, error) {
	var row entity.BinExclusion
	const q = `SELECT * FROM bin_exclusions WHERE id = $1`
	if err := r.db.GetContext(ctx, &row, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("GetBinExclusion: %w", err)
	}
	return &row, nil
}

// CreateBinExclusion inserts a new BIN exclusion.
func (r *NetworkRuleRepo) CreateBinExclusion(ctx context.Context, exc *entity.BinExclusion) error {
	const q = `
		INSERT INTO bin_exclusions (network, start_range, end_range, description,
			allow_frictionless, allow_otp, allow_oob, default_challenge_channel,
			second_factor_mode, mfa_provider_id, active)
		VALUES (:network, :start_range, :end_range, :description,
			:allow_frictionless, :allow_otp, :allow_oob, :default_challenge_channel,
			:second_factor_mode, :mfa_provider_id, :active)
		RETURNING id`
	rows, err := r.db.NamedQueryContext(ctx, q, exc)
	if err != nil {
		return fmt.Errorf("CreateBinExclusion: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		rows.Scan(&exc.ID)
	}
	return nil
}

// UpdateBinExclusion updates an existing BIN exclusion.
func (r *NetworkRuleRepo) UpdateBinExclusion(ctx context.Context, exc *entity.BinExclusion) error {
	const q = `
		UPDATE bin_exclusions SET
			network = :network,
			start_range = :start_range,
			end_range = :end_range,
			description = :description,
			allow_frictionless = :allow_frictionless,
			allow_otp = :allow_otp,
			allow_oob = :allow_oob,
			default_challenge_channel = :default_challenge_channel,
			second_factor_mode = :second_factor_mode,
			mfa_provider_id = :mfa_provider_id,
			active = :active
		WHERE id = :id`
	_, err := r.db.NamedExecContext(ctx, q, exc)
	if err != nil {
		return fmt.Errorf("UpdateBinExclusion: %w", err)
	}
	return nil
}

// DeleteBinExclusion removes a BIN exclusion by ID.
func (r *NetworkRuleRepo) DeleteBinExclusion(ctx context.Context, id int64) error {
	const q = `DELETE FROM bin_exclusions WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("DeleteBinExclusion: %w", err)
	}
	return nil
}

// ── Flow Resolution ─────────────────────────────────────────────────────────

// ResolveFlowSettings determines the flow-control settings for a given PAN on a network.
// It first checks for a matching BIN exclusion; if none, falls back to the network rule.
func (r *NetworkRuleRepo) ResolveFlowSettings(ctx context.Context, network entity.Network, pan string) (*entity.FlowSettings, error) {
	// Check for BIN exclusion match
	var exc entity.BinExclusion
	const excQ = `
		SELECT * FROM bin_exclusions
		WHERE network = $1 AND active = true
		  AND LEFT($2, LENGTH(start_range)) BETWEEN start_range AND end_range
		ORDER BY LENGTH(start_range) DESC
		LIMIT 1`
	err := r.db.GetContext(ctx, &exc, excQ, string(network), pan)
	if err == nil {
		return &entity.FlowSettings{
			AllowFrictionless:       exc.AllowFrictionless,
			AllowOTP:                exc.AllowOTP,
			AllowOOB:                exc.AllowOOB,
			DefaultChallengeChannel: exc.DefaultChallengeChannel,
			SecondFactorMode:        exc.SecondFactorMode,
			MFAProviderID:           exc.MFAProviderID,
			Source:                  "bin_exclusion",
		}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("ResolveFlowSettings(exclusion): %w", err)
	}

	// Fall back to network rule
	var rule entity.NetworkRule
	const ruleQ = `SELECT * FROM network_rules WHERE network = $1`
	err = r.db.GetContext(ctx, &rule, ruleQ, string(network))
	if err == nil {
		return &entity.FlowSettings{
			AllowFrictionless:       rule.AllowFrictionless,
			AllowOTP:                rule.AllowOTP,
			AllowOOB:                rule.AllowOOB,
			DefaultChallengeChannel: rule.DefaultChallengeChannel,
			SecondFactorMode:        rule.SecondFactorMode,
			MFAProviderID:           rule.MFAProviderID,
			Source:                  "network_rule",
		}, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		// No rule at all — permissive default
		return &entity.FlowSettings{
			AllowFrictionless:       true,
			AllowOTP:                true,
			AllowOOB:                true,
			DefaultChallengeChannel: "otp",
			Source:                  "default",
		}, nil
	}
	return nil, fmt.Errorf("ResolveFlowSettings(rule): %w", err)
}