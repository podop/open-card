package secrets

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"time"
)

// ── Nullable JSON wrappers ─────────────────────────────────────

type NullString sql.NullString

func (n NullString) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.String)
}

func (n *NullString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	n.Valid = true
	return json.Unmarshal(data, &n.String)
}

func (n *NullString) Scan(value interface{}) error {
	s := sql.NullString{String: n.String, Valid: n.Valid}
	if err := s.Scan(value); err != nil {
		return err
	}
	n.String, n.Valid = s.String, s.Valid
	return nil
}

func (n NullString) Value() (driver.Value, error) {
	return sql.NullString(n).Value()
}

type NullInt32 sql.NullInt32

func (n NullInt32) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Int32)
}

func (n *NullInt32) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	n.Valid = true
	return json.Unmarshal(data, &n.Int32)
}

func (n *NullInt32) Scan(value interface{}) error {
	s := sql.NullInt32{Int32: n.Int32, Valid: n.Valid}
	if err := s.Scan(value); err != nil {
		return err
	}
	n.Int32, n.Valid = s.Int32, s.Valid
	return nil
}

func (n NullInt32) Value() (driver.Value, error) {
	return sql.NullInt32(n).Value()
}

type NullTime sql.NullTime

func (n NullTime) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Time)
}

func (n *NullTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	n.Valid = true
	return json.Unmarshal(data, &n.Time)
}

func (n *NullTime) Scan(value interface{}) error {
	s := sql.NullTime{Time: n.Time, Valid: n.Valid}
	if err := s.Scan(value); err != nil {
		return err
	}
	n.Time, n.Valid = s.Time, s.Valid
	return nil
}

func (n NullTime) Value() (driver.Value, error) {
	return sql.NullTime(n).Value()
}

type SecurityProvider struct {
	ID           int             `db:"id" json:"id"`
	Name         string          `db:"name" json:"name"`
	ProviderType string          `db:"provider_type" json:"providerType"`
	Config       json.RawMessage `db:"config" json:"config"`
	Active       bool            `db:"active" json:"active"`
	CreatedBy    int             `db:"created_by" json:"createdBy"`
	CreatedAt    time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updatedAt"`
}

type RotationPolicy struct {
	ID               int    `db:"id" json:"id"`
	Name             string `db:"name" json:"name"`
	Description      string `db:"description" json:"description"`
	KeyType          string `db:"key_type" json:"keyType"`
	RotationInterval string `db:"rotation_interval" json:"rotationInterval"`
	NotifyBefore     string `db:"notify_before" json:"notifyBefore"`
	AutoRotate       bool   `db:"auto_rotate" json:"autoRotate"`
	IsTemplate       bool   `db:"is_template" json:"isTemplate"`
	CreatedBy        int    `db:"created_by" json:"createdBy"`
	CreatedAt        time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt        time.Time `db:"updated_at" json:"updatedAt"`
}

type Key struct {
	ID               int            `db:"id" json:"id"`
	UID              string         `db:"uid" json:"uid"`
	Name             string         `db:"name" json:"name"`
	Description      string         `db:"description" json:"description"`
	KeyType          string         `db:"key_type" json:"keyType"`
	KeyUsage         string         `db:"key_usage" json:"keyUsage"`
	ProviderID       int            `db:"provider_id" json:"providerId"`
	PublicKeyPEM     NullString `db:"public_key_pem" json:"publicKeyPem,omitempty"`
	PrivateKeyEnc    NullString `db:"private_key_enc" json:"-"`
	CertificatePEM   NullString `db:"certificate_pem" json:"certificatePem,omitempty"`
	HSMKeyHandle     NullString `db:"hsm_key_handle" json:"hsmKeyHandle,omitempty"`
	Fingerprint      string         `db:"fingerprint" json:"fingerprint"`
	KeySizeBits      int            `db:"key_size_bits" json:"keySizeBits"`
	NotBefore        NullTime  `db:"not_before" json:"notBefore,omitempty"`
	NotAfter         NullTime  `db:"not_after" json:"notAfter,omitempty"`
	RotationPolicyID NullInt32 `db:"rotation_policy_id" json:"rotationPolicyId,omitempty"`
	LastRotatedAt    NullTime  `db:"last_rotated_at" json:"lastRotatedAt,omitempty"`
	NextRotationAt   NullTime  `db:"next_rotation_at" json:"nextRotationAt,omitempty"`
	Version          int            `db:"version" json:"version"`
	PredecessorID    NullInt32 `db:"predecessor_id" json:"predecessorId,omitempty"`
	OwnerUserID      int            `db:"owner_user_id" json:"ownerUserId"`
	Active           bool           `db:"active" json:"active"`
	CreatedAt        time.Time      `db:"created_at" json:"createdAt"`
	UpdatedAt        time.Time      `db:"updated_at" json:"updatedAt"`
}

type KeyAccess struct {
	ID         int       `db:"id" json:"id"`
	KeyID      int       `db:"key_id" json:"keyId"`
	UserID     int       `db:"user_id" json:"userId"`
	Permission string    `db:"permission" json:"permission"`
	GrantedBy  int       `db:"granted_by" json:"grantedBy"`
	GrantedAt  time.Time `db:"granted_at" json:"grantedAt"`
}

type KeyUsageLog struct {
	ID        int64           `db:"id" json:"id"`
	KeyID     int             `db:"key_id" json:"keyId"`
	UserID    int             `db:"user_id" json:"userId"`
	Action    string          `db:"action" json:"action"`
	IPAddress NullString  `db:"ip_address" json:"ipAddress,omitempty"`
	UserAgent NullString  `db:"user_agent" json:"userAgent,omitempty"`
	Details   json.RawMessage `db:"details" json:"details"`
	CreatedAt time.Time       `db:"created_at" json:"createdAt"`
}

type RotationHistory struct {
	ID           int64          `db:"id" json:"id"`
	KeyID        int            `db:"key_id" json:"keyId"`
	OldVersion   int            `db:"old_version" json:"oldVersion"`
	NewVersion   int            `db:"new_version" json:"newVersion"`
	RotatedBy    NullInt32  `db:"rotated_by" json:"rotatedBy,omitempty"`
	Trigger      string         `db:"trigger" json:"trigger"`
	Status       string         `db:"status" json:"status"`
	ErrorMessage NullString `db:"error_message" json:"errorMessage,omitempty"`
	CreatedAt    time.Time      `db:"created_at" json:"createdAt"`
}
