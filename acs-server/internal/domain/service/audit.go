package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/acs-server/internal/domain/entity"
)

// AuditEventType classifies authentication lifecycle events.
type AuditEventType string

const (
	AuditEventFrictionless    AuditEventType = "auth_frictionless"
	AuditEventChallengeStart  AuditEventType = "auth_challenge_start"
	AuditEventChallengeSuccess AuditEventType = "challenge_success"
	AuditEventChallengeFail   AuditEventType = "challenge_fail"
	AuditEventUnavailable       AuditEventType = "auth_unavailable"
	AuditEventMethodCollected   AuditEventType = "method_collected"
	AuditEventChallengeCancel   AuditEventType = "challenge_cancel"
	AuditEventChallengeTimeout  AuditEventType = "challenge_timeout"
	AuditEventDecoupledStart    AuditEventType = "decoupled_start"
	AuditEventDecoupledComplete AuditEventType = "decoupled_complete"
)

// AuditEvent is a single immutable record of a 3DS lifecycle event.
type AuditEvent struct {
	ACSTransactionID uuid.UUID
	DSTransactionID  uuid.UUID
	EventType        AuditEventType
	TransStatus      entity.TransStatus
	CardBIN          string
	MerchantCountry  string
	DeviceChannel    entity.DeviceChannel
	SpecVersion      entity.SpecVersion
	Detail           map[string]any
	CreatedAt        time.Time
}

// AuditLogger persists audit events.
// Implementations can write to PostgreSQL, an event stream, or both.
type AuditLogger interface {
	Log(ctx context.Context, event *AuditEvent) error
}

// NoopAuditLogger silently discards events (useful in tests).
type NoopAuditLogger struct{}

func (n *NoopAuditLogger) Log(_ context.Context, _ *AuditEvent) error { return nil }

// NewAuditEvent constructs an AuditEvent from a Transaction and an event type.
func NewAuditEvent(tx *entity.Transaction, evt AuditEventType, detail map[string]any) *AuditEvent {
	return &AuditEvent{
		ACSTransactionID: tx.ACSTransactionID,
		DSTransactionID:  tx.DSTransactionID,
		EventType:        evt,
		TransStatus:      tx.TransStatus,
		CardBIN:          tx.CardBIN,
		MerchantCountry:  tx.MerchantCountry,
		DeviceChannel:    tx.DeviceChannel,
		SpecVersion:      tx.SpecVersion,
		Detail:           detail,
		CreatedAt:        time.Now().UTC(),
	}
}
