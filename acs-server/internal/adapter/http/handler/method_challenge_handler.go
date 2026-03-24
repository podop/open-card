package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/acs-server/internal/adapter/http/dto"
	"github.com/your-org/acs-server/internal/adapter/http/validation"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/repository"
	"github.com/your-org/acs-server/internal/infrastructure/dsclient"
	"github.com/your-org/acs-server/internal/usecase"
	"go.uber.org/zap"
)

// MethodHandler handles the 3DS Method device fingerprinting flow.
type MethodHandler struct {
	methodUC *usecase.ThreeDSMethodUseCase
	log      *zap.Logger
}

// NewMethodHandler creates a new MethodHandler.
func NewMethodHandler(methodUC *usecase.ThreeDSMethodUseCase, log *zap.Logger) *MethodHandler {
	return &MethodHandler{methodUC: methodUC, log: log}
}

// HandleMethodInit handles POST /v1/method
// Called by the browser iframe. Parses the threeDSMethodData form field.
func (h *MethodHandler) HandleMethodInit(c *gin.Context) {
	var form dto.ThreeDSMethodFormDTO
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing threeDSMethodData"})
		return
	}

	input := &usecase.InitiateMethodInput{
		ThreeDSMethodDataEncoded: form.ThreeDSMethodData,
		RemoteAddr:               c.ClientIP(),
		IPAddress:                c.ClientIP(),
		UserAgent:                c.GetHeader("User-Agent"),
		AcceptHeader:             c.GetHeader("Accept"),
	}

	out, err := h.methodUC.Initiate(c.Request.Context(), input)
	if err != nil {
		h.log.Error("3DS Method initiation failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Respond with a self-posting HTML form that notifies the 3DS Server.
	// Per spec: ACS posts threeDSMethodData back to threeDSMethodNotificationURL.
	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, methodCompletionHTML(out.SessionID.String(), out.NotifURL))
}

