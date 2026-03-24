package errors

import "fmt"

// NotFoundError is returned when a requested resource does not exist.
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// ValidationError is returned when input fails domain validation.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// UnsupportedVersionError is returned for unknown spec versions.
type UnsupportedVersionError struct {
	Version string
}

func (e *UnsupportedVersionError) Error() string {
	return fmt.Sprintf("unsupported 3DS protocol version: %s", e.Version)
}

// CardNotEnrolledError is returned when a PAN has no matching card range.
type CardNotEnrolledError struct {
	BIN string
}

func (e *CardNotEnrolledError) Error() string {
	return fmt.Sprintf("card BIN %s is not enrolled for 3DS authentication", e.BIN)
}

// ChallengeExpiredError is returned when an OTP or session has expired.
type ChallengeExpiredError struct{}

func (e *ChallengeExpiredError) Error() string {
	return "challenge expired"
}

// MaxAttemptsError is returned when a cardholder exhausts OTP attempts.
type MaxAttemptsError struct{}

func (e *MaxAttemptsError) Error() string {
	return "maximum challenge attempts exceeded"
}
