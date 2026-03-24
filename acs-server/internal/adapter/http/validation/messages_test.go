package validation_test

import (
	"testing"

	"github.com/your-org/acs-server/internal/adapter/http/dto"
	"github.com/your-org/acs-server/internal/adapter/http/validation"
)

// ── helpers ──────────────────────────────────────────────────────

func ptr[T any](v T) *T { return &v }

func hasError(errs []validation.FieldError, field, code string) bool {
	for _, e := range errs {
		if e.Field == field && e.ErrorCode == code {
			return true
		}
	}
	return false
}

// ── AReq ─────────────────────────────────────────────────────────

func validBrowserAReq() *dto.AReqDTO {
	return &dto.AReqDTO{
		MessageType:          "AReq",
		MessageVersion:       "2.2.0",
		DeviceChannel:        "02",
		MessageCategory:      "01",
		ThreeDSServerTransID: "550e8400-e29b-41d4-a716-446655440000",
		DSTransID:            "550e8400-e29b-41d4-a716-446655440001",
		AcctNumber:           "4111111111111111",
		MerchantName:         "Test Merchant",
		MerchantCountry:      "840",
		PurchaseAmount:       ptr("1000"),
		PurchaseCurrency:     ptr("840"),
		PurchaseExponent:     ptr(2),
		BrowserAcceptHeader:  ptr("text/html"),
		BrowserUserAgent:     ptr("Mozilla/5.0"),
		BrowserLanguage:      ptr("en-US"),
		BrowserColorDepth:    ptr("24"),
		BrowserScreenHeight:  ptr("900"),
		BrowserScreenWidth:   ptr("1440"),
		BrowserTZ:            ptr(-300),
		BrowserJavaEnabled:   ptr(false),
	}
}

