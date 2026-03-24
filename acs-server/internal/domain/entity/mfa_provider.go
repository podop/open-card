package entity

import (
	"encoding/json"
	"time"
)

// MFAProvider represents a registered multi-factor authentication provider.
type MFAProvider struct {
	ID          int64           `db:"id"           json:"id"`
	Name        string          `db:"name"         json:"name"`
	DisplayName string          `db:"display_name" json:"displayName"`
	Type        string          `db:"type"         json:"type"`       // "internal" | "external"
	FactorType  string          `db:"factor_type"  json:"factorType"` // "card_pin" | "authenticator" | "duo" etc.
	Active      bool            `db:"active"       json:"active"`
	Config      json.RawMessage `db:"config"       json:"config"`
	Description string          `db:"description"  json:"description"`
	CreatedAt   time.Time       `db:"created_at"   json:"createdAt"`
	UpdatedAt   time.Time       `db:"updated_at"   json:"updatedAt"`
}
