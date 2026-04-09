package hsm

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Store struct {
	db *sqlx.DB
}

func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// ── Endpoints ───────────────────────────────────────────────────

func (s *Store) ListEndpoints(ctx context.Context) ([]HSMEndpoint, error) {
	var eps []HSMEndpoint
	err := s.db.SelectContext(ctx, &eps, `SELECT * FROM sm_hsm_endpoints ORDER BY priority, id`)
	if err != nil {
		return nil, err
	}
	return eps, nil
}

func (s *Store) GetEndpoint(ctx context.Context, id int) (*HSMEndpoint, error) {
	var ep HSMEndpoint
	if err := s.db.GetContext(ctx, &ep, `SELECT * FROM sm_hsm_endpoints WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &ep, nil
}

func (s *Store) GetActiveEndpoints(ctx context.Context) ([]HSMEndpoint, error) {
	var eps []HSMEndpoint
	err := s.db.SelectContext(ctx, &eps, `SELECT * FROM sm_hsm_endpoints WHERE enabled=true ORDER BY priority, id`)
	if err != nil {
		return nil, err
	}
	return eps, nil
}

func (s *Store) CreateEndpoint(ctx context.Context, ep *HSMEndpoint) (int, error) {
	var id int
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO sm_hsm_endpoints (name, model, address, port, protocol, header_length,
		 firmware_version, host_command_timeout, connection_timeout, max_connections,
		 lmk_identifier, config, weight, priority, capabilities, enabled, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) RETURNING id`,
		ep.Name, ep.Model, ep.Address, ep.Port, ep.Protocol, ep.HeaderLength,
		ep.FirmwareVersion, ep.HostCommandTimeout, ep.ConnectionTimeout, ep.MaxConnections,
		ep.LMKIdentifier, ep.Config, ep.Weight, ep.Priority,
		ep.Capabilities, ep.Enabled, ep.CreatedBy).Scan(&id)
	return id, err
}

func (s *Store) UpdateEndpoint(ctx context.Context, id int, ep *HSMEndpoint) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sm_hsm_endpoints SET name=$1, model=$2, address=$3, port=$4, protocol=$5,
		 header_length=$6, firmware_version=$7, host_command_timeout=$8, connection_timeout=$9,
		 max_connections=$10, lmk_identifier=$11, config=$12, weight=$13, priority=$14,
		 capabilities=$15, enabled=$16, updated_at=now() WHERE id=$17`,
		ep.Name, ep.Model, ep.Address, ep.Port, ep.Protocol, ep.HeaderLength,
		ep.FirmwareVersion, ep.HostCommandTimeout, ep.ConnectionTimeout, ep.MaxConnections,
		ep.LMKIdentifier, ep.Config, ep.Weight, ep.Priority,
		ep.Capabilities, ep.Enabled, id)
	return err
}

func (s *Store) DeleteEndpoint(ctx context.Context, id int) error {
	// Delete dependent rows first (no ON DELETE CASCADE in schema)
	s.db.ExecContext(ctx, `DELETE FROM sm_hsm_metrics WHERE endpoint_id=$1`, id)   //nolint:errcheck
	s.db.ExecContext(ctx, `DELETE FROM sm_hsm_audit_log WHERE endpoint_id=$1`, id) //nolint:errcheck
	_, err := s.db.ExecContext(ctx, `DELETE FROM sm_hsm_endpoints WHERE id=$1`, id)
	return err
}

func (s *Store) UpdateEndpointStatus(ctx context.Context, id int, status string, healthOK bool, errMsg NullString) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sm_hsm_endpoints SET status=$1, last_health_ok=$2, last_health_check=now(),
		 error_message=$3, updated_at=now() WHERE id=$4`,
		status, healthOK, errMsg, id)
	return err
}

// ── Endpoint Groups ────────────────────────────────────────────

