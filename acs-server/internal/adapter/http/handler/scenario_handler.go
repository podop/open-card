package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/acs-server/internal/adapter/repository/postgres"
	"github.com/your-org/acs-server/internal/domain/service"
	"go.uber.org/zap"
)

// ScenarioHandler serves the test scenario CRUD and runner API.
type ScenarioHandler struct {
	repo        *postgres.ScenarioRepo
	acsBaseURL  string // internal ACS base URL for loopback calls
	otpResolver *service.SimulatedOTPResolver
	oobResolver *service.SimulatedOOBResolver
	log         *zap.Logger
}

// scenarioExec holds per-execution context for the scenario runner.
type scenarioExec struct {
	h   *ScenarioHandler
	ctx context.Context
}

// NewScenarioHandler creates a new ScenarioHandler.
func NewScenarioHandler(
	repo *postgres.ScenarioRepo,
	acsBaseURL string,
	otpResolver *service.SimulatedOTPResolver,
	oobResolver *service.SimulatedOOBResolver,
	log *zap.Logger,
) *ScenarioHandler {
	return &ScenarioHandler{repo: repo, acsBaseURL: acsBaseURL, otpResolver: otpResolver, oobResolver: oobResolver, log: log}
}

// ── Scenario CRUD ─────────────────────────────────────────────────────────

