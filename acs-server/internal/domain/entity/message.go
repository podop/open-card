package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Message records a raw 3DS protocol message for compliance and debugging.
type Message struct {
	ID               int64           `db:"id"`
	ACSTransactionID *uuid.UUID      `db:"acs_transaction_id"` // nil for system-level messages (OReq/ORes)
	MessageType      string          `db:"message_type"`       // AReq | ARes | CReq | CRes | RReq | RRes | OReq | ORes | Erro
	Direction        string          `db:"direction"`          // inbound | outbound
	Content          json.RawMessage `db:"content"`
	CreatedAt        time.Time       `db:"created_at"`
}