// Package validation implements EMV 3DS message-level validation rules for AReq, CReq,
// and RRes messages as specified in EMVCo 3-D Secure Protocol and Core Functions
// Specification v2.2.0 / v2.3.1.
//
// Error codes follow the EMVCo specification (Table A.4):
//
//	102 – Message Version Number Not Supported
//	201 – Required Data Element Missing
//	203 – Data element not in the allowed format or value set
package validation

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/your-org/acs-server/internal/adapter/http/dto"
	"github.com/your-org/acs-server/pkg/validator"
)

// ────────────────────────────────────────────────────────────────
// Error types
// ────────────────────────────────────────────────────────────────

// FieldError represents a single EMV 3DS field validation failure.
type FieldError struct {
	Field       string // field name as it appears in the EMV 3DS spec JSON
	ErrorCode   string // EMVCo error code
	Description string
}

func (e FieldError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.ErrorCode, e.Field, e.Description)
}

// ────────────────────────────────────────────────────────────────
// Shared helpers
// ────────────────────────────────────────────────────────────────

var (
	reNumericMax6  = regexp.MustCompile(`^\d{1,6}$`)
	reNumericMax48 = regexp.MustCompile(`^\d{1,48}$`)
	reNumeric6     = regexp.MustCompile(`^\d{6}$`)
	reNumeric2     = regexp.MustCompile(`^\d{2}$`)
)

var supportedVersions = map[string]bool{
	"2.2.0": true,
	"2.3.1": true,
}

var validColorDepths = map[string]bool{
	"1": true, "4": true, "8": true, "15": true,
	"16": true, "24": true, "32": true, "48": true,
}

var validChallengeWindowSizes = map[string]bool{
	"01": true, "02": true, "03": true, "04": true, "05": true,
}

func missing(field string) FieldError {
	return FieldError{Field: field, ErrorCode: "201", Description: "required data element missing"}
}

func format(field, detail string) FieldError {
	return FieldError{Field: field, ErrorCode: "203", Description: detail}
}

func versionErr(field, detail string) FieldError {
	return FieldError{Field: field, ErrorCode: "102", Description: detail}
}

// ────────────────────────────────────────────────────────────────
// AReq
// ────────────────────────────────────────────────────────────────

