package secrets

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

// ── Security Providers ──────────────────────────────────────────

func (s *Store) ListProviders(ctx context.Context) ([]SecurityProvider, error) {
	var providers []SecurityProvider
	err := s.db.SelectContext(ctx, &providers, `SELECT * FROM sm_security_providers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return providers, nil
}

func (s *Store) GetProvider(ctx context.Context, id int) (*SecurityProvider, error) {
	var p SecurityProvider
	if err := s.db.GetContext(ctx, &p, `SELECT * FROM sm_security_providers WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) GetProviderByName(ctx context.Context, name string) (*SecurityProvider, error) {
	var p SecurityProvider
	if err := s.db.GetContext(ctx, &p, `SELECT * FROM sm_security_providers WHERE name=$1`, name); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) CreateProvider(ctx context.Context, p *SecurityProvider) (int, error) {
	var id int
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO sm_security_providers (name, provider_type, config, active, created_by)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		p.Name, p.ProviderType, p.Config, p.Active, p.CreatedBy).Scan(&id)
	return id, err
}

func (s *Store) UpdateProvider(ctx context.Context, id int, name string, config []byte, active bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sm_security_providers SET name=$1, config=$2, active=$3, updated_at=now() WHERE id=$4`,
		name, config, active, id)
	return err
}

func (s *Store) DeleteProvider(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sm_security_providers WHERE id=$1`, id)
	return err
}

func (s *Store) CountKeysByProvider(ctx context.Context, providerID int) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM sm_keys WHERE provider_id=$1`, providerID)
	return count, err
}

// ── Rotation Policies ───────────────────────────────────────────

func (s *Store) ListPolicies(ctx context.Context) ([]RotationPolicy, error) {
	var policies []RotationPolicy
	err := s.db.SelectContext(ctx, &policies, `SELECT * FROM sm_rotation_policies ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return policies, nil
}

func (s *Store) GetPolicy(ctx context.Context, id int) (*RotationPolicy, error) {
	var p RotationPolicy
	if err := s.db.GetContext(ctx, &p, `SELECT * FROM sm_rotation_policies WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) CreatePolicy(ctx context.Context, p *RotationPolicy) (int, error) {
	var id int
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO sm_rotation_policies (name, description, key_type, rotation_interval, notify_before, auto_rotate, is_template, created_by)
		 VALUES ($1, $2, $3, $4::interval, $5::interval, $6, $7, $8) RETURNING id`,
		p.Name, p.Description, p.KeyType, p.RotationInterval, p.NotifyBefore, p.AutoRotate, p.IsTemplate, p.CreatedBy).Scan(&id)
	return id, err
}

func (s *Store) UpdatePolicy(ctx context.Context, id int, p *RotationPolicy) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sm_rotation_policies SET name=$1, description=$2, key_type=$3,
		 rotation_interval=$4::interval, notify_before=$5::interval, auto_rotate=$6, updated_at=now()
		 WHERE id=$7`,
		p.Name, p.Description, p.KeyType, p.RotationInterval, p.NotifyBefore, p.AutoRotate, id)
	return err
}

func (s *Store) DeletePolicy(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sm_rotation_policies WHERE id=$1`, id)
	return err
}

func (s *Store) PolicyExists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := s.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM sm_rotation_policies WHERE name=$1)`, name)
	return exists, err
}

// ── Keys ────────────────────────────────────────────────────────

func (s *Store) ListKeys(ctx context.Context) ([]Key, error) {
	var keys []Key
	err := s.db.SelectContext(ctx, &keys, `SELECT * FROM sm_keys ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *Store) GetKey(ctx context.Context, id int) (*Key, error) {
	var k Key
	if err := s.db.GetContext(ctx, &k, `SELECT * FROM sm_keys WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &k, nil
}

func (s *Store) CreateKey(ctx context.Context, k *Key) (int, error) {
	var id int
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO sm_keys (name, description, key_type, key_usage, provider_id,
		 public_key_pem, private_key_enc, certificate_pem, hsm_key_handle,
		 fingerprint, key_size_bits, not_before, not_after,
		 rotation_policy_id, last_rotated_at, next_rotation_at,
		 version, predecessor_id, owner_user_id, active)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
		 RETURNING id`,
		k.Name, k.Description, k.KeyType, k.KeyUsage, k.ProviderID,
		k.PublicKeyPEM, k.PrivateKeyEnc, k.CertificatePEM, k.HSMKeyHandle,
		k.Fingerprint, k.KeySizeBits, k.NotBefore, k.NotAfter,
		k.RotationPolicyID, k.LastRotatedAt, k.NextRotationAt,
		k.Version, k.PredecessorID, k.OwnerUserID, k.Active).Scan(&id)
	return id, err
}

