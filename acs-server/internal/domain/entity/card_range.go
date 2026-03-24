package entity

import "time"

// CardRange defines a BIN range for which this ACS provides authentication.
// Populated from the Directory Server during enrollment.
type CardRange struct {
	ID              int64       `db:"id"`
	StartRange      string      `db:"start_range"`     // e.g. "4000000000000000"
	EndRange        string      `db:"end_range"`       // e.g. "4999999999999999"
	Network         Network     `db:"network"`         // card scheme: visa | mastercard | amex | discover | unknown
	ACSURLMethod    string      `db:"acs_url_method"`  // 3DS Method URL
	ACSURLChallenge string      `db:"acs_url_challenge"`
	MinProtocol     SpecVersion `db:"min_protocol"`
	MaxProtocol     SpecVersion `db:"max_protocol"`
	ActionInd       string      `db:"action_ind"` // "A"=Add, "M"=Modify, "D"=Delete
	Active          bool        `db:"active"`
	CreatedAt       time.Time   `db:"created_at"`
	UpdatedAt       time.Time   `db:"updated_at"`
}

// Contains returns true if the given PAN falls within this card range.
func (cr *CardRange) Contains(pan string) bool {
	if len(pan) < len(cr.StartRange) {
		return false
	}
	prefix := pan[:len(cr.StartRange)]
	return prefix >= cr.StartRange && prefix <= cr.EndRange
}

// SupportsVersion returns true if the given spec version is within range.
func (cr *CardRange) SupportsVersion(v SpecVersion) bool {
	return v >= cr.MinProtocol && v <= cr.MaxProtocol
}
