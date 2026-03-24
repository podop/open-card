package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AuditLogRow is a flat read model for rows in the audit_log table.
type AuditLogRow struct {
	ID               int64           `db:"id"`
	ACSTransactionID uuid.UUID       `db:"acs_transaction_id"`
	DSTransactionID  uuid.UUID       `db:"ds_transaction_id"`
	EventType        string          `db:"event_type"`
	TransStatus      string          `db:"trans_status"`
	CardBIN          string          `db:"card_bin"`
	MerchantCountry  string          `db:"merchant_country"`
	DeviceChannel    string          `db:"device_channel"`
	SpecVersion      string          `db:"spec_version"`
	Detail           json.RawMessage `db:"detail"`
	CreatedAt        time.Time       `db:"created_at"`
}