func (s *Store) UpdateKey(ctx context.Context, id int, name, description, keyUsage string, active bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sm_keys SET name=$1, description=$2, key_usage=$3, active=$4, updated_at=now() WHERE id=$5`,
		name, description, keyUsage, active, id)
	return err
}

func (s *Store) DeleteKey(ctx context.Context, id int) error {
	// Clear predecessor references so successor keys are not orphaned
	s.db.ExecContext(ctx, `UPDATE sm_keys SET predecessor_id=NULL WHERE predecessor_id=$1`, id) //nolint:errcheck
	_, err := s.db.ExecContext(ctx, `DELETE FROM sm_keys WHERE id=$1`, id)
	return err
}

func (s *Store) DeactivateKey(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sm_keys SET active=false, updated_at=now() WHERE id=$1`, id)
	return err
}

func (s *Store) AssignPolicy(ctx context.Context, keyID, policyID int, nextRotation NullTime) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sm_keys SET rotation_policy_id=$1, next_rotation_at=$2, updated_at=now() WHERE id=$3`,
		policyID, nextRotation, keyID)
	return err
}

func (s *Store) GetKeysForRotation(ctx context.Context) ([]Key, error) {
	var keys []Key
	err := s.db.SelectContext(ctx, &keys,
		`SELECT * FROM sm_keys WHERE active=true AND next_rotation_at <= now() AND rotation_policy_id IS NOT NULL`)
	return keys, err
}

// ── Key Access ──────────────────────────────────────────────────

func (s *Store) ListKeyAccess(ctx context.Context, keyID int) ([]KeyAccess, error) {
	var access []KeyAccess
	err := s.db.SelectContext(ctx, &access, `SELECT * FROM sm_key_access WHERE key_id=$1 ORDER BY id`, keyID)
	if err != nil {
		return nil, err
	}
	return access, nil
}

func (s *Store) GrantKeyAccess(ctx context.Context, a *KeyAccess) (int, error) {
	var id int
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO sm_key_access (key_id, user_id, permission, granted_by)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (key_id, user_id) DO UPDATE SET permission=EXCLUDED.permission, granted_by=EXCLUDED.granted_by, granted_at=now()
		 RETURNING id`,
		a.KeyID, a.UserID, a.Permission, a.GrantedBy).Scan(&id)
	return id, err
}

func (s *Store) RevokeKeyAccess(ctx context.Context, keyID, userID int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sm_key_access WHERE key_id=$1 AND user_id=$2`, keyID, userID)
	return err
}

func (s *Store) CopyKeyAccess(ctx context.Context, fromKeyID, toKeyID int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sm_key_access (key_id, user_id, permission, granted_by)
		 SELECT $2, user_id, permission, granted_by FROM sm_key_access WHERE key_id=$1
		 ON CONFLICT (key_id, user_id) DO NOTHING`,
		fromKeyID, toKeyID)
	return err
}

// ── Key Usage Log ───────────────────────────────────────────────

func (s *Store) LogKeyUsage(ctx context.Context, log *KeyUsageLog) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sm_key_usage_log (key_id, user_id, action, ip_address, user_agent, details)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		log.KeyID, log.UserID, log.Action, log.IPAddress, log.UserAgent, log.Details)
	return err
}

func (s *Store) GetKeyUsageLog(ctx context.Context, keyID int) ([]KeyUsageLog, error) {
	var logs []KeyUsageLog
	err := s.db.SelectContext(ctx, &logs,
		`SELECT * FROM sm_key_usage_log WHERE key_id=$1 ORDER BY created_at DESC LIMIT 100`, keyID)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// ── Rotation History ────────────────────────────────────────────

func (s *Store) LogRotation(ctx context.Context, h *RotationHistory) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sm_rotation_history (key_id, old_version, new_version, rotated_by, trigger, status, error_message)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		h.KeyID, h.OldVersion, h.NewVersion, h.RotatedBy, h.Trigger, h.Status, h.ErrorMessage)
	return err
}

func (s *Store) ProviderExists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := s.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM sm_security_providers WHERE name=$1)`, name)
	return exists, err
}
