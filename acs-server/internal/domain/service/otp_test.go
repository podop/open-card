package service_test

import (
	"testing"
	"time"

	"github.com/your-org/acs-server/internal/domain/service"
)

func TestOTPService_GenerateAndVerify(t *testing.T) {
	svc := service.NewOTPService()

	otp, hash, expiry, err := svc.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if len(otp) != 6 {
		t.Errorf("expected 6-digit OTP, got %d chars", len(otp))
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if expiry.Before(time.Now()) {
		t.Error("expiry should be in the future")
	}

	// Correct OTP should verify
	if !svc.Verify(otp, hash, expiry) {
		t.Error("Verify() should return true for correct OTP")
	}

	// Wrong OTP should fail
	if svc.Verify("000000", hash, expiry) {
		t.Error("Verify() should return false for wrong OTP")
	}

	// Expired OTP should fail
	pastExpiry := time.Now().Add(-1 * time.Minute)
	if svc.Verify(otp, hash, pastExpiry) {
		t.Error("Verify() should return false for expired OTP")
	}
}

func TestOTPService_MaxAttemptsExceeded(t *testing.T) {
	svc := service.NewOTPService()
	if svc.MaxAttemptsExceeded(2) {
		t.Error("2 attempts should not exceed limit")
	}
	if !svc.MaxAttemptsExceeded(3) {
		t.Error("3 attempts should exceed limit")
	}
}