// ValidateAReq validates an AReq DTO against EMVCo field rules.
// Binding tags on the struct already enforce always-required UUID / literal fields;
// this function handles conditional groups and value-set / length constraints.
func ValidateAReq(req *dto.AReqDTO) []FieldError {
	var errs []FieldError

	// messageVersion
	if !supportedVersions[req.MessageVersion] {
		errs = append(errs, versionErr("messageVersion",
			fmt.Sprintf("unsupported version %q; accepted: 2.2.0, 2.3.1", req.MessageVersion)))
	}

	// deviceChannel / messageCategory — enum check
	if err := validator.ValidateDeviceChannel(req.DeviceChannel); err != nil {
		errs = append(errs, format("deviceChannel", err.Error()))
	}
	if err := validator.ValidateMessageCategory(req.MessageCategory); err != nil {
		errs = append(errs, format("messageCategory", err.Error()))
	}

	// acctNumber — 13–19 digit PAN or token
	if err := validator.ValidatePAN(req.AcctNumber); err != nil {
		errs = append(errs, format("acctNumber", err.Error()))
	}

	// merchantName — AN max 40
	if len(req.MerchantName) > 40 {
		errs = append(errs, format("merchantName", "max 40 characters"))
	}

	// merchantCountry — N-3 (ISO 3166-1 numeric)
	if err := validator.ValidateCountryCode(req.MerchantCountry); err != nil {
		errs = append(errs, format("merchantCountry", err.Error()))
	}

	// ── Conditional: Payment Authentication (messageCategory=01) ──────────
	if req.MessageCategory == "01" {
		if req.PurchaseAmount == nil {
			errs = append(errs, missing("purchaseAmount"))
		} else if !reNumericMax48.MatchString(*req.PurchaseAmount) {
			errs = append(errs, format("purchaseAmount", "must be numeric, max 48 digits"))
		}

		if req.PurchaseCurrency == nil {
			errs = append(errs, missing("purchaseCurrency"))
		} else if err := validator.ValidateCurrencyCode(*req.PurchaseCurrency); err != nil {
			errs = append(errs, format("purchaseCurrency", err.Error()))
		}

		if req.PurchaseExponent == nil {
			errs = append(errs, missing("purchaseExponent"))
		} else if *req.PurchaseExponent < 0 || *req.PurchaseExponent > 9 {
			errs = append(errs, format("purchaseExponent", "must be 0–9"))
		}
	}

	// ── Conditional: Browser channel (deviceChannel=02) ───────────────────
	if req.DeviceChannel == "02" {
		// browserAcceptHeader — required, AN max 2048
		if req.BrowserAcceptHeader == nil || *req.BrowserAcceptHeader == "" {
			errs = append(errs, missing("browserAcceptHeader"))
		} else if len(*req.BrowserAcceptHeader) > 2048 {
			errs = append(errs, format("browserAcceptHeader", "max 2048 characters"))
		}

		// browserUserAgent — required, AN max 2048
		if req.BrowserUserAgent == nil || *req.BrowserUserAgent == "" {
			errs = append(errs, missing("browserUserAgent"))
		} else if len(*req.BrowserUserAgent) > 2048 {
			errs = append(errs, format("browserUserAgent", "max 2048 characters"))
		}

		// browserLanguage — required, AN max 8
		if req.BrowserLanguage == nil || *req.BrowserLanguage == "" {
			errs = append(errs, missing("browserLanguage"))
		} else if len(*req.BrowserLanguage) > 8 {
			errs = append(errs, format("browserLanguage", "max 8 characters"))
		}

		// browserColorDepth — required, enum
		if req.BrowserColorDepth == nil {
			errs = append(errs, missing("browserColorDepth"))
		} else if !validColorDepths[*req.BrowserColorDepth] {
			errs = append(errs, format("browserColorDepth",
				"must be one of: 1, 4, 8, 15, 16, 24, 32, 48"))
		}

		// browserScreenHeight — required, N max 6
		if req.BrowserScreenHeight == nil {
			errs = append(errs, missing("browserScreenHeight"))
		} else if !reNumericMax6.MatchString(*req.BrowserScreenHeight) {
			errs = append(errs, format("browserScreenHeight", "must be numeric, max 6 digits"))
		}

		// browserScreenWidth — required, N max 6
		if req.BrowserScreenWidth == nil {
			errs = append(errs, missing("browserScreenWidth"))
		} else if !reNumericMax6.MatchString(*req.BrowserScreenWidth) {
			errs = append(errs, format("browserScreenWidth", "must be numeric, max 6 digits"))
		}

		// browserTZ — required, integer -840 to 720 (UTC offset in minutes)
		if req.BrowserTZ == nil {
			errs = append(errs, missing("browserTZ"))
		} else if *req.BrowserTZ < -840 || *req.BrowserTZ > 720 {
			errs = append(errs, format("browserTZ", "must be integer in range -840 to 720"))
		}

		// browserJavaEnabled — required boolean
		if req.BrowserJavaEnabled == nil {
			errs = append(errs, missing("browserJavaEnabled"))
		}
	}

	// ── Conditional: App channel (deviceChannel=01) ───────────────────────
	if req.DeviceChannel == "01" {
		// sdkAppID — required, UUID
		if req.SDKAppID == nil || *req.SDKAppID == "" {
			errs = append(errs, missing("sdkAppID"))
		} else if err := validator.ValidateUUID(*req.SDKAppID); err != nil {
			errs = append(errs, format("sdkAppID", err.Error()))
		}

		// sdkEncData — required (JWE compact serialisation)
		if req.SDKEncData == nil || *req.SDKEncData == "" {
			errs = append(errs, missing("sdkEncData"))
		}

		// sdkEphemPubKey — required (JWK JSON object)
		if req.SDKEphemPubKey == nil || *req.SDKEphemPubKey == "" {
			errs = append(errs, missing("sdkEphemPubKey"))
		}

		// sdkMaxTimeout — required, N-2, range 05–99
		if req.SDKMaxTimeout == nil {
			errs = append(errs, missing("sdkMaxTimeout"))
		} else if *req.SDKMaxTimeout < 5 || *req.SDKMaxTimeout > 99 {
			errs = append(errs, format("sdkMaxTimeout", "must be in range 05–99"))
		}

		// sdkReferenceNumber — required, AN max 32
		if req.SDKReferenceNumber == nil || *req.SDKReferenceNumber == "" {
			errs = append(errs, missing("sdkReferenceNumber"))
		} else if len(*req.SDKReferenceNumber) > 32 {
			errs = append(errs, format("sdkReferenceNumber", "max 32 characters"))
		}

		// sdkTransID — required, UUID
		if req.SDKTransID == nil {
			errs = append(errs, missing("sdkTransID"))
		} else if err := validator.ValidateUUID(*req.SDKTransID); err != nil {
			errs = append(errs, format("sdkTransID", err.Error()))
		}
	}

	// ── Conditional: 3RI channel (deviceChannel=03) ───────────────────────
	if req.DeviceChannel == "03" {
		// threeRIInd — required, N-2 (e.g. "01"–"11")
		if req.ThreeRIInd == nil || *req.ThreeRIInd == "" {
			errs = append(errs, missing("threeRIInd"))
		} else if !reNumeric2.MatchString(*req.ThreeRIInd) {
			errs = append(errs, format("threeRIInd", "must be N-2 (e.g. \"01\"–\"11\")"))
		} else {
			ind, _ := strconv.Atoi(*req.ThreeRIInd)
			if ind < 1 || ind > 15 {
				errs = append(errs, format("threeRIInd", "value out of range; valid values 01–15"))
			}
		}
	}

	return errs
}

