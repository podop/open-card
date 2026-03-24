package entity

import "time"

// Network identifies the card scheme / payment network (stored as a slug string).
type Network string

// Well-known network slugs (kept as convenience constants for tests and seed data).
// New networks can be created via the admin API without code changes.
const (
	NetworkVisa       Network = "visa"
	NetworkMastercard Network = "mastercard"
	NetworkAmex       Network = "amex"
	NetworkDiscover   Network = "discover"
	NetworkUnknown    Network = "unknown"
)

// ECIValues holds the ECI codes for different authentication outcomes on a given network.
type ECIValues struct {
	FullyAuthenticated string
	AttemptProcessing  string
	NotAuthenticated   string
}

// DefaultECIs returns fallback ECI codes when no DB record is available.
func (n Network) DefaultECIs() ECIValues {
	if n == NetworkMastercard {
		return ECIValues{FullyAuthenticated: "02", AttemptProcessing: "01", NotAuthenticated: "00"}
	}
	return ECIValues{FullyAuthenticated: "05", AttemptProcessing: "06", NotAuthenticated: "07"}
}

// NetworkEntry represents a row in the networks table.
type NetworkEntry struct {
	ID          int64     `db:"id"             json:"id"`
	Name        string    `db:"name"           json:"name"`
	DisplayName string    `db:"display_name"   json:"displayName"`
	Active      bool      `db:"active"         json:"active"`
	Color       string    `db:"color"          json:"color"`
	ECIFullAuth string    `db:"eci_full_auth"  json:"eciFullAuth"`
	ECIAttempt  string    `db:"eci_attempt"    json:"eciAttempt"`
	ECINotAuth  string    `db:"eci_not_auth"   json:"eciNotAuth"`
	CreatedAt   time.Time `db:"created_at"     json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at"     json:"updatedAt"`
}

// ECIs returns the ECI values from this DB entry.
func (e *NetworkEntry) ECIs() ECIValues {
	return ECIValues{
		FullyAuthenticated: e.ECIFullAuth,
		AttemptProcessing:  e.ECIAttempt,
		NotAuthenticated:   e.ECINotAuth,
	}
}