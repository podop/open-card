package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// TestScenario represents a saved test scenario.
type TestScenario struct {
	ID               int64     `db:"id"               json:"id"`
	Name             string    `db:"name"             json:"name"`
	Description      string    `db:"description"      json:"description"`
	FlowType         string    `db:"flow_type"        json:"flowType"`
	DeviceChannel    string    `db:"device_channel"   json:"deviceChannel"`
	Network          string    `db:"network"          json:"network"`
	CardNumber       string    `db:"card_number"      json:"cardNumber"`
	MerchantName     string    `db:"merchant_name"    json:"merchantName"`
	MerchantCountry  string    `db:"merchant_country" json:"merchantCountry"`
	PurchaseAmount   string    `db:"purchase_amount"  json:"purchaseAmount"`
	PurchaseCurrency string    `db:"purchase_currency" json:"purchaseCurrency"`
	ChallengeAction  string    `db:"challenge_action" json:"challengeAction"`
	CreatedAt        time.Time `db:"created_at"       json:"createdAt"`
	UpdatedAt        time.Time `db:"updated_at"       json:"updatedAt"`
}

// ScenarioRun represents a single execution of a test scenario.
type ScenarioRun struct {
	ID               int64            `db:"id"                json:"id"`
	ScenarioID       int64            `db:"scenario_id"       json:"scenarioId"`
	ScenarioName     string           `db:"scenario_name"     json:"scenarioName"`
	FlowType         string           `db:"flow_type"         json:"flowType"`
	ACSTransactionID *uuid.UUID       `db:"acs_transaction_id" json:"acsTransactionId"`
	Status           string           `db:"status"            json:"status"`
	Steps            json.RawMessage  `db:"steps"             json:"steps"`
	ErrorMsg         string           `db:"error_msg"         json:"errorMsg"`
	DurationMs       int              `db:"duration_ms"       json:"durationMs"`
	StartedAt        time.Time        `db:"started_at"        json:"startedAt"`
	CompletedAt      *time.Time       `db:"completed_at"      json:"completedAt"`
}

// ScenarioRepo provides CRUD for test_scenarios and scenario_runs.
type ScenarioRepo struct {
	db *sqlx.DB
}

// NewScenarioRepo creates a new ScenarioRepo.
func NewScenarioRepo(db *sqlx.DB) *ScenarioRepo {
	return &ScenarioRepo{db: db}
}

// ListScenarios returns all test scenarios ordered by name.
func (r *ScenarioRepo) ListScenarios(ctx context.Context) ([]*TestScenario, error) {
	var rows []*TestScenario
	if err := r.db.SelectContext(ctx, &rows, `SELECT * FROM test_scenarios ORDER BY name`); err != nil {
		return nil, fmt.Errorf("ScenarioRepo.ListScenarios: %w", err)
	}
	return rows, nil
}