func (h *ScenarioHandler) ListScenarios(c *gin.Context) {
	rows, err := h.repo.ListScenarios(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *ScenarioHandler) CreateScenario(c *gin.Context) {
	var s postgres.TestScenario
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.CreateScenario(c.Request.Context(), &s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, s)
}

func (h *ScenarioHandler) UpdateScenario(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var s postgres.TestScenario
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.ID = id
	if err := h.repo.UpdateScenario(c.Request.Context(), &s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *ScenarioHandler) DeleteScenario(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.repo.DeleteScenario(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *ScenarioHandler) GetScenario(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	s, err := h.repo.GetScenario(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if s == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scenario not found"})
		return
	}
	c.JSON(http.StatusOK, s)
}

// GetOTP returns the current OTP for a transaction (test-only helper).
func (h *ScenarioHandler) GetOTP(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}
	otp, err := h.repo.GetOTPForTransaction(c.Request.Context(), acsTransID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "OTP not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"otp": otp})
}

// GetPIN returns the current PIN for a transaction (test-only helper for second factor).
func (h *ScenarioHandler) GetPIN(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}
	pin, err := h.repo.GetPINForTransaction(c.Request.Context(), acsTransID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PIN not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pin": pin})
}

// ── Scenario Runner ───────────────────────────────────────────────────────

type stepResult struct {
	Step       string      `json:"step"`
	Status     string      `json:"status"` // ok, fail
	DurationMs int64       `json:"durationMs"`
	Request    interface{} `json:"request,omitempty"`
	Response   interface{} `json:"response,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// RunScenario executes a test scenario end-to-end against the ACS.
// Query param ?mode=interactive stops after AReq and returns challenge URL + OTP for manual testing.
func (h *ScenarioHandler) RunScenario(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	scenario, err := h.repo.GetScenario(c.Request.Context(), id)
	if err != nil || scenario == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scenario not found"})
		return
	}

	mode := c.DefaultQuery("mode", "automatic")

	start := time.Now()
	run := &postgres.ScenarioRun{
		ScenarioID:   scenario.ID,
		ScenarioName: scenario.Name,
		FlowType:     scenario.FlowType,
		Status:       "running",
		Steps:        json.RawMessage("[]"),
		StartedAt:    start,
	}
	if err := h.repo.CreateRun(c.Request.Context(), run); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	exec := &scenarioExec{h: h, ctx: c.Request.Context()}

	// OReq is system-level — no cardholder data, no challenge; always automatic.
	if scenario.FlowType == "oreq" {
		mode = "automatic"
	}

	var steps []stepResult
	if scenario.FlowType == "oreq" {
		steps = exec.executeOReq(scenario)
	} else if mode == "interactive" {
		steps = exec.executeInteractive(scenario)
	} else {
		steps = exec.executeScenario(scenario)
	}

	elapsed := time.Since(start)
	stepsJSON, _ := json.Marshal(steps)
	run.Steps = stepsJSON
	run.DurationMs = int(elapsed.Milliseconds())

	// Determine final status
	if mode == "interactive" {
		run.Status = "interactive"
	} else {
		run.Status = "completed"
		for _, s := range steps {
			if s.Status == "fail" {
				run.Status = "failed"
				run.ErrorMsg = s.Error
				break
			}
		}
	}

	// Extract ACS trans ID from step 1 response
	if len(steps) > 0 {
		if resp, ok := steps[0].Response.(map[string]interface{}); ok {
			if atsStr, ok := resp["acsTransID"].(string); ok {
				if uid, err := uuid.Parse(atsStr); err == nil {
					run.ACSTransactionID = &uid
				}
			}
		}
	}

	now := time.Now()
	if mode != "interactive" {
		run.CompletedAt = &now
	}
	_ = h.repo.UpdateRun(c.Request.Context(), run)

	c.JSON(http.StatusOK, run)
}

// CompleteInteractiveRun marks an interactive run as completed after the tester finishes.
func (h *ScenarioHandler) CompleteInteractiveRun(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	run, err := h.repo.GetRun(c.Request.Context(), id)
	if err != nil || run == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	if run.Status != "interactive" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run is not in interactive state"})
		return
	}

	// Re-read the transaction to get final status
	finalStatus := "completed"
	if run.ACSTransactionID != nil {
		tx, err := h.repo.GetTransactionStatus(c.Request.Context(), *run.ACSTransactionID)
		if err == nil && tx != "" {
			// If transaction ended in N or U, mark as failed
			if tx == "N" || tx == "U" {
				finalStatus = "completed" // still a valid test outcome
			}
		}
	}

	now := time.Now()
	run.Status = finalStatus
	run.CompletedAt = &now
	run.DurationMs = int(now.Sub(run.StartedAt).Milliseconds())
	_ = h.repo.UpdateRun(c.Request.Context(), run)
	c.JSON(http.StatusOK, run)
}

// executeInteractive runs only AReq + fetch OTP, returning challenge URL for manual testing.
func (e *scenarioExec) executeInteractive(scenario *postgres.TestScenario) []stepResult {
	var steps []stepResult

	// For challenge_unavailable, toggle resolvers before AReq
	if scenario.FlowType == "challenge_unavailable" {
		steps = append(steps, e.stepToggleResolvers(false, false))
	}

	// Step 1: Send AReq
	areqStep, aresData := e.stepAReq(scenario)
	steps = append(steps, areqStep)
	if areqStep.Status == "fail" {
		return steps
	}

	acsTransID, _ := aresData["acsTransID"].(string)
	transStatus, _ := aresData["transStatus"].(string)

	// Frictionless — nothing to interact with
	if transStatus == "Y" {
		steps = append(steps, stepResult{Step: "verify_frictionless", Status: "ok",
			Response: map[string]interface{}{"transStatus": "Y", "note": "Frictionless - no challenge page to show"}})
		return steps
	}

	// Challenge unavailable — return fallback URL for manual inspection
	if scenario.FlowType == "challenge_unavailable" {
		if transStatus != "U" {
			steps = append(steps, stepResult{Step: "verify_unavailable", Status: "fail",
				Error: fmt.Sprintf("expected transStatus=U, got %s", transStatus)})
			steps = append(steps, e.stepToggleResolvers(true, true))
			return steps
		}
		steps = append(steps, stepResult{Step: "verify_unavailable", Status: "ok"})
		challengeURL, _ := aresData["acsURL"].(string)
		steps = append(steps, stepResult{
			Step: "interactive_ready", Status: "ok",
			Response: map[string]interface{}{
				"challengeURL": challengeURL,
				"acsTransID":   acsTransID,
				"flowType":     scenario.FlowType,
				"note":         "Open the fallback page to see the unavailable message",
			},
		})
		steps = append(steps, e.stepToggleResolvers(true, true))
		return steps
	}

	isOOBInteractive := scenario.FlowType == "challenge_oob" || scenario.FlowType == "decoupled"
	expectedInteractive := "C"
	if isOOBInteractive {
		expectedInteractive = "D"
	}
	if transStatus != expectedInteractive {
		steps = append(steps, stepResult{Step: "verify_challenge_required", Status: "fail",
			Error: fmt.Sprintf("expected transStatus=%s, got %s", expectedInteractive, transStatus)})
		return steps
	}
	steps = append(steps, stepResult{Step: "verify_challenge_required", Status: "ok"})

	// Step 2: Fetch OTP (skip for OOB — no OTP generated)
	otp := ""
	if !isOOBInteractive {
		otpStep, fetchedOTP := e.stepFetchOTP(acsTransID)
		steps = append(steps, otpStep)
		otp = fetchedOTP
	}

	// Step 3: Return interactive info — challenge URL + OTP for the tester
	challengeURL, _ := aresData["acsURL"].(string)
	steps = append(steps, stepResult{
		Step:   "interactive_ready",
		Status: "ok",
		Response: map[string]interface{}{
			"challengeURL": challengeURL,
			"otp":          otp,
			"acsTransID":   acsTransID,
			"flowType":     scenario.FlowType,
			"note":         "Use the challenge page to complete the flow manually",
		},
	})

	return steps
}

func (e *scenarioExec) executeScenario(scenario *postgres.TestScenario) []stepResult {
	var steps []stepResult

	// For challenge_unavailable, toggle resolvers before AReq
	if scenario.FlowType == "challenge_unavailable" {
		steps = append(steps, e.stepToggleResolvers(false, false))
	}

	// Step 1: Send AReq
	areqStep, aresData := e.stepAReq(scenario)
	steps = append(steps, areqStep)
	if areqStep.Status == "fail" {
		if scenario.FlowType == "challenge_unavailable" {
			steps = append(steps, e.stepToggleResolvers(true, true))
		}
		return steps
	}

	acsTransID, _ := aresData["acsTransID"].(string)
	transStatus, _ := aresData["transStatus"].(string)

	// Frictionless — we're done
	if scenario.FlowType == "frictionless" {
		if transStatus != "Y" {
			steps = append(steps, stepResult{
				Step:   "verify_frictionless",
				Status: "fail",
				Error:  fmt.Sprintf("expected transStatus=Y, got %s", transStatus),
			})
		} else {
			steps = append(steps, stepResult{Step: "verify_frictionless", Status: "ok"})
		}
		return steps
	}

	// Challenge unavailable — verify both channels down
	if scenario.FlowType == "challenge_unavailable" {
		if transStatus != "U" {
			steps = append(steps, stepResult{
				Step:   "verify_unavailable",
				Status: "fail",
				Error:  fmt.Sprintf("expected transStatus=U, got %s", transStatus),
			})
			steps = append(steps, e.stepToggleResolvers(true, true))
			return steps
		}
		steps = append(steps, stepResult{Step: "verify_unavailable", Status: "ok"})

		// Get fallback page
		steps = append(steps, e.stepGetChallengeFallback(acsTransID))

		// Restore resolvers
		steps = append(steps, e.stepToggleResolvers(true, true))
		return steps
	}

	// For OOB/decoupled flows, transStatus should be D; for OTP challenge it should be C
	isOOB := scenario.FlowType == "challenge_oob" || scenario.FlowType == "decoupled"
	expectedStatus := "C"
	if isOOB {
		expectedStatus = "D"
	}
	if transStatus != expectedStatus {
		steps = append(steps, stepResult{
			Step:   "verify_challenge_required",
			Status: "fail",
			Error:  fmt.Sprintf("expected transStatus=%s, got %s", expectedStatus, transStatus),
		})
		return steps
	}
	steps = append(steps, stepResult{Step: "verify_challenge_required", Status: "ok"})

	// Step 2: Fetch the generated OTP from audit log (skip for OOB — no OTP generated)
	otp := ""
	if !isOOB {
		otpStep, fetchedOTP := e.stepFetchOTP(acsTransID)
		steps = append(steps, otpStep)
		otp = fetchedOTP
	}

	// Step 3: Get challenge page (verifies session exists)
	challengeStep := e.stepGetChallenge(acsTransID)
	steps = append(steps, challengeStep)
	if challengeStep.Status == "fail" {
		return steps
	}

	// submitOTPAndMaybePIN submits OTP and, if the response indicates second factor, fetches + submits PIN.
	submitOTPAndMaybePIN := func(tid, otpCode string) {
		otpResult := e.stepSubmitOTP(tid, otpCode)
		steps = append(steps, otpResult)
		if otpResult.Status == "ok" {
			if resp, ok := otpResult.Response.(map[string]interface{}); ok {
				if needPIN, _ := resp["needSecondFactor"].(bool); needPIN {
					pinStep, pin := e.stepFetchPIN(tid)
					steps = append(steps, pinStep)
					if pinStep.Status == "ok" {
						steps = append(steps, e.stepSubmitPIN(tid, pin))
					}
				}
			}
		}
	}

	// Step 4: Execute the challenge action based on scenario
	switch scenario.FlowType {
	case "challenge_otp":
		switch scenario.ChallengeAction {
		case "submit_otp":
			submitOTPAndMaybePIN(acsTransID, otp)
		case "cancel":
			steps = append(steps, e.stepCancel(acsTransID))
		case "timeout":
			steps = append(steps, e.stepTimeout(acsTransID))
		default:
			submitOTPAndMaybePIN(acsTransID, otp)
		}

	case "challenge_oob", "decoupled":
		// Initiate OOB
		oobStep, oobData := e.stepInitiateOOB(acsTransID)
		steps = append(steps, oobStep)
		if oobStep.Status == "fail" {
			return steps
		}
		_ = oobData

		switch scenario.ChallengeAction {
		case "oob_approve":
			steps = append(steps, e.stepOOBResult(acsTransID, "approved"))
		case "oob_deny":
			steps = append(steps, e.stepOOBResult(acsTransID, "denied"))
		case "oob_fallback":
			fbStep := e.stepOOBFallback(acsTransID)
			steps = append(steps, fbStep)
			if fbStep.Status == "ok" {
				// Fetch new OTP after fallback
				otpStep2, newOtp := e.stepFetchOTP(acsTransID)
				steps = append(steps, otpStep2)
				submitOTPAndMaybePIN(acsTransID, newOtp)
			}
		case "cancel":
			steps = append(steps, e.stepCancel(acsTransID))
		case "timeout":
			steps = append(steps, e.stepTimeout(acsTransID))
		default:
			steps = append(steps, e.stepOOBResult(acsTransID, "approved"))
		}
	}

	return steps
}

// ── Individual Steps ──────────────────────────────────────────────────────

func (e *scenarioExec) stepAReq(s *postgres.TestScenario) (stepResult, map[string]interface{}) {
	start := time.Now()

	dsTransID := uuid.New().String()
	serverTransID := uuid.New().String()

	amt := s.PurchaseAmount
	cur := s.PurchaseCurrency
	exp := 2

	// "04" = challenge requested (mandate); "01" = no preference (frictionless)
	challengeInd := "01"
	if s.FlowType != "frictionless" {
		challengeInd = "04"
	}

	areq := map[string]interface{}{
		"messageType":                    "AReq",
		"messageVersion":                 "2.2.0",
		"messageCategory":                "01",
		"deviceChannel":                  s.DeviceChannel,
		"threeDSServerTransID":           serverTransID,
		"dsTransID":                      dsTransID,
		"acctNumber":                     s.CardNumber,
		"merchantName":                   s.MerchantName,
		"merchantCountry":                s.MerchantCountry,
		"purchaseAmount":                 &amt,
		"purchaseCurrency":               &cur,
		"purchaseExponent":               &exp,
		"threeDSRequestorChallengeInd":   &challengeInd,
	}

	// For OOB/decoupled flows, signal decoupled auth preference
	if s.FlowType == "challenge_oob" || s.FlowType == "decoupled" {
		decReqInd := "Y"
		areq["threeDSRequestorDecReqInd"] = &decReqInd
	}

	// Add browser info for browser channel
	if s.DeviceChannel == "02" {
		accept := "text/html"
		ip := "127.0.0.1"
		ua := "ScenarioRunner/1.0"
		lang := "en"
		colorDepth := "24"
		screenH := "1080"
		screenW := "1920"
		tz := 0
		javaEnabled := false
		jsEnabled := true
		areq["browserAcceptHeader"] = &accept
		areq["browserIP"] = &ip
		areq["browserUserAgent"] = &ua
		areq["browserLanguage"] = &lang
		areq["browserColorDepth"] = &colorDepth
		areq["browserScreenHeight"] = &screenH
		areq["browserScreenWidth"] = &screenW
		areq["browserTZ"] = &tz
		areq["browserJavaEnabled"] = &javaEnabled
		areq["browserJavascriptEnabled"] = &jsEnabled
	}

	// Add 3RI fields
	if s.DeviceChannel == "03" {
		threeRI := "01"
		areq["threeRIInd"] = &threeRI
	}

	respData, err := e.postJSON("/v1/auth", areq)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return stepResult{Step: "areq", Status: "fail", DurationMs: elapsed, Request: areq, Error: err.Error()}, nil
	}

	return stepResult{Step: "areq", Status: "ok", DurationMs: elapsed, Request: areq, Response: respData}, respData
}

func (e *scenarioExec) stepGetChallenge(acsTransID string) stepResult {
	start := time.Now()
	url := fmt.Sprintf("%s/v1/challenge/%s", e.h.acsBaseURL, acsTransID)
	resp, err := http.Get(url)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return stepResult{Step: "get_challenge_page", Status: "fail", DurationMs: elapsed, Error: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return stepResult{Step: "get_challenge_page", Status: "fail", DurationMs: elapsed, Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))}
	}
	return stepResult{Step: "get_challenge_page", Status: "ok", DurationMs: elapsed, Response: map[string]interface{}{"statusCode": 200, "contentType": resp.Header.Get("Content-Type")}}
}

func (e *scenarioExec) stepFetchOTP(acsTransID string) (stepResult, string) {
	start := time.Now()
	uid, err := uuid.Parse(acsTransID)
	if err != nil {
		return stepResult{Step: "fetch_otp", Status: "fail", DurationMs: 0, Error: "invalid UUID"}, ""
	}
	otp, err := e.h.repo.GetOTPForTransaction(e.ctx, uid)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return stepResult{Step: "fetch_otp", Status: "fail", DurationMs: elapsed, Error: err.Error()}, ""
	}
	return stepResult{Step: "fetch_otp", Status: "ok", DurationMs: elapsed, Response: map[string]interface{}{"otp": otp}}, otp
}

func (e *scenarioExec) stepSubmitOTP(acsTransID string, otp string) stepResult {
	start := time.Now()
	payload := map[string]interface{}{
		"acsTransID": acsTransID,
		"otpCode":    otp,
	}
	respData, err := e.postJSON("/v1/challenge/submit", payload)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return stepResult{Step: "submit_otp", Status: "fail", DurationMs: elapsed, Request: payload, Error: err.Error()}
	}
	return stepResult{Step: "submit_otp", Status: "ok", DurationMs: elapsed, Request: payload, Response: respData}
}

func (e *scenarioExec) stepFetchPIN(acsTransID string) (stepResult, string) {
	start := time.Now()
	uid, err := uuid.Parse(acsTransID)
	if err != nil {
		return stepResult{Step: "fetch_pin", Status: "fail", DurationMs: 0, Error: "invalid UUID"}, ""
	}
	pin, err := e.h.repo.GetPINForTransaction(e.ctx, uid)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return stepResult{Step: "fetch_pin", Status: "fail", DurationMs: elapsed, Error: err.Error()}, ""
	}
	return stepResult{Step: "fetch_pin", Status: "ok", DurationMs: elapsed, Response: map[string]interface{}{"pin": pin}}, pin
}

func (e *scenarioExec) stepSubmitPIN(acsTransID string, pin string) stepResult {
	start := time.Now()
	payload := map[string]interface{}{
		"acsTransID": acsTransID,
		"pinCode":    pin,
	}
	respData, err := e.postJSON("/v1/challenge/pin/submit", payload)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return stepResult{Step: "submit_pin", Status: "fail", DurationMs: elapsed, Request: payload, Error: err.Error()}
	}
	return stepResult{Step: "submit_pin", Status: "ok", DurationMs: elapsed, Request: payload, Response: respData}
}

func (e *scenarioExec) stepCancel(acsTransID string) stepResult {
	start := time.Now()
	url := fmt.Sprintf("%s/v1/challenge/%s/cancel", e.h.acsBaseURL, acsTransID)
	resp, err := http.Post(url, "application/json", nil)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return stepResult{Step: "cancel", Status: "fail", DurationMs: elapsed, Error: err.Error()}
	}
	defer resp.Body.Close()
	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	if resp.StatusCode >= 400 {
		return stepResult{Step: "cancel", Status: "fail", DurationMs: elapsed, Response: data, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return stepResult{Step: "cancel", Status: "ok", DurationMs: elapsed, Response: data}
}

func (e *scenarioExec) stepTimeout(acsTransID string) stepResult {
	start := time.Now()
	url := fmt.Sprintf("%s/v1/challenge/%s/timeout", e.h.acsBaseURL, acsTransID)
	resp, err := http.Post(url, "application/json", nil)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return stepResult{Step: "timeout", Status: "fail", DurationMs: elapsed, Error: err.Error()}
	}
	defer resp.Body.Close()
	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	if resp.StatusCode >= 400 {
		return stepResult{Step: "timeout", Status: "fail", DurationMs: elapsed, Response: data, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return stepResult{Step: "timeout", Status: "ok", DurationMs: elapsed, Response: data}
}

func (e *scenarioExec) stepInitiateOOB(acsTransID string) (stepResult, map[string]interface{}) {
	start := time.Now()
	respData, err := e.postJSON(fmt.Sprintf("/v1/challenge/%s/oob", acsTransID), map[string]interface{}{})
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return stepResult{Step: "initiate_oob", Status: "fail", DurationMs: elapsed, Error: err.Error()}, nil
	}
	return stepResult{Step: "initiate_oob", Status: "ok", DurationMs: elapsed, Response: respData}, respData
}

func (e *scenarioExec) stepOOBResult(acsTransID, result string) stepResult {
	start := time.Now()
	payload := map[string]interface{}{"result": result}
	respData, err := e.postJSON(fmt.Sprintf("/v1/challenge/%s/oob/result", acsTransID), payload)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return stepResult{Step: "oob_result_" + result, Status: "fail", DurationMs: elapsed, Request: payload, Error: err.Error()}
	}
	return stepResult{Step: "oob_result_" + result, Status: "ok", DurationMs: elapsed, Request: payload, Response: respData}
}

func (e *scenarioExec) stepOOBFallback(acsTransID string) stepResult {
	start := time.Now()
	respData, err := e.postJSON(fmt.Sprintf("/v1/challenge/%s/oob/fallback", acsTransID), map[string]interface{}{})
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return stepResult{Step: "oob_fallback", Status: "fail", DurationMs: elapsed, Error: err.Error()}
	}
	return stepResult{Step: "oob_fallback", Status: "ok", DurationMs: elapsed, Response: respData}
}

func (e *scenarioExec) stepToggleResolvers(otpAvail, oobAvail bool) stepResult {
	start := time.Now()
	if e.h.otpResolver != nil {
		e.h.otpResolver.SetAvailable(otpAvail)
	}
	if e.h.oobResolver != nil {
		e.h.oobResolver.SetAvailable(oobAvail)
	}
	elapsed := time.Since(start).Milliseconds()
	return stepResult{
		Step:       "toggle_resolvers",
		Status:     "ok",
		DurationMs: elapsed,
		Response:   map[string]interface{}{"otpAvailable": otpAvail, "oobAvailable": oobAvail},
	}
}

func (e *scenarioExec) stepGetChallengeFallback(acsTransID string) stepResult {
	start := time.Now()
	url := fmt.Sprintf("%s/v1/challenge/%s?flow=fallback", e.h.acsBaseURL, acsTransID)
	resp, err := http.Get(url)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return stepResult{Step: "get_fallback_page", Status: "fail", DurationMs: elapsed, Error: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return stepResult{Step: "get_fallback_page", Status: "fail", DurationMs: elapsed, Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))}
	}
	return stepResult{Step: "get_fallback_page", Status: "ok", DurationMs: elapsed, Response: map[string]interface{}{"statusCode": 200, "contentType": resp.Header.Get("Content-Type")}}
}

// ── OReq scenario ─────────────────────────────────────────────────────────

// executeOReq sends an OReq to the ACS (simulating the DS) and verifies the ORes.
func (e *scenarioExec) executeOReq(scenario *postgres.TestScenario) []stepResult {
	var steps []stepResult

	// Step 1: Send OReq to ACS
	oreqStep, oresData := e.stepOReq(scenario)
	steps = append(steps, oreqStep)
	if oreqStep.Status == "fail" {
		return steps
	}

	// Step 2: Validate ORes
	msgType, _ := oresData["messageType"].(string)
	if msgType != "ORes" {
		steps = append(steps, stepResult{
			Step:   "verify_ores",
			Status: "fail",
			Error:  fmt.Sprintf("expected messageType=ORes, got %s", msgType),
		})
		return steps
	}
	steps = append(steps, stepResult{Step: "verify_ores", Status: "ok", Response: oresData})

	return steps
}

func (e *scenarioExec) stepOReq(s *postgres.TestScenario) (stepResult, map[string]interface{}) {
	start := time.Now()

	// Determine availability from challengeAction field
	availability := "01" // default: available
	switch s.ChallengeAction {
	case "temp_unavailable":
		availability = "02"
	case "perm_unavailable":
		availability = "03"
	}

	oreq := map[string]interface{}{
		"messageType":        "OReq",
		"messageVersion":     "2.3.1",
		"acsOperatorID":      "scenario-runner",
		"acsReferenceNumber": "3DS_LOA_ACS_DEV_020200_00000",
		"acsAvailability":    availability,
	}

	respData, err := e.postJSON("/v1/oreq", oreq)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return stepResult{Step: "oreq", Status: "fail", DurationMs: elapsed, Request: oreq, Error: err.Error()}, nil
	}

	return stepResult{Step: "oreq", Status: "ok", DurationMs: elapsed, Request: oreq, Response: respData}, respData
}

// ── HTTP helpers ──────────────────────────────────────────────────────────

func (e *scenarioExec) postJSON(path string, body interface{}) (map[string]interface{}, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := e.h.acsBaseURL + path
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	if resp.StatusCode >= 400 {
		return data, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return data, nil
}

// ── Run listing & stats ───────────────────────────────────────────────────

func (h *ScenarioHandler) ListRuns(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	scenarioID, _ := strconv.ParseInt(c.Query("scenario_id"), 10, 64)

	search := c.Query("search")
	rows, total, err := h.repo.ListRuns(c.Request.Context(), scenarioID, search, page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total, "page": page, "per_page": perPage})
}

func (h *ScenarioHandler) GetRun(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	run, err := h.repo.GetRun(c.Request.Context(), id)
	if err != nil || run == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, run)
}

func (h *ScenarioHandler) GetRunStats(c *gin.Context) {
	stats, flowStats, err := h.repo.GetRunStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stats": stats, "byFlow": flowStats})
}