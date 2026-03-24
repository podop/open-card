package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/acs-server/internal/adapter/http/dto"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/repository"
	"github.com/your-org/acs-server/internal/infrastructure/config"
	"go.uber.org/zap"
)

// OReqHandler handles inbound OReq messages from the Directory Server.
// Per EMV 3DS spec, the DS originates OReq and the ACS responds with ORes.
type OReqHandler struct {
	cfg     *config.Config
	msgRepo repository.MessageRepository
	log     *zap.Logger
}

// NewOReqHandler creates a new OReqHandler.
func NewOReqHandler(cfg *config.Config, msgRepo repository.MessageRepository, log *zap.Logger) *OReqHandler {
	return &OReqHandler{cfg: cfg, msgRepo: msgRepo, log: log}
}

// HandleOReq processes POST /v1/oreq — receives OReq from DS and returns ORes.
func (h *OReqHandler) HandleOReq(c *gin.Context) {
	var oreq dto.OReqDTO
	if err := c.ShouldBindJSON(&oreq); err != nil {
		h.log.Warn("invalid OReq payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, dto.ErrorDTO{
			MessageType:      "Erro",
			MessageVersion:   "2.2.0",
			ErrorCode:        "203",
			ErrorComponent:   "A",
			ErrorDescription: "Message format invalid",
			ErrorDetail:      err.Error(),
			ErrorMessageType: "OReq",
		})
		return
	}

	// Validate messageType
	if oreq.MessageType != "OReq" {
		h.log.Warn("unexpected messageType in OReq", zap.String("messageType", oreq.MessageType))
		c.JSON(http.StatusBadRequest, dto.ErrorDTO{
			MessageType:      "Erro",
			MessageVersion:   oreq.MessageVersion,
			ErrorCode:        "101",
			ErrorComponent:   "A",
			ErrorDescription: "Message type not valid",
			ErrorDetail:      "expected OReq",
			ErrorMessageType: "OReq",
		})
		return
	}

	// Validate required fields
	if oreq.MessageVersion == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorDTO{
			MessageType:      "Erro",
			MessageVersion:   "2.2.0",
			ErrorCode:        "201",
			ErrorComponent:   "A",
			ErrorDescription: "Required data element missing",
			ErrorDetail:      "messageVersion",
			ErrorMessageType: "OReq",
		})
		return
	}

	now := time.Now().UTC()

	h.log.Info("OReq received from DS",
		zap.String("acsOperatorID", oreq.ACSOperatorID),
		zap.String("acsReferenceNumber", oreq.ACSReferenceNumber),
		zap.String("availability", oreq.ACSAvailability),
		zap.String("messageVersion", oreq.MessageVersion),
		zap.Int("cardRanges", len(oreq.CardRangeData)),
	)

	// Save inbound OReq message (no transaction ID — operational message)
	oreqRaw, _ := json.Marshal(oreq)
	if h.msgRepo != nil {
		if err := h.msgRepo.Save(c.Request.Context(), &entity.Message{
			ACSTransactionID: nil,
			MessageType:      "OReq",
			Direction:        "inbound",
			Content:          oreqRaw,
			CreatedAt:        now,
		}); err != nil {
			h.log.Warn("failed to save OReq message", zap.Error(err))
		}
	}

	// Build ORes response
	ores := dto.OResDTO{
		MessageType:        "ORes",
		MessageVersion:     oreq.MessageVersion,
		ACSOperatorID:      h.cfg.ACS.OperatorID,
		ACSReferenceNumber: h.cfg.ACS.ReferenceNumber,
	}

	// Save outbound ORes message
	oresRaw, _ := json.Marshal(ores)
	if h.msgRepo != nil {
		if err := h.msgRepo.Save(c.Request.Context(), &entity.Message{
			ACSTransactionID: nil,
			MessageType:      "ORes",
			Direction:        "outbound",
			Content:          oresRaw,
			CreatedAt:        now,
		}); err != nil {
			h.log.Warn("failed to save ORes message", zap.Error(err))
		}
	}

	h.log.Info("ORes sent to DS",
		zap.String("acsOperatorID", ores.ACSOperatorID),
		zap.String("messageVersion", ores.MessageVersion),
	)

	c.JSON(http.StatusOK, ores)
}