// ────────────────────────────────────────────────────────────────
// CReq
// ────────────────────────────────────────────────────────────────

// ValidateCReq validates a CReq DTO against EMVCo field rules.
func ValidateCReq(req *dto.CReqDTO) []FieldError {
	var errs []FieldError

	// messageVersion
	if !supportedVersions[req.MessageVersion] {
		errs = append(errs, versionErr("messageVersion",
			fmt.Sprintf("unsupported version %q; accepted: 2.2.0, 2.3.1", req.MessageVersion)))
	}

	// challengeWindowSize — optional; if present must be "01"–"05"
	if req.ChallengeWindowSize != "" && !validChallengeWindowSizes[req.ChallengeWindowSize] {
		errs = append(errs, format("challengeWindowSize",
			"must be one of: 01 (250x400), 02 (390x400), 03 (500x600), 04 (600x400), 05 (full-screen)"))
	}

	// ── Conditional: app-based challenge (sdkTransID present implies app channel) ──
	if req.SDKTransID != nil {
		if err := validator.ValidateUUID(*req.SDKTransID); err != nil {
			errs = append(errs, format("sdkTransID", err.Error()))
		}

		// sdkCounterStoA — required when sdkTransID is present; exactly N-6 digits
		if req.SDKCounterStoA == nil || *req.SDKCounterStoA == "" {
			errs = append(errs, missing("sdkCounterStoA"))
		} else if !reNumeric6.MatchString(*req.SDKCounterStoA) {
			errs = append(errs, format("sdkCounterStoA", "must be exactly 6 digits"))
		}
	}

	return errs
}

// ────────────────────────────────────────────────────────────────
// RRes
// ────────────────────────────────────────────────────────────────

// ValidateRRes validates an RRes DTO against EMVCo field rules.
// Called when the ACS processes the DS's reply to an outbound RReq.
func ValidateRRes(res *dto.RResDTO) []FieldError {
	var errs []FieldError

	// messageVersion
	if !supportedVersions[res.MessageVersion] {
		errs = append(errs, versionErr("messageVersion",
			fmt.Sprintf("unsupported version %q; accepted: 2.2.0, 2.3.1", res.MessageVersion)))
	}

	// UUID fields validated by binding tags (threeDSServerTransID, acsTransID, dsTransID)

	// resultsStatus — only "01" (results acknowledged) is valid per spec
	if res.ResultsStatus != "01" {
		errs = append(errs, format("resultsStatus",
			fmt.Sprintf("must be \"01\" (results acknowledged), got %q", res.ResultsStatus)))
	}

	return errs
}