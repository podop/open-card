package entity_test

import (
	"testing"

	"github.com/your-org/acs-server/internal/domain/entity"
)

func TestCardRange_Contains(t *testing.T) {
	cr := &entity.CardRange{
		StartRange: "400000",
		EndRange:   "499999",
	}

	cases := []struct {
		pan  string
		want bool
	}{
		{"4000001234567890", true},
		{"4999991234567890", true},
		{"5000001234567890", false}, // Mastercard BIN
		{"3782822463100", false},    // Amex
		{"400", false},              // too short
	}

	for _, tc := range cases {
		got := cr.Contains(tc.pan)
		if got != tc.want {
			t.Errorf("Contains(%q) = %v, want %v", tc.pan, got, tc.want)
		}
	}
}

func TestCardRange_SupportsVersion(t *testing.T) {
	cr := &entity.CardRange{
		MinProtocol: entity.SpecVersion220,
		MaxProtocol: entity.SpecVersion231,
	}

	if !cr.SupportsVersion(entity.SpecVersion220) {
		t.Error("should support 2.2.0")
	}
	if !cr.SupportsVersion(entity.SpecVersion231) {
		t.Error("should support 2.3.1")
	}
	if cr.SupportsVersion("2.1.0") {
		t.Error("should not support 2.1.0")
	}
}

func TestTransaction_Complete(t *testing.T) {
	tx := entity.NewTransaction(
		entity.SpecVersion220,
		entity.DeviceChannelBrowser,
		entity.MessageCategoryPayment,
		[16]byte{},
		[16]byte{},
	)

	tx.Complete(entity.TransStatusAuthenticated, "CAVV123", "05")

	if tx.TransStatus != entity.TransStatusAuthenticated {
		t.Errorf("expected Y, got %s", tx.TransStatus)
	}
	if tx.AuthValue == nil || *tx.AuthValue != "CAVV123" {
		t.Error("auth value not set")
	}
	if tx.ECI == nil || *tx.ECI != "05" {
		t.Error("ECI not set")
	}
	if tx.CompletedAt == nil {
		t.Error("completed_at should be set")
	}
}