// GetScenario returns a single test scenario by ID.
func (r *ScenarioRepo) GetScenario(ctx context.Context, id int64) (*TestScenario, error) {
	var s TestScenario
	err := r.db.GetContext(ctx, &s, `SELECT * FROM test_scenarios WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ScenarioRepo.GetScenario: %w", err)
	}
	return &s, nil
}

// CreateScenario inserts a new test scenario.
func (r *ScenarioRepo) CreateScenario(ctx context.Context, s *TestScenario) error {
	const q = `INSERT INTO test_scenarios
		(name, description, flow_type, device_channel, network, card_number,
		 merchant_name, merchant_country, purchase_amount, purchase_currency, challenge_action)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRowContext(ctx, q,
		s.Name, s.Description, s.FlowType, s.DeviceChannel, s.Network, s.CardNumber,
		s.MerchantName, s.MerchantCountry, s.PurchaseAmount, s.PurchaseCurrency, s.ChallengeAction,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

// UpdateScenario updates an existing test scenario.
func (r *ScenarioRepo) UpdateScenario(ctx context.Context, s *TestScenario) error {
	const q = `UPDATE test_scenarios SET
		name=$2, description=$3, flow_type=$4, device_channel=$5, network=$6, card_number=$7,
		merchant_name=$8, merchant_country=$9, purchase_amount=$10, purchase_currency=$11,
		challenge_action=$12, updated_at=now()
		WHERE id=$1`
	_, err := r.db.ExecContext(ctx, q, s.ID,
		s.Name, s.Description, s.FlowType, s.DeviceChannel, s.Network, s.CardNumber,
		s.MerchantName, s.MerchantCountry, s.PurchaseAmount, s.PurchaseCurrency, s.ChallengeAction,
	)
	return err
}

// DeleteScenario removes a test scenario by ID.
func (r *ScenarioRepo) DeleteScenario(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM test_scenarios WHERE id = $1`, id)
	return err
}

// CreateRun inserts a new scenario run.
func (r *ScenarioRepo) CreateRun(ctx context.Context, run *ScenarioRun) error {
	const q = `INSERT INTO scenario_runs
		(scenario_id, scenario_name, flow_type, acs_transaction_id, status, steps, error_msg, duration_ms, started_at, completed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id`
	return r.db.QueryRowContext(ctx, q,
		run.ScenarioID, run.ScenarioName, run.FlowType, run.ACSTransactionID,
		run.Status, run.Steps, run.ErrorMsg, run.DurationMs, run.StartedAt, run.CompletedAt,
	).Scan(&run.ID)
}

// UpdateRun updates a scenario run.
func (r *ScenarioRepo) UpdateRun(ctx context.Context, run *ScenarioRun) error {
	const q = `UPDATE scenario_runs SET
		acs_transaction_id=$2, status=$3, steps=$4, error_msg=$5, duration_ms=$6, completed_at=$7
		WHERE id=$1`
	_, err := r.db.ExecContext(ctx, q, run.ID,
		run.ACSTransactionID, run.Status, run.Steps, run.ErrorMsg, run.DurationMs, run.CompletedAt,
	)
	return err
}

// ListRuns returns paginated scenario runs.
func (r *ScenarioRepo) ListRuns(ctx context.Context, scenarioID int64, search string, page, perPage int) ([]*ScenarioRun, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 200 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	where := "WHERE 1=1"
	args := []interface{}{}
	n := 1
	if scenarioID > 0 {
		where += fmt.Sprintf(" AND scenario_id = $%d", n)
		args = append(args, scenarioID)
		n++
	}
	if search != "" {
		like := "%" + search + "%"
		where += fmt.Sprintf(
			" AND (scenario_name ILIKE $%d OR flow_type ILIKE $%d OR status ILIKE $%d OR COALESCE(acs_transaction_id::text,'') ILIKE $%d OR COALESCE(error_msg,'') ILIKE $%d)",
			n, n, n, n, n)
		args = append(args, like)
		n++
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM scenario_runs "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := fmt.Sprintf("SELECT * FROM scenario_runs %s ORDER BY started_at DESC LIMIT $%d OFFSET $%d", where, n, n+1)
	args = append(args, perPage, offset)

	var rows []*ScenarioRun
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetRun returns a single scenario run by ID.
func (r *ScenarioRepo) GetRun(ctx context.Context, id int64) (*ScenarioRun, error) {
	var run ScenarioRun
	err := r.db.GetContext(ctx, &run, `SELECT * FROM scenario_runs WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// RunStats returns aggregate statistics for scenario runs.
type RunStats struct {
	TotalRuns      int64   `db:"total_runs"      json:"totalRuns"`
	CompletedRuns  int64   `db:"completed_runs"  json:"completedRuns"`
	FailedRuns     int64   `db:"failed_runs"     json:"failedRuns"`
	AvgDurationMs  float64 `db:"avg_duration_ms" json:"avgDurationMs"`
}

type FlowStats struct {
	FlowType  string `db:"flow_type" json:"flowType"`
	Total     int64  `db:"total"     json:"total"`
	Completed int64  `db:"completed" json:"completed"`
	Failed    int64  `db:"failed"    json:"failed"`
}

func (r *ScenarioRepo) GetRunStats(ctx context.Context) (*RunStats, []FlowStats, error) {
	var stats RunStats
	err := r.db.GetContext(ctx, &stats, `
		SELECT
			COUNT(*) as total_runs,
			COUNT(*) FILTER (WHERE status='completed') as completed_runs,
			COUNT(*) FILTER (WHERE status='failed') as failed_runs,
			COALESCE(AVG(duration_ms) FILTER (WHERE status='completed'), 0) as avg_duration_ms
		FROM scenario_runs`)
	if err != nil {
		return nil, nil, err
	}

	var flowStats []FlowStats
	err = r.db.SelectContext(ctx, &flowStats, `
		SELECT flow_type,
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status='completed') as completed,
			COUNT(*) FILTER (WHERE status='failed') as failed
		FROM scenario_runs
		GROUP BY flow_type
		ORDER BY flow_type`)
	if err != nil {
		return nil, nil, err
	}
	return &stats, flowStats, nil
}

// RecentRuns returns the last N runs.
func (r *ScenarioRepo) RecentRuns(ctx context.Context, limit int) ([]*ScenarioRun, error) {
	var rows []*ScenarioRun
	err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM scenario_runs ORDER BY started_at DESC LIMIT $1`, limit)
	return rows, err
}

// GetTransactionStatus returns the trans_status for a given ACS transaction ID.
func (r *ScenarioRepo) GetTransactionStatus(ctx context.Context, acsTransID uuid.UUID) (string, error) {
	var status string
	err := r.db.QueryRowContext(ctx, `SELECT trans_status FROM transactions WHERE acs_transaction_id = $1`, acsTransID).Scan(&status)
	return status, err
}

// GetOTPForTransaction retrieves the current OTP from the most recent
// auth_challenge_start or challenge_fail audit log entry for the given ACS transaction ID.
// This is a TEST-ONLY helper — OTP is stored in the audit detail JSON.
func (r *ScenarioRepo) GetOTPForTransaction(ctx context.Context, acsTransID uuid.UUID) (string, error) {
	var detail json.RawMessage
	err := r.db.QueryRowContext(ctx, `
		SELECT detail FROM audit_log
		WHERE acs_transaction_id = $1
		  AND event_type IN ('auth_challenge_start', 'challenge_fail')
		  AND detail IS NOT NULL
		ORDER BY created_at DESC
		LIMIT 1`, acsTransID).Scan(&detail)
	if err != nil {
		return "", err
	}
	var d map[string]interface{}
	if err := json.Unmarshal(detail, &d); err != nil {
		return "", err
	}
	// Check "otp" first (from challenge_start), then "new_otp" (from fail/rotate)
	if otp, ok := d["otp"].(string); ok && otp != "" {
		return otp, nil
	}
	if otp, ok := d["new_otp"].(string); ok && otp != "" {
		return otp, nil
	}
	return "", fmt.Errorf("no OTP found in audit detail")
}

// GetPINForTransaction retrieves the current PIN from the most recent
// challenge_success or challenge_fail audit log entry that contains a PIN.
// This is a TEST-ONLY helper — PIN is stored in the audit detail JSON.
func (r *ScenarioRepo) GetPINForTransaction(ctx context.Context, acsTransID uuid.UUID) (string, error) {
	var detail json.RawMessage
	err := r.db.QueryRowContext(ctx, `
		SELECT detail FROM audit_log
		WHERE acs_transaction_id = $1
		  AND event_type IN ('challenge_success', 'challenge_fail')
		  AND detail IS NOT NULL
		ORDER BY created_at DESC
		LIMIT 1`, acsTransID).Scan(&detail)
	if err != nil {
		return "", err
	}
	var d map[string]interface{}
	if err := json.Unmarshal(detail, &d); err != nil {
		return "", err
	}
	// Check "pin" first (from first_factor success), then "new_pin" (from pin fail/rotate)
	if pin, ok := d["pin"].(string); ok && pin != "" {
		return pin, nil
	}
	if pin, ok := d["new_pin"].(string); ok && pin != "" {
		return pin, nil
	}
	return "", fmt.Errorf("no PIN found in audit detail")
}