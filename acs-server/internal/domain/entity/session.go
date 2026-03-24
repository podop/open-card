package entity

import (
	"time"

	"github.com/google/uuid"
)

// SessionStatus tracks lifecycle of a 3DS Method or Challenge session.
type SessionStatus string

const (
	SessionStatusPending   SessionStatus = "pending"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusExpired   SessionStatus = "expired"
)

// ACSSession holds transient state for browser-based 3DS Method and challenge flows.
// Stored in PostgreSQL with a short TTL; suitable for migration to Redis later.
type ACSSession struct {
	ID            uuid.UUID     `db:"id"`
	TransactionID uuid.UUID     `db:"transaction_id"`
	SessionType   string        `db:"session_type"` // "method" | "challenge"
	Status        SessionStatus `db:"status"`
	NotifURL      string        `db:"notif_url"` // threeDSMethodNotificationURL
	DeviceData    []byte        `db:"device_data"` // raw 3DS Method form data (encrypted)
	ExpiresAt     time.Time     `db:"expires_at"`
	CreatedAt     time.Time     `db:"created_at"`
}

// NewMethodSession creates a 3DS Method session (device fingerprinting).
func NewMethodSession(transactionID uuid.UUID, notifURL string) *ACSSession {
	now := time.Now().UTC()
	return &ACSSession{
		ID:            uuid.New(),
		TransactionID: transactionID,
		SessionType:   "method",
		Status:        SessionStatusPending,
		NotifURL:      notifURL,
		ExpiresAt:     now.Add(10 * time.Minute),
		CreatedAt:     now,
	}
}

// NewChallengeSession creates a challenge session.
func NewChallengeSession(transactionID uuid.UUID) *ACSSession {
	now := time.Now().UTC()
	return &ACSSession{
		ID:            uuid.New(),
		TransactionID: transactionID,
		SessionType:   "challenge",
		Status:        SessionStatusPending,
		ExpiresAt:     now.Add(10 * time.Minute),
		CreatedAt:     now,
	}
}

// IsExpired returns true if the session TTL has passed.
func (s *ACSSession) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}
