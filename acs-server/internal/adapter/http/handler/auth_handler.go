package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/acs-server/internal/adapter/http/dto"
	"github.com/your-org/acs-server/internal/adapter/http/validation"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/repository"
	"github.com/your-org/acs-server/internal/domain/service"
	"github.com/your-org/acs-server/internal/infrastructure/crypto"
	"github.com/your-org/acs-server/internal/infrastructure/dsclient"
	"github.com/your-org/acs-server/internal/usecase"
	"go.uber.org/zap"
)

// AuthHandler handles AReq → ARes authentication requests from the DS.
type AuthHandler struct {
	authUC       *usecase.AuthenticateUseCase
	acsRefNum    string // EMVCo-assigned ACS Reference Number
	sdkDecryptor *crypto.SDKEncDataDecryptor // nil if SDK key not configured
	msgRepo      repository.MessageRepository
	dsClient     *dsclient.Client
	log          *zap.Logger
}

// NewAuthHandler creates a new AuthHandler.
// sdkDecryptor may be nil; when nil, sdkEncData is skipped (dev/test only).
func NewAuthHandler(authUC *usecase.AuthenticateUseCase, acsRefNum string, sdkDecryptor *crypto.SDKEncDataDecryptor, msgRepo repository.MessageRepository, dsClient *dsclient.Client, log *zap.Logger) *AuthHandler {
	return &AuthHandler{authUC: authUC, acsRefNum: acsRefNum, sdkDecryptor: sdkDecryptor, msgRepo: msgRepo, dsClient: dsClient, log: log}
}

