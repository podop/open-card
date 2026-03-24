package entity

import (
	"time"

	"github.com/google/uuid"
)

// SpecVersion represents supported EMV 3DS protocol versions.
type SpecVersion string

const (
	SpecVersion220 SpecVersion = "2.2.0"
	SpecVersion231 SpecVersion = "2.3.1"
)

// TransStatus represents the outcome of an authentication attempt.
type TransStatus string

const (
	TransStatusAuthenticated          TransStatus = "Y" // Frictionless success
	TransStatusNotAuthenticated       TransStatus = "N" // Authentication failed
	TransStatusChallenge              TransStatus = "C" // Challenge required
	TransStatusDecoupled              TransStatus = "D" // Decoupled authentication
	TransStatusInformationalOnly      TransStatus = "I" // Informational only
	TransStatusAttemptProcessing      TransStatus = "A" // Attempts processing performed
	TransStatusUnavailable            TransStatus = "U" // Authentication could not be performed
	TransStatusRejected               TransStatus = "R" // Authentication rejected
)

// DeviceChannel identifies the type of channel used for the transaction.
type DeviceChannel string

const (
	DeviceChannelApp     DeviceChannel = "01" // App-based
	DeviceChannelBrowser DeviceChannel = "02" // Browser-based
	DeviceChannelThreeRI DeviceChannel = "03" // 3DS Requestor Initiated
)

// MessageCategory distinguishes payment from non-payment authentications.
type MessageCategory string

const (
	MessageCategoryPayment    MessageCategory = "01"
	MessageCategoryNonPayment MessageCategory = "02"
)

// Transaction is the core aggregate representing a single 3DS authentication lifecycle.
type Transaction struct {
	ID                  uuid.UUID       `db:"id"`
	ACSTransactionID    uuid.UUID       `db:"acs_transaction_id"`
	DSTransactionID     uuid.UUID       `db:"ds_transaction_id"`
	ServerTransactionID uuid.UUID       `db:"server_transaction_id"`
	SDKTransactionID    *uuid.UUID      `db:"sdk_transaction_id"` // nil for browser-based

	// Protocol
	SpecVersion     SpecVersion     `db:"spec_version"`
	MessageCategory MessageCategory `db:"message_category"`
	DeviceChannel   DeviceChannel   `db:"device_channel"`

	// Card
	AcctNumber string  `db:"acct_number"` // PAN or token (stored encrypted)
	CardBIN    string  `db:"card_bin"`
	Network    Network `db:"network"` // card scheme resolved from CardRange

	// Merchant
	MerchantName    string `db:"merchant_name"`
	MerchantCountry string `db:"merchant_country"`
	MerchantID      string `db:"merchant_id"`

	// Amount
	PurchaseAmount   *string `db:"purchase_amount"`
	PurchaseCurrency *string `db:"purchase_currency"`
	PurchaseExponent *int    `db:"purchase_exponent"`

	// Authentication outcome
	TransStatus     TransStatus `db:"trans_status"`
	AuthValue       *string     `db:"auth_value"` // CAVV / Authentication Value
	ECI             *string     `db:"eci"`

	// Challenge state
	ChallengeAttempts int    `db:"challenge_attempts"`
	OTPCode           string `db:"-"` // never persisted directly; stored hashed
	OTPHash           string `db:"otp_hash"`
	OTPExpiry         *time.Time `db:"otp_expiry"`

	// Second factor (MFA) state
	SecondFactorRequired bool       `db:"second_factor_required"`
	PINHash              string     `db:"pin_hash"`
	PINExpiry            *time.Time `db:"pin_expiry"`
	PINAttempts          int        `db:"pin_attempts"`

	// Timestamps
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	CompletedAt *time.Time `db:"completed_at"`
}

// NewTransaction creates a new Transaction aggregate with required identifiers.
func NewTransaction(
	specVersion SpecVersion,
	deviceChannel DeviceChannel,
	msgCategory MessageCategory,
	dsTransID, serverTransID uuid.UUID,
) *Transaction {
	now := time.Now().UTC()
	return &Transaction{
		ID:                  uuid.New(),
		ACSTransactionID:    uuid.New(),
		DSTransactionID:     dsTransID,
		ServerTransactionID: serverTransID,
		SpecVersion:         specVersion,
		MessageCategory:     msgCategory,
		DeviceChannel:       deviceChannel,
		TransStatus:         TransStatusUnavailable, // default until evaluated
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

// IsChallengeable returns true if the transaction can be challenged (OTP or OOB/decoupled).
func (t *Transaction) IsChallengeable() bool {
	return t.TransStatus == TransStatusChallenge || t.TransStatus == TransStatusDecoupled
}

// Complete marks the transaction as finished with a final status.
func (t *Transaction) Complete(status TransStatus, authValue, eci string) {
	now := time.Now().UTC()
	t.TransStatus = status
	if authValue != "" {
		t.AuthValue = &authValue
	}
	if eci != "" {
		t.ECI = &eci
	}
	t.CompletedAt = &now
	t.UpdatedAt = now
}