func (s *Store) ListEndpointGroups(ctx context.Context) ([]HSMEndpointGroup, error) {
	var groups []HSMEndpointGroup
	err := s.db.SelectContext(ctx, &groups, `SELECT * FROM sm_hsm_endpoint_groups ORDER BY name`)
	if err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *Store) GetEndpointGroup(ctx context.Context, id int) (*HSMEndpointGroup, error) {
	var g HSMEndpointGroup
	if err := s.db.GetContext(ctx, &g, `SELECT * FROM sm_hsm_endpoint_groups WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Store) CreateEndpointGroup(ctx context.Context, g *HSMEndpointGroup) (int, error) {
	var id int
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO sm_hsm_endpoint_groups (name, description, strategy, enabled, created_by)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		g.Name, g.Description, g.Strategy, g.Enabled, g.CreatedBy).Scan(&id)
	return id, err
}

func (s *Store) UpdateEndpointGroup(ctx context.Context, id int, g *HSMEndpointGroup) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sm_hsm_endpoint_groups SET name=$1, description=$2, strategy=$3, enabled=$4, updated_at=now() WHERE id=$5`,
		g.Name, g.Description, g.Strategy, g.Enabled, id)
	return err
}

func (s *Store) DeleteEndpointGroup(ctx context.Context, id int) error {
	s.db.ExecContext(ctx, `UPDATE sm_hsm_endpoints SET group_id=NULL WHERE group_id=$1`, id) //nolint:errcheck
	_, err := s.db.ExecContext(ctx, `DELETE FROM sm_hsm_endpoint_groups WHERE id=$1`, id)
	return err
}

func (s *Store) GetEndpointsByGroup(ctx context.Context, groupID int) ([]HSMEndpoint, error) {
	var eps []HSMEndpoint
	err := s.db.SelectContext(ctx, &eps,
		`SELECT * FROM sm_hsm_endpoints WHERE group_id=$1 AND enabled=true ORDER BY priority, id`, groupID)
	if err != nil {
		return nil, err
	}
	return eps, nil
}

func (s *Store) SetEndpointGroup(ctx context.Context, endpointID int, groupID *int) error {
	if groupID == nil {
		_, err := s.db.ExecContext(ctx,
			`UPDATE sm_hsm_endpoints SET group_id=NULL, updated_at=now() WHERE id=$1`, endpointID)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE sm_hsm_endpoints SET group_id=$1, updated_at=now() WHERE id=$2`, *groupID, endpointID)
	return err
}

// ── Audit Log ───────────────────────────────────────────────────

func (s *Store) LogAudit(ctx context.Context, entry *HSMAuditEntry) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sm_hsm_audit_log (group_id, endpoint_id, command_code, api_operation,
		 request_size, response_size, response_code, duration_ms, error_message, user_id, ip_address)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		entry.GroupID, entry.EndpointID, entry.CommandCode, entry.APIOperation,
		entry.RequestSize, entry.ResponseSize, entry.ResponseCode,
		entry.DurationMS, entry.ErrorMessage, entry.UserID, entry.IPAddress)
	return err
}

func (s *Store) GetAuditLog(ctx context.Context, limit int, endpointID *int) ([]HSMAuditEntry, error) {
	var entries []HSMAuditEntry
	if limit <= 0 {
		limit = 100
	}
	if endpointID != nil {
		err := s.db.SelectContext(ctx, &entries,
			`SELECT * FROM sm_hsm_audit_log WHERE endpoint_id=$1 ORDER BY created_at DESC LIMIT $2`,
			*endpointID, limit)
		return entries, err
	}
	err := s.db.SelectContext(ctx, &entries,
		`SELECT * FROM sm_hsm_audit_log ORDER BY created_at DESC LIMIT $1`, limit)
	return entries, err
}

// ── Metrics ─────────────────────────────────────────────────────

func (s *Store) GetMetrics(ctx context.Context, endpointID int, limit int) ([]HSMMetrics, error) {
	var metrics []HSMMetrics
	if limit <= 0 {
		limit = 24
	}
	err := s.db.SelectContext(ctx, &metrics,
		`SELECT * FROM sm_hsm_metrics WHERE endpoint_id=$1 ORDER BY period_start DESC LIMIT $2`,
		endpointID, limit)
	return metrics, err
}

func (s *Store) GetAllMetricsSummary(ctx context.Context) ([]HSMMetrics, error) {
	var metrics []HSMMetrics
	err := s.db.SelectContext(ctx, &metrics,
		`SELECT * FROM sm_hsm_metrics WHERE period_end > now() - interval '1 hour' ORDER BY period_start DESC`)
	return metrics, err
}
