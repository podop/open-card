package validator_test

import (
	"testing"

	"github.com/your-org/acs-server/pkg/validator"
)

func TestValidatePAN(t *testing.T) {
	cases := []struct{ pan string; ok bool }{
		{"4111111111111111", true},
		{"378282246310005", true},
		{"41111111111", false},        // too short
		{"41111111111111111111", false}, // too long
		{"4111111111111abc", false},   // non-digit
	}
	for _, tc := range cases {
		err := validator.ValidatePAN(tc.pan)
		if (err == nil) != tc.ok {
			t.Errorf("ValidatePAN(%q) err=%v, want ok=%v", tc.pan, err, tc.ok)
		}
	}
}

func TestValidateDeviceChannel(t *testing.T) {
	if err := validator.ValidateDeviceChannel("01"); err != nil {
		t.Errorf("unexpected error for 01: %v", err)
	}
	if err := validator.ValidateDeviceChannel("99"); err == nil {
		t.Error("expected error for unknown channel 99")
	}
}

func TestValidateSpecVersion(t *testing.T) {
	if err := validator.ValidateSpecVersion("2.2.0"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validator.ValidateSpecVersion("invalid"); err == nil {
		t.Error("expected error for invalid version format")
	}
}
