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

// ── Sessions ────────────────────────────────────────────────────

func (s *Store) ListActiveSessions(ctx context.Context) ([]HSMSession, error) {
	var sessions []HSMSession
	err := s.db.SelectContext(ctx, &sessions,
		`SELECT * FROM sm_hsm_sessions WHERE status='active' ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (s *Store) GetSession(ctx context.Context, id string) (*HSMSession, error) {
	var sess HSMSession
	if err := s.db.GetContext(ctx, &sess, `SELECT * FROM sm_hsm_sessions WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) CreateSession(ctx context.Context, sess *HSMSession) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO sm_hsm_sessions (endpoint_id, client_label, strategy, ttl_seconds, metadata)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		sess.EndpointID, sess.ClientLabel, sess.Strategy, sess.TTLSeconds, sess.Metadata).Scan(&id)
	return id, err
}

func (s *Store) CloseSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sm_hsm_sessions SET status='closed', closed_at=now() WHERE id=$1`, id)
	return err
}

func (s *Store) TouchSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sm_hsm_sessions SET last_ping=now() WHERE id=$1 AND status='active'`, id)
	return err
}

// ── Audit Log ───────────────────────────────────────────────────

func (s *Store) LogAudit(ctx context.Context, entry *HSMAuditEntry) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sm_hsm_audit_log (session_id, endpoint_id, command_code, api_operation,
		 request_size, response_size, response_code, duration_ms, error_message, user_id, ip_address)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		entry.SessionID, entry.EndpointID, entry.CommandCode, entry.APIOperation,
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
