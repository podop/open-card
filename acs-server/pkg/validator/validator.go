package validator

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	rePAN     = regexp.MustCompile(`^\d{13,19}$`)
	reUUID    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	reVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
)

// ValidatePAN checks that a string is a valid 13–19 digit PAN (Luhn not enforced here).
func ValidatePAN(pan string) error {
	if !rePAN.MatchString(pan) {
		return fmt.Errorf("PAN must be 13–19 digits")
	}
	return nil
}

// ValidateUUID checks that a string is a valid lowercase UUID v4.
func ValidateUUID(s string) error {
	if !reUUID.MatchString(s) {
		return fmt.Errorf("invalid UUID format: %s", s)
	}
	return nil
}

// ValidateSpecVersion checks that a version string matches N.N.N format.
func ValidateSpecVersion(v string) error {
	if !reVersion.MatchString(v) {
		return fmt.Errorf("invalid spec version format: %s", v)
	}
	return nil
}

// ValidateDeviceChannel checks for a known device channel code.
func ValidateDeviceChannel(ch string) error {
	switch ch {
	case "01", "02", "03":
		return nil
	default:
		return fmt.Errorf("unknown device channel: %s (expected 01, 02, or 03)", ch)
	}
}

// ValidateMessageCategory checks for a known message category code.
func ValidateMessageCategory(mc string) error {
	switch mc {
	case "01", "02":
		return nil
	default:
		return fmt.Errorf("unknown message category: %s (expected 01 or 02)", mc)
	}
}

// ValidateCurrencyCode checks for a 3-digit ISO 4217 numeric code.
func ValidateCurrencyCode(code string) error {
	if len(code) != 3 {
		return fmt.Errorf("currency code must be exactly 3 digits")
	}
	if _, err := strconv.Atoi(code); err != nil {
		return fmt.Errorf("currency code must be numeric: %s", code)
	}
	return nil
}

// ValidateCountryCode checks for a 3-character ISO 3166-1 numeric country code.
func ValidateCountryCode(code string) error {
	if len(code) != 3 {
		return fmt.Errorf("country code must be exactly 3 chars")
	}
	if _, err := strconv.Atoi(code); err != nil {
		return fmt.Errorf("country code must be numeric: %s", code)
	}
	return nil
}
