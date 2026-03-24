package entity

import "time"

// NetworkRule holds per-network default flow-control settings.
type NetworkRule struct {
	ID                      int64     `db:"id"                        json:"id"`
	Network                 Network   `db:"network"                   json:"network"`
	AllowFrictionless       bool      `db:"allow_frictionless"        json:"allowFrictionless"`
	AllowOTP                bool      `db:"allow_otp"                 json:"allowOtp"`
	AllowOOB                bool      `db:"allow_oob"                 json:"allowOob"`
	DefaultChallengeChannel string    `db:"default_challenge_channel" json:"defaultChallengeChannel"`
	SecondFactorMode        string    `db:"second_factor_mode"        json:"secondFactorMode"`  // "none" | "otp" | "oob" | "both"
	MFAProviderID           *int64    `db:"mfa_provider_id"           json:"mfaProviderId"`
	CreatedAt               time.Time `db:"created_at"                json:"createdAt"`
	UpdatedAt               time.Time `db:"updated_at"                json:"updatedAt"`
}

// BinExclusion overrides network-level rules for a specific BIN range.
type BinExclusion struct {
	ID                      int64     `db:"id"                        json:"id"`
	Network                 Network   `db:"network"                   json:"network"`
	StartRange              string    `db:"start_range"               json:"startRange"`
	EndRange                string    `db:"end_range"                 json:"endRange"`
	Description             string    `db:"description"               json:"description"`
	AllowFrictionless       bool      `db:"allow_frictionless"        json:"allowFrictionless"`
	AllowOTP                bool      `db:"allow_otp"                 json:"allowOtp"`
	AllowOOB                bool      `db:"allow_oob"                 json:"allowOob"`
	DefaultChallengeChannel string    `db:"default_challenge_channel" json:"defaultChallengeChannel"`
	SecondFactorMode        string    `db:"second_factor_mode"        json:"secondFactorMode"` // "none" | "otp" | "oob" | "both"
	MFAProviderID           *int64    `db:"mfa_provider_id"           json:"mfaProviderId"`
	Active                  bool      `db:"active"                    json:"active"`
	CreatedAt               time.Time `db:"created_at"                json:"createdAt"`
	UpdatedAt               time.Time `db:"updated_at"                json:"updatedAt"`
}

// FlowSettings is a resolved set of flow-control flags for a given transaction.
type FlowSettings struct {
	AllowFrictionless       bool
	AllowOTP                bool
	AllowOOB                bool
	DefaultChallengeChannel string // "otp" | "oob"
	SecondFactorMode        string // "none" | "otp" | "oob" | "both"
	MFAProviderID           *int64
	Source                  string // "network_rule" | "bin_exclusion"
}