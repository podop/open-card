package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/ds-server-emulator/internal/store"
	"go.uber.org/zap"
)

// DSHandler handles incoming 3DS protocol messages from the ACS
// and sends operational messages (OReq) to the ACS.
type DSHandler struct {
	store  *store.InMemoryStore
	acsURL string // base URL of the ACS (for sending OReq)
	log    *zap.Logger
}

func New(s *store.InMemoryStore, acsURL string, log *zap.Logger) *DSHandler {
	return &DSHandler{store: s, acsURL: acsURL, log: log}
}

// HandleRReq handles POST /ds/v2/rreq — receives RReq from ACS, responds with RRes.
// EMV 3DS spec: Directory Server receives Results Request from ACS and acknowledges.
func (h *DSHandler) HandleRReq(c *gin.Context) {
	var rreq map[string]interface{}
	if err := c.ShouldBindJSON(&rreq); err != nil {
		h.log.Warn("invalid RReq payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid RReq JSON"})
		return
	}

	// Validate required fields
	msgType, _ := rreq["messageType"].(string)
	if msgType != "RReq" {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("expected messageType RReq, got %q", msgType)})
		return
	}

	acsTransID, _ := rreq["acsTransID"].(string)
	dsTransID, _ := rreq["dsTransID"].(string)
	serverTransID, _ := rreq["threeDSServerTransID"].(string)
	msgVersion, _ := rreq["messageVersion"].(string)
	transStatus, _ := rreq["transStatus"].(string)

	if acsTransID == "" || dsTransID == "" || serverTransID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required transaction IDs"})
		return
	}

	// Validate UUIDs
	if _, err := uuid.Parse(acsTransID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}
	if _, err := uuid.Parse(dsTransID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dsTransID"})
		return
	}

	h.log.Info("RReq received",
		zap.String("acsTransID", acsTransID),
		zap.String("dsTransID", dsTransID),
		zap.String("transStatus", transStatus),
		zap.String("messageVersion", msgVersion),
	)

	// Build RRes per EMV 3DS spec
	rres := map[string]interface{}{
		"messageType":          "RRes",
		"messageVersion":       msgVersion,
		"acsTransID":           acsTransID,
		"dsTransID":            dsTransID,
		"threeDSServerTransID": serverTransID,
		"resultsStatus":        "01", // 01 = acknowledged
	}

	// Store the record
	rec := store.RReqRecord{
		ACSTransID:  acsTransID,
		DSTransID:   dsTransID,
		ServerTxID:  serverTransID,
		TransStatus: transStatus,
		MsgVersion:  msgVersion,
		RReq:        rreq,
		RRes:        rres,
		ReceivedAt:  time.Now().UTC(),
	}
	h.store.Add(rec)

	h.log.Info("RRes sent",
		zap.String("acsTransID", acsTransID),
		zap.String("resultsStatus", "01"),
	)

	c.JSON(http.StatusOK, rres)
}

// SendOReqToACS handles POST /api/send-oreq — DS sends OReq to ACS and returns ORes.
// Per EMV 3DS spec, the DS originates OReq and the ACS responds with ORes.
func (h *DSHandler) SendOReqToACS(c *gin.Context) {
	if h.acsURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ACS URL not configured"})
		return
	}

	var req struct {
		Availability    string `json:"availability"`    // "01"=available, "02"=temp unavail, "03"=perm unavail
		MessageVersion  string `json:"messageVersion"`  // e.g. "2.3.1"
		ACSOperatorID   string `json:"acsOperatorID"`
		ACSRefNumber    string `json:"acsReferenceNumber"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Availability == "" {
		req.Availability = "01"
	}
	if req.MessageVersion == "" {
		req.MessageVersion = "2.3.1"
	}
	if req.ACSOperatorID == "" {
		req.ACSOperatorID = "ds-emulator"
	}
	if req.ACSRefNumber == "" {
		req.ACSRefNumber = "3DS_LOA_ACS_DEV_020200_00000"
	}

	oreq := map[string]interface{}{
		"messageType":        "OReq",
		"messageVersion":     req.MessageVersion,
		"acsOperatorID":      req.ACSOperatorID,
		"acsReferenceNumber": req.ACSRefNumber,
		"acsAvailability":    req.Availability,
	}

	oreqRaw, _ := json.Marshal(oreq)
	url := h.acsURL + "/v1/oreq"

	h.log.Info("sending OReq to ACS",
		zap.String("acsURL", url),
		zap.String("availability", req.Availability),
	)

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, bytes.NewReader(oreqRaw))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("build request: %v", err)})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		h.log.Error("OReq delivery to ACS failed", zap.Error(err))
		// Store failed attempt
		rec := store.OReqRecord{
			ACSOperatorID: req.ACSOperatorID,
			ACSRefNumber:  req.ACSRefNumber,
			Availability:  req.Availability,
			MsgVersion:    req.MessageVersion,
			OReq:          oreq,
			ORes:          map[string]interface{}{"error": err.Error()},
			ReceivedAt:    time.Now().UTC(),
		}
		h.store.AddOReq(rec)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("ACS unreachable: %v", err)})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var ores map[string]interface{}
	if err := json.Unmarshal(body, &ores); err != nil {
		ores = map[string]interface{}{"raw": string(body)}
	}

	rec := store.OReqRecord{
		ACSOperatorID: req.ACSOperatorID,
		ACSRefNumber:  req.ACSRefNumber,
		Availability:  req.Availability,
		MsgVersion:    req.MessageVersion,
		OReq:          oreq,
		ORes:          ores,
		ReceivedAt:    time.Now().UTC(),
	}
	h.store.AddOReq(rec)

	if resp.StatusCode != http.StatusOK {
		h.log.Warn("ACS returned non-200 for OReq", zap.Int("status", resp.StatusCode))
		c.JSON(resp.StatusCode, gin.H{
			"error": fmt.Sprintf("ACS returned status %d", resp.StatusCode),
			"ores":  ores,
		})
		return
	}

	h.log.Info("ORes received from ACS",
		zap.String("acsOperatorID", req.ACSOperatorID),
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"oreq":    oreq,
		"ores":    ores,
	})
}

// ── API endpoints for DS dashboard ──────────────────────────────────────

// ListRecords handles GET /api/records — paginated list of RReq/RRes pairs.
func (h *DSHandler) ListRecords(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	records, total := h.store.List(page, perPage)
	c.JSON(http.StatusOK, gin.H{
		"data":     records,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// ListOReqRecords handles GET /api/oreq-records — paginated list of OReq/ORes pairs.
func (h *DSHandler) ListOReqRecords(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	records, total := h.store.ListOReq(page, perPage)
	c.JSON(http.StatusOK, gin.H{
		"data":     records,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// GetStats handles GET /api/stats — aggregate stats.
func (h *DSHandler) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.store.Stats())
}

// ListCommunication handles GET /api/communication-log — unified paginated message log.
func (h *DSHandler) ListCommunication(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	entries, total := h.store.ListCommunication(page, perPage)
	c.JSON(http.StatusOK, gin.H{
		"data":     entries,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// GetStatus handles GET /api/status — DS emulator status info.
func (h *DSHandler) GetStatus(c *gin.Context) {
	stats := h.store.Stats()
	c.JSON(http.StatusOK, gin.H{
		"active":   true,
		"acsURL":   h.acsURL,
		"rreqTotal": stats["total"],
		"oreqTotal": stats["oreq_total"],
	})
}

// HealthCheck handles GET /health
func (h *DSHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "ds-server-emulator"})
}