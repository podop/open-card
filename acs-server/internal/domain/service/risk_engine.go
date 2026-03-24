package service

import (
	"context"
	"strconv"

	"github.com/your-org/acs-server/internal/domain/entity"
	"go.uber.org/zap"
)

// RiskScore represents the output of risk assessment.
type RiskScore struct {
	Score      int    // 0–100; lower = lower risk
	Frictionless bool // true = skip challenge
	Reason     string
}

// RiskEngine evaluates transaction risk and recommends a TransStatus.
// Implement this interface to plug in ML models, velocity checks, or 3rd-party services.
type RiskEngine interface {
	Evaluate(ctx context.Context, tx *entity.Transaction, browserInfo *BrowserInfo, sdkInfo *SDKDeviceInfo) (*RiskScore, error)
}

// BrowserInfo carries device/browser data collected by the 3DS Method (DeviceChannel=02).
type BrowserInfo struct {
	IPAddress         string
	UserAgent         string
	Language          string
	ColorDepth        int
	ScreenHeight      int
	ScreenWidth       int
	TimeZoneOffset    int
	JavaEnabled       bool
	JavaScriptEnabled bool
}

// SDKDeviceInfo carries device data decrypted from the sdkEncData JWE (DeviceChannel=01).
type SDKDeviceInfo struct {
	// SDKType: "01"=default, "02"=split-SDK.
	SDKType string
	// DeviceBindingStatus: "01"=bound, "02"=not bound, "03"=not available, "04"=not eligible.
	DeviceBindingStatus string
	// SDKVersion is the SDK version reported by the app.
	SDKVersion string
	// AppVersion is the 3DS Requestor app version.
	AppVersion string
	// SDKReferenceNumber identifies the SDK vendor/version.
	SDKReferenceNumber string
	// ClientDeviceInfo is a base64url-encoded device fingerprint blob from the SDK.
	ClientDeviceInfo string
}

// DefaultRiskEngine is a rule-based engine suitable for development and testing.
// Replace with your production risk logic or vendor integration.
type DefaultRiskEngine struct {
	log *zap.Logger
}

// NewDefaultRiskEngine creates a new DefaultRiskEngine.
func NewDefaultRiskEngine(log *zap.Logger) *DefaultRiskEngine {
	return &DefaultRiskEngine{log: log}
}

// Evaluate applies simple heuristic rules and returns a RiskScore.
func (e *DefaultRiskEngine) Evaluate(_ context.Context, tx *entity.Transaction, info *BrowserInfo, sdkInfo *SDKDeviceInfo) (*RiskScore, error) {
	score := &RiskScore{}

	// Rule 1: low-value transactions are frictionless.
	if tx.PurchaseAmount != nil {
		amt, _ := strconv.ParseInt(*tx.PurchaseAmount, 10, 64)
		if amt > 0 && amt <= 10000 { // ≤ 100.00 in minor units (exponent=2)
			score.Score = 10
			score.Frictionless = true
			score.Reason = "low_value"
			return score, nil
		}
	}

	// Rule 2: no device info collected → challenge required.
	// Browser channel requires browserInfo+IP; app channel requires sdkInfo.
	hasDeviceInfo := (info != nil && info.IPAddress != "") || sdkInfo != nil
	if !hasDeviceInfo {
		score.Score = 70
		score.Frictionless = false
		score.Reason = "no_device_info"
		return score, nil
	}

	// Rule 3: app channel — unbound device adds friction.
	if sdkInfo != nil && sdkInfo.DeviceBindingStatus == "02" { // "02" = not bound
		score.Score = 60
		score.Frictionless = false
		score.Reason = "sdk_device_not_bound"
		return score, nil
	}

	// Rule 4: default frictionless for known-good devices (demo).
	score.Score = 20
	score.Frictionless = true
	score.Reason = "default_allow"
	return score, nil
}

// AuthValueService computes the Authentication Value (CAVV) after a successful auth.
type AuthValueService interface {
	Generate(ctx context.Context, tx *entity.Transaction) (authValue, eci string, err error)
}