// HandleAReq processes POST /v1/auth
// This endpoint receives the AReq from the DS and returns the ARes.
func (h *AuthHandler) HandleAReq(c *gin.Context) {
	var req dto.AReqDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("invalid AReq binding", zap.Error(err))
		c.JSON(http.StatusBadRequest, errorResponse("2.2.0", "203", "A", "Message format invalid", err.Error(), "AReq"))
		return
	}

	// EMVCo field-level validation (conditional groups, enum values, length constraints)
	if errs := validation.ValidateAReq(&req); len(errs) > 0 {
		first := errs[0]
		h.log.Warn("AReq validation failed", zap.String("field", first.Field), zap.String("code", first.ErrorCode), zap.Int("total_errors", len(errs)))
		c.JSON(http.StatusBadRequest, errorResponse(req.MessageVersion, first.ErrorCode, "A", first.Description, first.Field, "AReq"))
		return
	}

	// Parse UUIDs
	dsTransID, err := uuid.Parse(req.DSTransID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(req.MessageVersion, "203", "A", "Invalid dsTransID", err.Error(), "AReq"))
		return
	}
	serverTransID, err := uuid.Parse(req.ThreeDSServerTransID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(req.MessageVersion, "203", "A", "Invalid threeDSServerTransID", err.Error(), "AReq"))
		return
	}

	// Map SDK trans ID
	var sdkTransID *uuid.UUID
	if req.SDKTransID != nil {
		if id, e := uuid.Parse(*req.SDKTransID); e == nil {
			sdkTransID = &id
		}
	}

	var browserInfo *service.BrowserInfo
	var sdkDeviceInfo *service.SDKDeviceInfo

	switch req.DeviceChannel {
	case string(entity.DeviceChannelBrowser): // "02"
		if req.BrowserIP != nil {
			browserInfo = &service.BrowserInfo{
				IPAddress: derefStr(req.BrowserIP),
				UserAgent: derefStr(req.BrowserUserAgent),
				Language:  derefStr(req.BrowserLanguage),
			}
			if req.BrowserTZ != nil {
				browserInfo.TimeZoneOffset = *req.BrowserTZ
			}
			if req.BrowserJavaEnabled != nil {
				browserInfo.JavaEnabled = *req.BrowserJavaEnabled
			}
			if req.BrowserJavascriptEnabled != nil {
				browserInfo.JavaScriptEnabled = *req.BrowserJavascriptEnabled
			}
		}

	case string(entity.DeviceChannelApp): // "01"
		if req.SDKEncData != nil {
			if h.sdkDecryptor == nil {
				h.log.Warn("sdkEncData present but SDK decryptor not configured — skipping decryption")
			} else {
				payload, decryptErr := h.sdkDecryptor.Decrypt(*req.SDKEncData)
				if decryptErr != nil {
					h.log.Error("sdkEncData decryption failed", zap.Error(decryptErr))
					c.JSON(http.StatusBadRequest, errorResponse(req.MessageVersion, "203", "A", "Invalid sdkEncData", decryptErr.Error(), "AReq"))
					return
				}
				sdkDeviceInfo = &service.SDKDeviceInfo{
					SDKType:             payload.SDKType,
					DeviceBindingStatus: payload.DeviceBindingStatus,
					SDKVersion:          payload.SDKVersion,
					AppVersion:          payload.AppVersion,
					SDKReferenceNumber:  payload.SDKReferenceNumber,
					ClientDeviceInfo:    payload.ClientDeviceInfo,
				}
			}
		} else if sdkTransID != nil {
			// sdkTransID present but no sdkEncData — log and continue without device info
			h.log.Warn("app-channel AReq missing sdkEncData", zap.String("sdkTransID", sdkTransID.String()))
		}
	}

	input := &usecase.AuthenticateInput{
		MessageVersion:               req.MessageVersion,
		DeviceChannel:                entity.DeviceChannel(req.DeviceChannel),
		MessageCategory:              entity.MessageCategory(req.MessageCategory),
		DSTransactionID:              dsTransID,
		ServerTransactionID:          serverTransID,
		SDKTransactionID:             sdkTransID,
		AcctNumber:                   req.AcctNumber,
		MerchantName:                 req.MerchantName,
		MerchantCountry:              req.MerchantCountry,
		MerchantID:                   req.MerchantID,
		PurchaseAmount:               req.PurchaseAmount,
		PurchaseCurrency:             req.PurchaseCurrency,
		PurchaseExponent:             req.PurchaseExponent,
		ThreeDSRequestorChallengeInd: derefStr(req.ThreeDSRequestorChallengeInd),
		ThreeDSRequestorDecReqInd:    derefStr(req.ThreeDSRequestorDecReqInd),
		BrowserInfo:                  browserInfo,
		SDKDeviceInfo:                sdkDeviceInfo,
		SDKEphemPubKey:               req.SDKEphemPubKey,
		ThreeDSMethodNotifURL:        req.ThreeDSMethodNotifURL,
	}

	out, err := h.authUC.Execute(c.Request.Context(), input)
	if err != nil {
		h.log.Error("authentication use case error", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResponse(req.MessageVersion, "999", "A", "Internal error", err.Error(), "AReq"))
		return
	}

	ares := dto.AResDTO{
		MessageType:        "ARes",
		MessageVersion:     req.MessageVersion,
		ACSTransID:         out.ACSTransactionID.String(),
		DSTransID:          req.DSTransID,
		ServerTransID:      req.ThreeDSServerTransID,
		TransStatus:        string(out.TransStatus),
		TransStatusReason:  out.TransStatusReason,
		AuthValue:          out.AuthValue,
		ECI:                out.ECI,
		ACSURL:             out.ACSChallengeURL,
		ACSReferenceNumber: h.acsRefNum,
	}

	h.saveMessages(c.Request.Context(), out.ACSTransactionID, &req, &ares)

	// For frictionless (Y) and rejected (R/N) outcomes, send RReq to DS.
	if h.dsClient != nil && (out.TransStatus == entity.TransStatusAuthenticated || out.TransStatus == entity.TransStatusNotAuthenticated || out.TransStatus == entity.TransStatusRejected) {
		dsTransID, _ := uuid.Parse(req.DSTransID)
		serverTransID, _ := uuid.Parse(req.ThreeDSServerTransID)
		go h.dsClient.SendRReq(c.Request.Context(), &dsclient.RReqPayload{
			ACSTransactionID:    out.ACSTransactionID,
			DSTransactionID:     dsTransID,
			ServerTransactionID: serverTransID,
			TransStatus:         out.TransStatus,
			AuthValue:           out.AuthValue,
			ECI:                 out.ECI,
			MessageVersion:      entity.SpecVersion(req.MessageVersion),
			Network:             out.Network,
		})
	}

	c.JSON(http.StatusOK, ares)
}

// saveMessages persists the AReq (inbound) and ARes (outbound) pair to the messages table.
func (h *AuthHandler) saveMessages(ctx context.Context, acsTransID uuid.UUID, req *dto.AReqDTO, ares *dto.AResDTO) {
	if h.msgRepo == nil {
		return
	}
	now := time.Now().UTC()
	for _, m := range []struct {
		obj     any
		msgType string
		dir     string
	}{
		{req, "AReq", "inbound"},
		{ares, "ARes", "outbound"},
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

func errorResponse(version, code, component, desc, detail, msgType string) dto.ErrorDTO {
	if version == "" {
		version = "2.2.0"
	}
	return dto.ErrorDTO{
		MessageType:      "Erro",
		MessageVersion:   version,
		ErrorCode:        code,
		ErrorComponent:   component,
		ErrorDescription: desc,
		ErrorDetail:      detail,
		ErrorMessageType: msgType,
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}