func TestValidateAReq_Valid(t *testing.T) {
	errs := validation.ValidateAReq(validBrowserAReq())
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateAReq_UnsupportedVersion(t *testing.T) {
	req := validBrowserAReq()
	req.MessageVersion = "1.0.0"
	errs := validation.ValidateAReq(req)
	if !hasError(errs, "messageVersion", "102") {
		t.Error("expected messageVersion error")
	}
}

func TestValidateAReq_InvalidDeviceChannel(t *testing.T) {
	req := validBrowserAReq()
	req.DeviceChannel = "99"
	errs := validation.ValidateAReq(req)
	if !hasError(errs, "deviceChannel", "203") {
		t.Error("expected deviceChannel error")
	}
}

func TestValidateAReq_InvalidPAN(t *testing.T) {
	req := validBrowserAReq()
	req.AcctNumber = "123"
	errs := validation.ValidateAReq(req)
	if !hasError(errs, "acctNumber", "203") {
		t.Error("expected acctNumber error")
	}
}

func TestValidateAReq_MerchantNameTooLong(t *testing.T) {
	req := validBrowserAReq()
	req.MerchantName = "This merchant name is way too long and exceeds the forty character limit"
	errs := validation.ValidateAReq(req)
	if !hasError(errs, "merchantName", "203") {
		t.Error("expected merchantName error")
	}
}

func TestValidateAReq_PA_MissingPurchaseFields(t *testing.T) {
	req := validBrowserAReq()
	req.PurchaseAmount = nil
	req.PurchaseCurrency = nil
	req.PurchaseExponent = nil
	errs := validation.ValidateAReq(req)
	if !hasError(errs, "purchaseAmount", "201") {
		t.Error("expected purchaseAmount missing error")
	}
	if !hasError(errs, "purchaseCurrency", "201") {
		t.Error("expected purchaseCurrency missing error")
	}
	if !hasError(errs, "purchaseExponent", "201") {
		t.Error("expected purchaseExponent missing error")
	}
}

func TestValidateAReq_Browser_MissingAcceptHeader(t *testing.T) {
	req := validBrowserAReq()
	req.BrowserAcceptHeader = nil
	errs := validation.ValidateAReq(req)
	if !hasError(errs, "browserAcceptHeader", "201") {
		t.Error("expected browserAcceptHeader missing error")
	}
}

func TestValidateAReq_Browser_InvalidColorDepth(t *testing.T) {
	req := validBrowserAReq()
	req.BrowserColorDepth = ptr("9")
	errs := validation.ValidateAReq(req)
	if !hasError(errs, "browserColorDepth", "203") {
		t.Error("expected browserColorDepth format error")
	}
}

func TestValidateAReq_Browser_TZOutOfRange(t *testing.T) {
	req := validBrowserAReq()
	req.BrowserTZ = ptr(999)
	errs := validation.ValidateAReq(req)
	if !hasError(errs, "browserTZ", "203") {
		t.Error("expected browserTZ range error")
	}
}

func TestValidateAReq_Browser_MissingJavaEnabled(t *testing.T) {
	req := validBrowserAReq()
	req.BrowserJavaEnabled = nil
	errs := validation.ValidateAReq(req)
	if !hasError(errs, "browserJavaEnabled", "201") {
		t.Error("expected browserJavaEnabled missing error")
	}
}

func TestValidateAReq_App_RequiredSDKFields(t *testing.T) {
	req := &dto.AReqDTO{
		MessageType:          "AReq",
		MessageVersion:       "2.2.0",
		DeviceChannel:        "01",
		MessageCategory:      "01",
		ThreeDSServerTransID: "550e8400-e29b-41d4-a716-446655440000",
		DSTransID:            "550e8400-e29b-41d4-a716-446655440001",
		AcctNumber:           "4111111111111111",
		MerchantName:         "Merchant",
		MerchantCountry:      "840",
		PurchaseAmount:       ptr("1000"),
		PurchaseCurrency:     ptr("840"),
		PurchaseExponent:     ptr(2),
		// SDK fields deliberately omitted
	}
	errs := validation.ValidateAReq(req)
	for _, field := range []string{"sdkAppID", "sdkEncData", "sdkEphemPubKey", "sdkMaxTimeout", "sdkReferenceNumber", "sdkTransID"} {
		if !hasError(errs, field, "201") {
			t.Errorf("expected %s missing error", field)
		}
	}
}

func TestValidateAReq_App_SDKMaxTimeoutOutOfRange(t *testing.T) {
	req := &dto.AReqDTO{
		MessageVersion: "2.2.0", DeviceChannel: "01", MessageCategory: "02",
		AcctNumber: "4111111111111111", MerchantName: "M", MerchantCountry: "840",
		SDKAppID: ptr("550e8400-e29b-41d4-a716-446655440002"),
		SDKEncData: ptr("somejwe"), SDKEphemPubKey: ptr("{}"),
		SDKMaxTimeout: ptr(100), // out of range
		SDKReferenceNumber: ptr("REF001"),
		SDKTransID: ptr("550e8400-e29b-41d4-a716-446655440003"),
	}
	errs := validation.ValidateAReq(req)
	if !hasError(errs, "sdkMaxTimeout", "203") {
		t.Error("expected sdkMaxTimeout range error")
	}
}

func TestValidateAReq_3RI_MissingThreeRIInd(t *testing.T) {
	req := &dto.AReqDTO{
		MessageVersion: "2.2.0", DeviceChannel: "03", MessageCategory: "02",
		AcctNumber: "4111111111111111", MerchantName: "M", MerchantCountry: "840",
	}
	errs := validation.ValidateAReq(req)
	if !hasError(errs, "threeRIInd", "201") {
		t.Error("expected threeRIInd missing error")
	}
}

func TestValidateAReq_NPA_NoPurchaseFieldsRequired(t *testing.T) {
	req := validBrowserAReq()
	req.MessageCategory = "02" // NPA — purchase fields not required
	req.PurchaseAmount = nil
	req.PurchaseCurrency = nil
	req.PurchaseExponent = nil
	errs := validation.ValidateAReq(req)
	if hasError(errs, "purchaseAmount", "201") {
		t.Error("purchaseAmount should NOT be required for NPA")
	}
}

// ── CReq ─────────────────────────────────────────────────────────

func validCReq() *dto.CReqDTO {
	return &dto.CReqDTO{
		MessageType:          "CReq",
		MessageVersion:       "2.2.0",
		ThreeDSServerTransID: "550e8400-e29b-41d4-a716-446655440000",
		ACSTransID:           "550e8400-e29b-41d4-a716-446655440001",
	}
}

func TestValidateCReq_Valid(t *testing.T) {
	errs := validation.ValidateCReq(validCReq())
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateCReq_UnsupportedVersion(t *testing.T) {
	req := validCReq()
	req.MessageVersion = "1.0.0"
	errs := validation.ValidateCReq(req)
	if !hasError(errs, "messageVersion", "102") {
		t.Error("expected messageVersion error")
	}
}

func TestValidateCReq_InvalidWindowSize(t *testing.T) {
	req := validCReq()
	req.ChallengeWindowSize = "99"
	errs := validation.ValidateCReq(req)
	if !hasError(errs, "challengeWindowSize", "203") {
		t.Error("expected challengeWindowSize error")
	}
}

func TestValidateCReq_ValidWindowSize(t *testing.T) {
	for _, v := range []string{"01", "02", "03", "04", "05"} {
		req := validCReq()
		req.ChallengeWindowSize = v
		errs := validation.ValidateCReq(req)
		if len(errs) != 0 {
			t.Errorf("challengeWindowSize %q should be valid, got: %v", v, errs)
		}
	}
}

func TestValidateCReq_App_MissingSDKCounter(t *testing.T) {
	req := validCReq()
	req.SDKTransID = ptr("550e8400-e29b-41d4-a716-446655440002")
	// sdkCounterStoA deliberately omitted
	errs := validation.ValidateCReq(req)
	if !hasError(errs, "sdkCounterStoA", "201") {
		t.Error("expected sdkCounterStoA missing error when sdkTransID present")
	}
}

func TestValidateCReq_App_InvalidSDKCounter(t *testing.T) {
	req := validCReq()
	req.SDKTransID = ptr("550e8400-e29b-41d4-a716-446655440002")
	req.SDKCounterStoA = ptr("abc") // must be 6 digits
	errs := validation.ValidateCReq(req)
	if !hasError(errs, "sdkCounterStoA", "203") {
		t.Error("expected sdkCounterStoA format error")
	}
}

func TestValidateCReq_App_Valid(t *testing.T) {
	req := validCReq()
	req.SDKTransID = ptr("550e8400-e29b-41d4-a716-446655440002")
	req.SDKCounterStoA = ptr("000001")
	errs := validation.ValidateCReq(req)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid app CReq, got: %v", errs)
	}
}

// ── RRes ─────────────────────────────────────────────────────────

func validRRes() *dto.RResDTO {
	return &dto.RResDTO{
		MessageType:          "RRes",
		MessageVersion:       "2.2.0",
		ThreeDSServerTransID: "550e8400-e29b-41d4-a716-446655440000",
		ACSTransID:           "550e8400-e29b-41d4-a716-446655440001",
		DSTransID:            "550e8400-e29b-41d4-a716-446655440002",
		ResultsStatus:        "01",
	}
}

func TestValidateRRes_Valid(t *testing.T) {
	errs := validation.ValidateRRes(validRRes())
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateRRes_UnsupportedVersion(t *testing.T) {
	res := validRRes()
	res.MessageVersion = "3.0.0"
	errs := validation.ValidateRRes(res)
	if !hasError(errs, "messageVersion", "102") {
		t.Error("expected messageVersion error")
	}
}

func TestValidateRRes_InvalidResultsStatus(t *testing.T) {
	res := validRRes()
	res.ResultsStatus = "02"
	errs := validation.ValidateRRes(res)
	if !hasError(errs, "resultsStatus", "203") {
		t.Error("expected resultsStatus error")
	}
}