// HandleMethodNotification handles POST /v1/method/notification/:sessionID
// The browser posts back here after the ACS iframe loads.
func (h *MethodHandler) HandleMethodNotification(c *gin.Context) {
	sessionIDStr := c.Param("sessionID")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sessionID"})
		return
	}

	notifURL, err := h.methodUC.Complete(c.Request.Context(), sessionID)
	if err != nil {
		h.log.Error("3DS Method completion failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Post completion notification to the 3DS Server notification URL.
	// In production this would be an async HTTP POST; here we redirect.
	h.log.Info("3DS Method completed, posting notification", zap.String("notifURL", notifURL))
	c.Status(http.StatusOK)
}

// methodCompletionHTML renders the silent iframe HTML that auto-submits to the notification URL.
func methodCompletionHTML(sessionID, notifURL string) string {
	t := template.Must(template.New("method").Parse(
		`<!DOCTYPE html><html><head><meta charset="UTF-8"></head><body>
<form id="notifForm" method="POST" action="{{.NotifURL}}">
  <input type="hidden" name="threeDSMethodData" value="{{.SessionID}}">
</form>
<script>document.getElementById("notifForm").submit();</script>
</body></html>`))
	var buf strings.Builder
	_ = t.Execute(&buf, struct{ SessionID, NotifURL string }{sessionID, notifURL})
	return buf.String()
}

// ────────────────────────────────────────────────────────────────
// ChallengeHandler
// ────────────────────────────────────────────────────────────────

// ChallengeHandler handles the OTP-based challenge flow.
type ChallengeHandler struct {
	challengeUC *usecase.ChallengeUseCase
	msgRepo     repository.MessageRepository
	dsClient    *dsclient.Client
	log         *zap.Logger
}

// NewChallengeHandler creates a new ChallengeHandler.
func NewChallengeHandler(challengeUC *usecase.ChallengeUseCase, msgRepo repository.MessageRepository, dsClient *dsclient.Client, log *zap.Logger) *ChallengeHandler {
	return &ChallengeHandler{challengeUC: challengeUC, msgRepo: msgRepo, dsClient: dsClient, log: log}
}

// parseCReq deserialises a CReq from either:
//   - application/json          (app channel, 3DS Server → ACS direct call)
//   - application/x-www-form-urlencoded  (browser channel, base64url `creq` field)
func parseCReq(c *gin.Context) (*dto.CReqDTO, error) {
	raw := c.GetHeader("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(raw)

	if strings.EqualFold(mediaType, "application/x-www-form-urlencoded") {
		// Browser transport: `creq` field carries base64url-encoded CReq JSON.
		encoded := c.PostForm("creq")
		decoded, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return nil, err
		}
		var req dto.CReqDTO
		if err := json.Unmarshal(decoded, &req); err != nil {
			return nil, err
		}
		return &req, nil
	}

	// Default: application/json
	var req dto.CReqDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

// HandleCReq handles POST /v1/challenge/creq
// Receives a CReq from the 3DS Server or browser and renders the network-specific
// OTP challenge page (initial CRes to the browser).
//   - application/json                  — app channel / direct DS call
//   - application/x-www-form-urlencoded — browser channel (base64url `creq` field)
func (h *ChallengeHandler) HandleCReq(c *gin.Context) {
	req, err := parseCReq(c)
	if err != nil {
		h.log.Warn("invalid CReq binding", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"errorCode": "203", "errorDescription": "Message format invalid", "errorDetail": err.Error()})
		return
	}

	if errs := validation.ValidateCReq(req); len(errs) > 0 {
		first := errs[0]
		h.log.Warn("CReq validation failed",
			zap.String("field", first.Field),
			zap.String("code", first.ErrorCode),
			zap.Int("total_errors", len(errs)),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"errorCode":        first.ErrorCode,
			"errorDescription": first.Description,
			"errorDetail":      first.Field,
		})
		return
	}

	acsTransID, err := uuid.Parse(req.ACSTransID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errorCode": "203", "errorDescription": "invalid acsTransID"})
		return
	}

	info, err := h.challengeUC.GetChallengeInfo(c.Request.Context(), acsTransID)
	if err != nil {
		h.log.Error("get challenge info failed", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found or not in challenge state"})
		return
	}

	txnID := acsTransID.String()
	renderChallengePage(c, info.Network, challengePageData{
		ACSTransID:   txnID,
		MerchantName: info.MerchantName,
		Amount:       formatAmount(info.PurchaseAmount, info.PurchaseCurrency),
		AttemptsLeft: info.AttemptsLeft,
		CSRFToken:    globalCSRF.Generate(txnID),
	})
}

// HandleChallengeInit handles GET /v1/challenge/:acsTransID
// Renders the network-specific OTP challenge page for the cardholder.
// For OOB/decoupled flows, pass ?flow=oob to render the OOB waiting page instead.
// For fallback (both channels unavailable), pass ?flow=fallback to render the unavailable page.
func (h *ChallengeHandler) HandleChallengeInit(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	// Fallback flow bypasses GetChallengeInfo (transaction is already U status).
	if c.Query("flow") == "fallback" {
		h.handleFallbackInit(c, acsTransID)
		return
	}

	info, err := h.challengeUC.GetChallengeInfo(c.Request.Context(), acsTransID)
	if err != nil {
		h.log.Error("get challenge info failed", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found or not in challenge state"})
		return
	}

	txnID := acsTransID.String()
	data := challengePageData{
		ACSTransID:   txnID,
		MerchantName: info.MerchantName,
		Amount:       formatAmount(info.PurchaseAmount, info.PurchaseCurrency),
		AttemptsLeft: info.AttemptsLeft,
		CSRFToken:    globalCSRF.Generate(txnID),
	}

	if flow := c.Query("flow"); flow == "oob" || flow == "decoupled" {
		renderOOBPage(c, info.Network, data)
		return
	}

	renderChallengePage(c, info.Network, data)
}

// handleFallbackInit renders the "Verification Unavailable" page for transactions
// where both challenge channels (OTP and OOB) are unavailable (TransStatus=U).
func (h *ChallengeHandler) handleFallbackInit(c *gin.Context, acsTransID uuid.UUID) {
	info, err := h.challengeUC.GetTransactionDisplayInfo(c.Request.Context(), acsTransID)
	if err != nil {
		h.log.Error("get transaction display info failed", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	cres := dto.CResDTO{
		MessageType:            "CRes",
		MessageVersion:         string(info.SpecVersion),
		ACSTransID:             acsTransID.String(),
		ThreeDSServerTransID:   info.ServerTransactionID.String(),
		TransStatus:            "U",
		ChallengeCompletionInd: "Y",
	}
	cresJSON, _ := json.Marshal(cres)
	cresB64 := base64.RawURLEncoding.EncodeToString(cresJSON)

	renderFallbackPage(c, challengeCompleteData{
		NotifURL: info.NotifURL,
		CResB64:  cresB64,
	})
}

// HandlePINInit handles GET /v1/challenge/:acsTransID/pin
// Renders the network-specific Card PIN challenge page (second factor).
func (h *ChallengeHandler) HandlePINInit(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	info, err := h.challengeUC.GetPINInfo(c.Request.Context(), acsTransID)
	if err != nil {
		h.log.Error("get PIN info failed", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found or not in PIN phase"})
		return
	}

	txnID := acsTransID.String()
	renderPINChallengePage(c, info.Network, challengePageData{
		ACSTransID:   txnID,
		MerchantName: info.MerchantName,
		Amount:       formatAmount(info.PurchaseAmount, info.PurchaseCurrency),
		AttemptsLeft: info.AttemptsLeft,
		CSRFToken:    globalCSRF.Generate(txnID + "_pin"),
	})
}

// HandlePINFormSubmit handles POST /v1/challenge/:acsTransID/pin
// Processes the card PIN submitted via the browser HTML form (second factor).
func (h *ChallengeHandler) HandlePINFormSubmit(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	txnID := acsTransID.String()
	if !globalCSRF.Validate(txnID+"_pin", c.PostForm("csrfToken")) {
		h.log.Warn("CSRF validation failed on PIN submission", zap.String("acsTransID", txnID))
		c.String(http.StatusForbidden, "Invalid or duplicate request. Please go back and try again.")
		return
	}

	pinCode := c.PostForm("pinCode")

	out, err := h.challengeUC.SubmitPIN(c.Request.Context(), &usecase.SubmitPINInput{
		ACSTransactionID: acsTransID,
		SubmittedPIN:     pinCode,
	})
	if err != nil {
		h.log.Error("PIN submit failed", zap.Error(err))
		c.String(http.StatusInternalServerError, "Authentication error. Please try again.")
		return
	}

	// Final outcome → render completion page
	if out.RReqPayload != nil {
		cres := dto.CResDTO{
			MessageType:            "CRes",
			MessageVersion:         string(out.RReqPayload.MessageVersion),
			ACSTransID:             txnID,
			ThreeDSServerTransID:   out.RReqPayload.ServerTransactionID.String(),
			TransStatus:            string(out.TransStatus),
			ChallengeCompletionInd: "Y",
		}
		h.saveChallengeMessages(c.Request.Context(), acsTransID, gin.H{"acsTransID": txnID, "pinCode": "***"}, &cres)
		h.sendRReq(c.Request.Context(), out.RReqPayload)
		renderCompletePage(c, out.NotifURL, cres)
		return
	}

	// Wrong PIN, attempts remaining → re-render PIN page
	renderPINChallengePage(c, out.Network, challengePageData{
		ACSTransID:   txnID,
		AttemptsLeft: out.AttemptsLeft,
		Error:        "Incorrect PIN. Please try again.",
		CSRFToken:    globalCSRF.Refresh(txnID + "_pin"),
	})
}

// HandlePINSubmitJSON handles POST /v1/challenge/pin/submit (JSON API for non-browser clients / scenario runner).
func (h *ChallengeHandler) HandlePINSubmitJSON(c *gin.Context) {
	var req struct {
		ACSTransID string `json:"acsTransID"`
		PINCode    string `json:"pinCode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	acsTransID, err := uuid.Parse(req.ACSTransID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	out, err := h.challengeUC.SubmitPIN(c.Request.Context(), &usecase.SubmitPINInput{
		ACSTransactionID: acsTransID,
		SubmittedPIN:     req.PINCode,
	})
	if err != nil {
		h.log.Error("PIN submit (JSON) failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if out.RReqPayload != nil {
		cres := dto.CResDTO{
			MessageType:            "CRes",
			MessageVersion:         string(out.RReqPayload.MessageVersion),
			ACSTransID:             req.ACSTransID,
			ThreeDSServerTransID:   out.RReqPayload.ServerTransactionID.String(),
			TransStatus:            string(out.TransStatus),
			ChallengeCompletionInd: "Y",
		}
		h.saveChallengeMessages(c.Request.Context(), acsTransID, gin.H{"acsTransID": req.ACSTransID, "pinCode": "***"}, &cres)
		h.sendRReq(c.Request.Context(), out.RReqPayload)
		c.JSON(http.StatusOK, cres)
		return
	}

	// Wrong PIN or max attempts
	c.JSON(http.StatusOK, gin.H{
		"transStatus":  string(out.TransStatus),
		"attemptsLeft": out.AttemptsLeft,
		"error":        "Incorrect PIN",
	})
}

// HandleChallengeFormSubmit handles POST /v1/challenge/:acsTransID/form
// Processes the OTP submitted via the browser HTML form.
//   - Wrong OTP with attempts remaining → re-renders the network-specific page with an error.
//   - Final outcome (success or max attempts) → renders the completion page that auto-posts
//     the CRes (base64url JSON) to the 3DS Server notification URL.
func (h *ChallengeHandler) HandleChallengeFormSubmit(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	txnID := acsTransID.String()
	if !globalCSRF.Validate(txnID, c.PostForm("csrfToken")) {
		h.log.Warn("CSRF validation failed or duplicate submission", zap.String("acsTransID", txnID))
		c.String(http.StatusForbidden, "Invalid or duplicate request. Please go back and try again.")
		return
	}

	otpCode := c.PostForm("otpCode")

	out, err := h.challengeUC.SubmitChallenge(c.Request.Context(), &usecase.SubmitChallengeInput{
		ACSTransactionID: acsTransID,
		SubmittedOTP:     otpCode,
	})
	if err != nil {
		h.log.Error("challenge form submit failed", zap.Error(err))
		c.String(http.StatusInternalServerError, "Authentication error. Please try again.")
		return
	}

	// OTP success but second factor (PIN) required → redirect to PIN page
	if out.NeedSecondFactor {
		c.Redirect(http.StatusFound, fmt.Sprintf("/v1/challenge/%s/pin", txnID))
		return
	}

	// Final outcome → render completion page that auto-posts CRes to the notification URL.
	if out.RReqPayload != nil {
		cres := dto.CResDTO{
			MessageType:            "CRes",
			MessageVersion:         string(out.RReqPayload.MessageVersion),
			ACSTransID:             txnID,
			ThreeDSServerTransID:   out.RReqPayload.ServerTransactionID.String(),
			TransStatus:            string(out.TransStatus),
			ChallengeCompletionInd: "Y",
		}
		h.saveChallengeMessages(c.Request.Context(), acsTransID, gin.H{"acsTransID": txnID, "otpCode": otpCode}, &cres)
		h.sendRReq(c.Request.Context(), out.RReqPayload)
		renderCompletePage(c, out.NotifURL, cres)
		return
	}

	// Wrong OTP, attempts remaining → re-render the network-specific page with a fresh CSRF token.
	renderChallengePage(c, out.Network, challengePageData{
		ACSTransID:   txnID,
		AttemptsLeft: out.AttemptsLeft,
		Error:        "Incorrect code. Please try again.",
		CSRFToken:    globalCSRF.Refresh(txnID),
	})
}

// HandleChallengeCancel handles POST /v1/challenge/:acsTransID/cancel
// Cardholder clicks "Cancel" on the challenge page.
// Per EMV 3DS §5.4.2: transStatus=N, challengeCancel=01.
func (h *ChallengeHandler) HandleChallengeCancel(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	// CSRF check for browser form submissions; JSON API clients skip this.
	if c.ContentType() != "application/json" {
		txnID := acsTransID.String()
		if !globalCSRF.Validate(txnID, c.PostForm("csrfToken")) {
			h.log.Warn("CSRF validation failed or duplicate cancel", zap.String("acsTransID", txnID))
			c.String(http.StatusForbidden, "Invalid or duplicate request.")
			return
		}
	}

	out, err := h.challengeUC.CancelChallenge(c.Request.Context(), acsTransID)
	if err != nil {
		h.log.Error("challenge cancel failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cres := dto.CResDTO{
		MessageType:            "CRes",
		MessageVersion:         string(out.SpecVersion),
		ACSTransID:             acsTransID.String(),
		ThreeDSServerTransID:   out.ServerTransactionID.String(),
		TransStatus:            string(out.TransStatus),
		ChallengeCompletionInd: "Y",
	}
	h.saveChallengeMessages(c.Request.Context(), acsTransID,
		gin.H{"acsTransID": acsTransID.String(), "action": "cancel"}, &cres)
	h.sendRReq(c.Request.Context(), out.RReqPayload)

	// If browser: render cancelled page (or auto-post if notifURL exists). If JSON: return CRes.
	if c.GetHeader("Accept") == "application/json" || c.ContentType() == "application/json" {
		c.JSON(http.StatusOK, cres)
		return
	}
	if out.NotifURL == "" {
		renderCancelledPage(c)
		return
	}
	renderCompletePage(c, out.NotifURL, cres)
}

// HandleChallengeTimeout handles POST /v1/challenge/:acsTransID/timeout
// Called by the browser when the challenge timer expires (client-side JS timeout).
// Per EMV 3DS §5.4.3: transStatus=U.
func (h *ChallengeHandler) HandleChallengeTimeout(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	// CSRF check for browser form submissions; JSON API clients skip this.
	if c.ContentType() != "application/json" {
		txnID := acsTransID.String()
		if !globalCSRF.Validate(txnID, c.PostForm("csrfToken")) {
			h.log.Warn("CSRF validation failed or duplicate timeout", zap.String("acsTransID", txnID))
			c.String(http.StatusForbidden, "Invalid or duplicate request.")
			return
		}
	}

	out, err := h.challengeUC.TimeoutChallenge(c.Request.Context(), acsTransID)
	if err != nil {
		h.log.Error("challenge timeout failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cres := dto.CResDTO{
		MessageType:            "CRes",
		MessageVersion:         string(out.SpecVersion),
		ACSTransID:             acsTransID.String(),
		ThreeDSServerTransID:   out.ServerTransactionID.String(),
		TransStatus:            string(out.TransStatus),
		ChallengeCompletionInd: "Y",
	}
	h.saveChallengeMessages(c.Request.Context(), acsTransID,
		gin.H{"acsTransID": acsTransID.String(), "action": "timeout"}, &cres)
	h.sendRReq(c.Request.Context(), out.RReqPayload)

	if c.GetHeader("Accept") == "application/json" || c.ContentType() == "application/json" {
		c.JSON(http.StatusOK, cres)
		return
	}
	renderCompletePage(c, out.NotifURL, cres)
}

// HandleOOBInitiate handles POST /v1/challenge/:acsTransID/oob
// Starts an OOB/decoupled authentication flow.
func (h *ChallengeHandler) HandleOOBInitiate(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	out, err := h.challengeUC.InitiateOOBChallenge(c.Request.Context(), acsTransID)
	if err != nil {
		h.log.Error("OOB initiation failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"acsTransID":       out.ACSTransactionID.String(),
		"transStatus":      "D",
		"oobAppURL":        out.OOBAppURL,
		"oobContinueLabel": out.OOBContinueLabel,
	})
}

// HandleOOBStatus handles GET /v1/challenge/:acsTransID/oob/status
// The browser polls this endpoint while waiting for the cardholder to act on the OOB channel.
func (h *ChallengeHandler) HandleOOBStatus(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	out, err := h.challengeUC.GetOOBStatus(c.Request.Context(), acsTransID)
	if err != nil {
		h.log.Error("OOB status check failed", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transStatus": string(out.TransStatus),
		"completed":   out.Completed,
	})
}

// HandleOOBFallback handles POST /v1/challenge/:acsTransID/oob/fallback
// Reverts from OOB/decoupled (D) back to OTP challenge (C) when the OOB push expires.
func (h *ChallengeHandler) HandleOOBFallback(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	out, err := h.challengeUC.FallbackOOBToOTP(c.Request.Context(), acsTransID)
	if err != nil {
		h.log.Error("OOB fallback failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"acsTransID":   out.ACSTransactionID.String(),
		"transStatus":  string(out.TransStatus),
		"attemptsLeft": out.AttemptsLeft,
	})
}

// HandleOOBResult handles POST /v1/challenge/:acsTransID/oob/result
// Callback from the OOB channel (e.g. banking app) with the cardholder's decision.
func (h *ChallengeHandler) HandleOOBResult(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	var req struct {
		Approved bool `json:"approved"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out, err := h.challengeUC.CompleteOOBChallenge(c.Request.Context(), &usecase.OOBResultInput{
		ACSTransactionID: acsTransID,
		Approved:         req.Approved,
	})
	if err != nil {
		h.log.Error("OOB result processing failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cres := dto.CResDTO{
		MessageType:            "CRes",
		MessageVersion:         string(out.SpecVersion),
		ACSTransID:             acsTransID.String(),
		ThreeDSServerTransID:   out.ServerTransactionID.String(),
		TransStatus:            string(out.TransStatus),
		ChallengeCompletionInd: "Y",
	}
	h.saveChallengeMessages(c.Request.Context(), acsTransID,
		gin.H{"acsTransID": acsTransID.String(), "action": "oob_result", "approved": req.Approved}, &cres)
	h.sendRReq(c.Request.Context(), out.RReqPayload)

	c.JSON(http.StatusOK, cres)
}

// HandleChallengeSubmit handles POST /v1/challenge/submit (JSON API — kept for non-browser clients).
func (h *ChallengeHandler) HandleChallengeSubmit(c *gin.Context) {
	var req dto.OTPSubmitDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	acsTransID, err := uuid.Parse(req.ACSTransID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	out, err := h.challengeUC.SubmitChallenge(c.Request.Context(), &usecase.SubmitChallengeInput{
		ACSTransactionID: acsTransID,
		SubmittedOTP:     req.OTPCode,
	})
	if err != nil {
		h.log.Error("challenge submit failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if out.NeedSecondFactor {
		c.JSON(http.StatusOK, gin.H{
			"needSecondFactor": true,
			"acsTransID":      req.ACSTransID,
			"pinURL":          fmt.Sprintf("/v1/challenge/%s/pin", req.ACSTransID),
		})
		return
	}

	completionInd := "N"
	if out.RReqPayload != nil {
		completionInd = "Y"
	}
	cres := dto.CResDTO{
		MessageType:            "CRes",
		MessageVersion:         string(out.SpecVersion),
		ACSTransID:             req.ACSTransID,
		ThreeDSServerTransID:   out.ServerTransactionID.String(),
		TransStatus:            string(out.TransStatus),
		ChallengeCompletionInd: completionInd,
	}
	h.saveChallengeMessages(c.Request.Context(), acsTransID, req, &cres)
	h.sendRReq(c.Request.Context(), out.RReqPayload)
	c.JSON(http.StatusOK, cres)
}

// sendRReq converts a usecase RReqPayload and dispatches it via the DS client.
func (h *ChallengeHandler) sendRReq(ctx context.Context, rreq *usecase.RReqPayload) {
	if h.dsClient == nil || rreq == nil {
		return
	}
	h.dsClient.SendRReq(ctx, &dsclient.RReqPayload{
		ACSTransactionID:    rreq.ACSTransactionID,
		DSTransactionID:     rreq.DSTransactionID,
		ServerTransactionID: rreq.ServerTransactionID,
		TransStatus:         rreq.TransStatus,
		AuthValue:           rreq.AuthValue,
		ECI:                 rreq.ECI,
		MessageVersion:      rreq.MessageVersion,
		Network:             rreq.Network,
	})
}

// saveChallengeMessages persists a CReq (inbound) and CRes (outbound) pair.
func (h *ChallengeHandler) saveChallengeMessages(ctx context.Context, acsTransID uuid.UUID, req any, cres *dto.CResDTO) {
	if h.msgRepo == nil {
		return
	}
	now := time.Now().UTC()
	for _, m := range []struct {
		obj     any
		msgType string
		dir     string
	}{
		{req, "CReq", "inbound"},
		{cres, "CRes", "outbound"},
	} {
		raw, err := json.Marshal(m.obj)
		if err != nil {
			h.log.Warn("failed to marshal message for storage", zap.String("type", m.msgType), zap.Error(err))
			continue
		}
		txID := acsTransID
		if err := h.msgRepo.Save(ctx, &entity.Message{
			ACSTransactionID: &txID,
			MessageType:      m.msgType,
			Direction:        m.dir,
			Content:          raw,
			CreatedAt:        now,
		}); err != nil {
			h.log.Warn("failed to save message", zap.String("type", m.msgType), zap.Error(err))
		}
	}
}

// formatAmount builds a display string from optional amount/currency pointers.
func formatAmount(amount, currency *string) string {
	if amount == nil || currency == nil {
		return ""
	}
	return *amount + " " + *currency
}
