package handler

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/acs-server/internal/adapter/repository/postgres"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/repository"
	"github.com/your-org/acs-server/internal/domain/service"
	"github.com/your-org/acs-server/internal/infrastructure/config"
	"github.com/your-org/acs-server/internal/usecase"
	"go.uber.org/zap"
)

// CertType identifies the purpose of a network certificate per EMV 3DS spec.
const (
	CertTypeServer  = "server"  // TLS server certificate for the ACS
	CertTypeClient  = "client"  // Mutual-TLS client certificate for DS communication
	CertTypeSigning = "signing" // Message signing certificate (ARes, RReq)
)

// ValidCertTypes is the set of allowed certificate types.
var ValidCertTypes = map[string]string{
	CertTypeServer:  "Server",
	CertTypeClient:  "Client",
	CertTypeSigning: "Signing",
}

// NetworkCertInfo holds parsed metadata about a network-specific certificate.
type NetworkCertInfo struct {
	Network      string    `json:"network"`
	CertType     string    `json:"certType"`
	Subject      string    `json:"subject"`
	Issuer       string    `json:"issuer"`
	NotBefore    time.Time `json:"notBefore"`
	NotAfter     time.Time `json:"notAfter"`
	SerialNumber string    `json:"serialNumber"`
	Fingerprint  string    `json:"fingerprint"`
	PEMData      string    `json:"-"`
	KeyPEM       string    `json:"-"`
	HasKey       bool      `json:"hasKey"`
	UploadedAt   time.Time `json:"uploadedAt"`
}

// ApiHandler serves the monitoring and admin REST API consumed by the UI.
type ApiHandler struct {
	txRepo          repository.TransactionRepository
	auditLogRepo    *postgres.AuditLogRepo
	rangeRepo       repository.CardRangeRepository
	msgRepo         *postgres.MessageRepo
	networkRuleRepo *postgres.NetworkRuleRepo
	networkRepo     *postgres.NetworkRepo
	mfaProviderRepo *postgres.MFAProviderRepo
	cfg             *config.Config
	cfgMu           sync.RWMutex
	policyStore     usecase.NetworkPolicyStore
	networkCerts    map[string]map[string]*NetworkCertInfo // network -> certType -> info
	certsMu         sync.RWMutex
	otpResolver     *service.SimulatedOTPResolver
	oobResolver     *service.SimulatedOOBResolver
	log             *zap.Logger
}

// NewApiHandler creates a new ApiHandler.
func NewApiHandler(
	txRepo repository.TransactionRepository,
	auditLogRepo *postgres.AuditLogRepo,
	rangeRepo repository.CardRangeRepository,
	msgRepo *postgres.MessageRepo,
	networkRuleRepo *postgres.NetworkRuleRepo,
	networkRepo *postgres.NetworkRepo,
	mfaProviderRepo *postgres.MFAProviderRepo,
	cfg *config.Config,
	policyStore usecase.NetworkPolicyStore,
	otpResolver *service.SimulatedOTPResolver,
	oobResolver *service.SimulatedOOBResolver,
	log *zap.Logger,
) *ApiHandler {
	return &ApiHandler{
		txRepo:          txRepo,
		auditLogRepo:    auditLogRepo,
		rangeRepo:       rangeRepo,
		msgRepo:         msgRepo,
		networkRuleRepo: networkRuleRepo,
		networkRepo:     networkRepo,
		mfaProviderRepo: mfaProviderRepo,
		cfg:             cfg,
		policyStore:     policyStore,
		networkCerts:    make(map[string]map[string]*NetworkCertInfo),
		otpResolver:     otpResolver,
		oobResolver:     oobResolver,
		log:             log,
	}
}

// ── Dashboard ──────────────────────────────────────────────────────────────

// DashboardStats holds summary counters for the dashboard page.
type DashboardStats struct {
	Total         int64 `json:"total"`
	Frictionless  int64 `json:"frictionless"`
	Challenged    int64 `json:"challenged"`
	Authenticated int64 `json:"authenticated"`
	Failed        int64 `json:"failed"`
	Unavailable   int64 `json:"unavailable"`
}

// GetDashboard handles GET /api/dashboard
func (h *ApiHandler) GetDashboard(c *gin.Context) {
	ctx := c.Request.Context()

	stats, err := h.dashboardStats(ctx)
	if err != nil {
		h.log.Error("dashboard stats query failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	networks, err := h.networkBreakdown(ctx)
	if err != nil {
		h.log.Error("network breakdown query failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	activity, err := h.hourlyActivity(ctx)
	if err != nil {
		h.log.Error("hourly activity query failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Recent 10 transactions
	recent, _, err := h.txRepo.List(ctx, repository.TransactionFilter{Page: 1, PerPage: 10})
	if err != nil {
		h.log.Error("recent transactions query failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats":    stats,
		"networks": networks,
		"activity": activity,
		"recent":   txListItems(recent),
	})
}

type statusCount struct {
	Status string `db:"trans_status" json:"status"`
	Count  int64  `db:"count"        json:"count"`
}

func (h *ApiHandler) dashboardStats(ctx context.Context) (*DashboardStats, error) {
	// Cast txRepo to concrete type to access DB — use a raw interface assertion.
	type dbGetter interface{ DB() interface{ QueryRowContext(context.Context, string, ...interface{}) interface{ Scan(...interface{}) error } } }

	// Use the List approach for each status to avoid exposing DB directly.
	var s DashboardStats
	for _, status := range []string{"Y", "C", "N", "U", "A", "R", "D", "I"} {
		_, cnt, err := h.txRepo.List(ctx, repository.TransactionFilter{Status: status, Page: 1, PerPage: 1})
		if err != nil {
			return nil, err
		}
		s.Total += cnt
		switch status {
		case "Y":
			// frictionless = authenticated without challenge
			// We'll treat audit events instead — for now count Y trans
			s.Frictionless = cnt
		case "C":
			s.Challenged = cnt
		case "N":
			s.Failed = cnt
		case "U":
			s.Unavailable = cnt
		}
	}
	// Authenticated = Y + A (attempt processing)
	_, a, _ := h.txRepo.List(ctx, repository.TransactionFilter{Status: "A", Page: 1, PerPage: 1})
	s.Authenticated = s.Frictionless + a
	return &s, nil
}

type networkBreakdownItem struct {
	Network string `json:"network"`
	Count   int64  `json:"count"`
}

func (h *ApiHandler) networkBreakdown(ctx context.Context) ([]networkBreakdownItem, error) {
	networks, err := h.networkRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	result := []networkBreakdownItem{}
	for _, net := range networks {
		_, cnt, err := h.txRepo.List(ctx, repository.TransactionFilter{Network: net.Name, Page: 1, PerPage: 1})
		if err != nil {
			return nil, err
		}
		if cnt > 0 {
			result = append(result, networkBreakdownItem{Network: net.Name, Count: cnt})
		}
	}
	return result, nil
}

type activityPoint struct {
	Hour  string `json:"hour"`
	Total int64  `json:"total"`
	Auth  int64  `json:"auth"`
}

func (h *ApiHandler) hourlyActivity(ctx context.Context) ([]activityPoint, error) {
	// Use audit_log for last 24h activity (has event_type which we use to classify)
	rows, _, err := h.auditLogRepo.List(ctx, repository.AuditLogFilter{Page: 1, PerPage: 200})
	if err != nil {
		return nil, err
	}
	// Bucket by hour
	buckets := map[string]*activityPoint{}
	for _, r := range rows {
		h := r.CreatedAt.UTC().Format("2006-01-02T15:00:00Z")
		if buckets[h] == nil {
			buckets[h] = &activityPoint{Hour: h}
		}
		buckets[h].Total++
		if r.EventType == "auth_frictionless" || r.EventType == "challenge_success" {
			buckets[h].Auth++
		}
	}
	pts := make([]activityPoint, 0, len(buckets))
	for _, v := range buckets {
		pts = append(pts, *v)
	}
	return pts, nil
}

// ── Transactions ───────────────────────────────────────────────────────────

type txListItem struct {
	ACSTransID      string  `json:"acsTransID"`
	DSTransID       string  `json:"dsTransID"`
	ServerTransID   string  `json:"serverTransID"`
	Network         string  `json:"network"`
	Channel         string  `json:"deviceChannel"`
	TransStatus     string  `json:"transStatus"`
	MerchantName    string  `json:"merchantName"`
	MerchantCountry string  `json:"merchantCountry"`
	Amount          *string `json:"purchaseAmount"`
	Currency        *string `json:"purchaseCurrency"`
	Attempts        int     `json:"challengeAttempts"`
	AuthValue       *string `json:"authValue"`
	ECI             *string `json:"eci"`
	CreatedAt       string  `json:"createdAt"`
	CompletedAt     *string `json:"completedAt"`
}

func txListItems(rows []*entity.Transaction) []txListItem {
	out := make([]txListItem, 0, len(rows))
	for _, r := range rows {
		item := txListItem{
			ACSTransID:      r.ACSTransactionID.String(),
			DSTransID:       r.DSTransactionID.String(),
			ServerTransID:   r.ServerTransactionID.String(),
			Network:         string(r.Network),
			Channel:         string(r.DeviceChannel),
			TransStatus:     string(r.TransStatus),
			MerchantName:    r.MerchantName,
			MerchantCountry: r.MerchantCountry,
			Amount:          r.PurchaseAmount,
			Currency:        r.PurchaseCurrency,
			Attempts:        r.ChallengeAttempts,
			AuthValue:       r.AuthValue,
			ECI:             r.ECI,
			CreatedAt:       r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if r.CompletedAt != nil {
			s := r.CompletedAt.UTC().Format("2006-01-02T15:04:05Z")
			item.CompletedAt = &s
		}
		out = append(out, item)
	}
	return out
}

// GetTransactions handles GET /api/transactions
func (h *ApiHandler) GetTransactions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	rows, total, err := h.txRepo.List(c.Request.Context(), repository.TransactionFilter{
		Status:  c.Query("status"),
		Network: c.Query("network"),
		Channel: c.Query("channel"),
		Search:  c.Query("search"),
		Page:    page,
		PerPage: perPage,
	})
	if err != nil {
		h.log.Error("transactions list failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":     txListItems(rows),
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// GetTransaction handles GET /api/transactions/:acsTransID — transaction drilldown.
func (h *ApiHandler) GetTransaction(c *gin.Context) {
	acsTransID, err := uuid.Parse(c.Param("acsTransID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acsTransID"})
		return
	}

	tx, err := h.txRepo.FindByACSTransactionID(c.Request.Context(), acsTransID)
	if err != nil || tx == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	messages, _ := h.msgRepo.ListByTransaction(c.Request.Context(), acsTransID)
	auditLogs, _ := h.auditLogRepo.ListByTransaction(c.Request.Context(), acsTransID)

	type msgItem struct {
		ID          int64       `json:"id"`
		MessageType string      `json:"messageType"`
		Direction   string      `json:"direction"`
		Content     interface{} `json:"content"`
		CreatedAt   string      `json:"createdAt"`
	}
	msgItems := make([]msgItem, 0, len(messages))
	for _, m := range messages {
		msgItems = append(msgItems, msgItem{
			ID:          m.ID,
			MessageType: m.MessageType,
			Direction:   m.Direction,
			Content:     m.Content,
			CreatedAt:   m.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	type auditItem struct {
		ID          int64       `json:"id"`
		EventType   string      `json:"eventType"`
		TransStatus string      `json:"transStatus"`
		Detail      interface{} `json:"detail"`
		CreatedAt   string      `json:"createdAt"`
	}
	auditItems := make([]auditItem, 0, len(auditLogs))
	for _, a := range auditLogs {
		auditItems = append(auditItems, auditItem{
			ID:          a.ID,
			EventType:   a.EventType,
			TransStatus: a.TransStatus,
			Detail:      a.Detail,
			CreatedAt:   a.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"transaction": txListItems([]*entity.Transaction{tx})[0],
		"messages":    msgItems,
		"auditLogs":   auditItems,
	})
}

// ── Audit Logs ─────────────────────────────────────────────────────────────

// GetAuditLogs handles GET /api/audit-logs
func (h *ApiHandler) GetAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	rows, total, err := h.auditLogRepo.List(c.Request.Context(), repository.AuditLogFilter{
		EventType: c.Query("event_type"),
		Search:    c.Query("search"),
		Page:      page,
		PerPage:   perPage,
	})
	if err != nil {
		h.log.Error("audit logs list failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type auditItem struct {
		ID              int64  `json:"id"`
		ACSTransID      string `json:"acsTransID"`
		DSTransID       string `json:"dsTransID"`
		EventType       string `json:"eventType"`
		TransStatus     string `json:"transStatus"`
		CardBIN         string `json:"cardBIN"`
		MerchantCountry string `json:"merchantCountry"`
		DeviceChannel   string `json:"deviceChannel"`
		SpecVersion     string `json:"specVersion"`
		Detail          interface{} `json:"detail"`
		CreatedAt       string `json:"createdAt"`
	}
	items := make([]auditItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, auditItem{
			ID:              r.ID,
			ACSTransID:      r.ACSTransactionID.String(),
			DSTransID:       r.DSTransactionID.String(),
			EventType:       r.EventType,
			TransStatus:     r.TransStatus,
			CardBIN:         r.CardBIN,
			MerchantCountry: r.MerchantCountry,
			DeviceChannel:   r.DeviceChannel,
			SpecVersion:     r.SpecVersion,
			Detail:          r.Detail,
			CreatedAt:       r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"data":     items,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// ── Card Ranges ────────────────────────────────────────────────────────────

type cardRangeRequest struct {
	StartRange  string `json:"startRange"  binding:"required"`
	EndRange    string `json:"endRange"    binding:"required"`
	Network     string `json:"network"     binding:"required"`
	MinProtocol string `json:"minProtocol"`
	MaxProtocol string `json:"maxProtocol"`
}

// GetCardRanges handles GET /api/card-ranges
func (h *ApiHandler) GetCardRanges(c *gin.Context) {
	// Use a wildcard PAN lookup trick — list by fetching all active ranges via sequential PANs.
	// Since there's no List method on CardRangeRepository, we add it here via a direct cast.
	// For now return a placeholder — see PostCardRange for the write path.
	type rangeItem struct {
		ID          int64  `json:"id"`
		StartRange  string `json:"startRange"`
		EndRange    string `json:"endRange"`
		Network     string `json:"network"`
		MinProtocol string `json:"minProtocol"`
		MaxProtocol string `json:"maxProtocol"`
		Active      bool   `json:"active"`
		CreatedAt   string `json:"createdAt"`
	}

	// Assert to concrete type to access DB.
	type lister interface {
		ListAll(ctx context.Context) ([]*entity.CardRange, error)
	}
	if lr, ok := h.rangeRepo.(lister); ok {
		ranges, err := lr.ListAll(c.Request.Context())
		if err != nil {
			h.log.Error("list card ranges failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		items := make([]rangeItem, 0, len(ranges))
		for _, r := range ranges {
			items = append(items, rangeItem{
				ID:          r.ID,
				StartRange:  r.StartRange,
				EndRange:    r.EndRange,
				Network:     string(r.Network),
				MinProtocol: string(r.MinProtocol),
				MaxProtocol: string(r.MaxProtocol),
				Active:      r.Active,
				CreatedAt:   r.CreatedAt.Format("2006-01-02T15:04:05Z"),
			})
		}
		c.JSON(http.StatusOK, gin.H{"data": items})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": []rangeItem{}})
}

// PostCardRange handles POST /api/card-ranges
func (h *ApiHandler) PostCardRange(c *gin.Context) {
	var req cardRangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	minProto := entity.SpecVersion("2.2.0")
	maxProto := entity.SpecVersion("2.3.1")
	if req.MinProtocol != "" {
		minProto = entity.SpecVersion(req.MinProtocol)
	}
	if req.MaxProtocol != "" {
		maxProto = entity.SpecVersion(req.MaxProtocol)
	}
	cr := &entity.CardRange{
		StartRange:  req.StartRange,
		EndRange:    req.EndRange,
		Network:     entity.Network(req.Network),
		MinProtocol: minProto,
		MaxProtocol: maxProto,
		ActionInd:   "A",
		Active:      true,
	}
	if err := h.rangeRepo.Upsert(c.Request.Context(), cr); err != nil {
		h.log.Error("upsert card range failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "card range saved"})
}

// DeleteCardRange handles DELETE /api/card-ranges
func (h *ApiHandler) DeleteCardRange(c *gin.Context) {
	start := c.Query("start")
	end := c.Query("end")
	if start == "" || end == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start and end query params required"})
		return
	}
	if err := h.rangeRepo.DeleteByRange(c.Request.Context(), start, end); err != nil {
		h.log.Error("delete card range failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "card range deleted"})
}

// ── Config ─────────────────────────────────────────────────────────────────

// maskKey returns the first 8 chars of a hex key + "..." or "(not set)" if empty.
func maskKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 8 {
		return key + "..."
	}
	return key[:8] + "..."
}

// maskDSN hides the password portion of a Postgres DSN.
func maskDSN(dsn string) string {
	// postgres://user:password@host:5432/db?sslmode=disable → postgres://user:***@host:5432/db?...
	if idx := strings.Index(dsn, "://"); idx != -1 {
		rest := dsn[idx+3:]
		if atIdx := strings.Index(rest, "@"); atIdx != -1 {
			userPart := rest[:atIdx]
			if colonIdx := strings.Index(userPart, ":"); colonIdx != -1 {
				return dsn[:idx+3] + userPart[:colonIdx] + ":***@" + rest[atIdx+1:]
			}
		}
	}
	return dsn
}

// GetConfig handles GET /api/config — returns all admin-relevant settings.
func (h *ApiHandler) GetConfig(c *gin.Context) {
	type networkPolicy struct {
		Network           string `json:"network"`
		AllowFrictionless bool   `json:"allowFrictionless"`
		HasCustomKey      bool   `json:"hasCustomKey"`
		HasCertificate    bool   `json:"hasCertificate"`
		CertTypes         []string `json:"certTypes"`
	}

	h.cfgMu.RLock()
	policies := make([]networkPolicy, 0, len(h.cfg.Networks))
	for name, nc := range h.cfg.Networks {
		p, _ := h.policyStore.Get(entity.Network(name))
		h.certsMu.RLock()
		certs := h.networkCerts[name]
		var certTypes []string
		for ct := range certs {
			certTypes = append(certTypes, ct)
		}
		h.certsMu.RUnlock()
		policies = append(policies, networkPolicy{
			Network:           name,
			AllowFrictionless: p.AllowFrictionless,
			HasCustomKey:      nc.MasterKeyHex != "",
			HasCertificate:    len(certTypes) > 0,
			CertTypes:         certTypes,
		})
	}

	resp := gin.H{
		"server": gin.H{
			"host":        h.cfg.Server.Host,
			"port":        h.cfg.Server.Port,
			"tlsCertFile": h.cfg.Server.TLSCertFile,
			"tlsKeyFile":  h.cfg.Server.TLSKeyFile,
		},
		"database": gin.H{
			"dsn": maskDSN(h.cfg.Database.DSN),
		},
		"acs": gin.H{
			"baseURL":           h.cfg.ACS.BaseURL,
			"referenceNumber":   h.cfg.ACS.ReferenceNumber,
			"operatorID":        h.cfg.ACS.OperatorID,
			"supportedVersions": h.cfg.ACS.SupportedVersions,
			"sdkPrivateKeyFile": h.cfg.ACS.SDKPrivateKeyFile,
			"dsURL":             h.cfg.ACS.DSURL,
		},
		"crypto": gin.H{
			"masterKeyHex":       maskKey(h.cfg.Crypto.MasterKeyHex),
			"panEncryptionKeyHex": maskKey(h.cfg.Crypto.PANEncryptionKeyHex),
		},
		"log": gin.H{
			"level":  h.cfg.Log.Level,
			"format": h.cfg.Log.Format,
		},
		"networkPolicies": policies,
	}
	h.cfgMu.RUnlock()

	c.JSON(http.StatusOK, resp)
}

// PutConfig handles PUT /api/config — update core ACS settings at runtime.
func (h *ApiHandler) PutConfig(c *gin.Context) {
	var req struct {
		ACS *struct {
			BaseURL           string   `json:"baseURL"`
			ReferenceNumber   string   `json:"referenceNumber"`
			OperatorID        string   `json:"operatorID"`
			SupportedVersions []string `json:"supportedVersions"`
			DSURL             string   `json:"dsURL"`
		} `json:"acs"`
		Log *struct {
			Level  string `json:"level"`
			Format string `json:"format"`
		} `json:"log"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.cfgMu.Lock()
	if req.ACS != nil {
		if req.ACS.BaseURL != "" {
			h.cfg.ACS.BaseURL = req.ACS.BaseURL
		}
		if req.ACS.ReferenceNumber != "" {
			h.cfg.ACS.ReferenceNumber = req.ACS.ReferenceNumber
		}
		if req.ACS.OperatorID != "" {
			h.cfg.ACS.OperatorID = req.ACS.OperatorID
		}
		if len(req.ACS.SupportedVersions) > 0 {
			h.cfg.ACS.SupportedVersions = req.ACS.SupportedVersions
		}
		// dsURL can be set to empty to disable
		h.cfg.ACS.DSURL = req.ACS.DSURL
	}
	if req.Log != nil {
		if req.Log.Level != "" {
			h.cfg.Log.Level = req.Log.Level
		}
		if req.Log.Format != "" {
			h.cfg.Log.Format = req.Log.Format
		}
	}
	h.cfgMu.Unlock()

	h.log.Info("config updated via admin API")
	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}

// PatchNetworkPolicy handles PATCH /api/config/network/:network (legacy toggle)
func (h *ApiHandler) PatchNetworkPolicy(c *gin.Context) {
	name := c.Param("network")
	var req struct {
		AllowFrictionless bool `json:"allowFrictionless"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.cfgMu.RLock()
	_, ok := h.cfg.Networks[name]
	h.cfgMu.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "network not found"})
		return
	}

	h.policyStore.Set(entity.Network(name), usecase.NetworkPolicy{
		AllowFrictionless: req.AllowFrictionless,
	})

	h.log.Info("network policy updated", zap.String("network", name), zap.Bool("allowFrictionless", req.AllowFrictionless))
	c.JSON(http.StatusOK, gin.H{"network": name, "allowFrictionless": req.AllowFrictionless})
}

// PutNetworkConfig handles PUT /api/config/network/:network — update full network config.
func (h *ApiHandler) PutNetworkConfig(c *gin.Context) {
	name := c.Param("network")
	var req struct {
		AllowFrictionless bool   `json:"allowFrictionless"`
		MasterKeyHex      string `json:"masterKeyHex"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate master key hex if provided.
	if req.MasterKeyHex != "" {
		if len(req.MasterKeyHex) != 64 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "masterKeyHex must be exactly 64 hex characters"})
			return
		}
		if _, err := hex.DecodeString(req.MasterKeyHex); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "masterKeyHex must be valid hex"})
			return
		}
	}

	h.cfgMu.Lock()
	if h.cfg.Networks == nil {
		h.cfg.Networks = make(map[string]config.NetworkConfig)
	}
	nc := h.cfg.Networks[name]
	nc.AllowFrictionless = req.AllowFrictionless
	if req.MasterKeyHex != "" {
		nc.MasterKeyHex = req.MasterKeyHex
	}
	h.cfg.Networks[name] = nc
	h.cfgMu.Unlock()

	h.policyStore.Set(entity.Network(name), usecase.NetworkPolicy{
		AllowFrictionless: req.AllowFrictionless,
	})

	h.log.Info("network config updated", zap.String("network", name))
	c.JSON(http.StatusOK, gin.H{"network": name, "allowFrictionless": req.AllowFrictionless, "hasCustomKey": req.MasterKeyHex != "" || nc.MasterKeyHex != ""})
}

// ── Network Certificate Management ──────────────────────────────────────────
//
// Each network can have up to 3 certificates (per EMV 3DS spec):
//   - server:  TLS server certificate for the ACS
//   - client:  Mutual-TLS client certificate for DS communication
//   - signing: Message signing certificate (ARes, RReq signatures)
//
// The ?type= query param selects which one. Default is "server" for backward compat.

func certTypeFromQuery(c *gin.Context) (string, bool) {
	ct := c.DefaultQuery("type", CertTypeServer)
	if _, ok := ValidCertTypes[ct]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid certificate type %q — must be server, client, or signing", ct)})
		return "", false
	}
	return ct, true
}

// GetNetworkCertificate handles GET /api/config/network/:network/certificate?type=server|client|signing
func (h *ApiHandler) GetNetworkCertificate(c *gin.Context) {
	name := c.Param("network")
	ct, ok := certTypeFromQuery(c)
	if !ok {
		return
	}

	h.certsMu.RLock()
	var cert *NetworkCertInfo
	if m, exists := h.networkCerts[name]; exists {
		cert = m[ct]
	}
	h.certsMu.RUnlock()

	if cert == nil {
		c.JSON(http.StatusOK, gin.H{"certificate": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"certificate": cert})
}

// GetNetworkCertificates handles GET /api/config/network/:network/certificates (all types)
func (h *ApiHandler) GetNetworkCertificates(c *gin.Context) {
	name := c.Param("network")

	h.certsMu.RLock()
	m := h.networkCerts[name]
	result := make(map[string]*NetworkCertInfo, 3)
	for _, ct := range []string{CertTypeServer, CertTypeClient, CertTypeSigning} {
		if m != nil {
			result[ct] = m[ct] // may be nil
		}
	}
	h.certsMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{"certificates": result})
}

// PostNetworkCertificate handles POST /api/config/network/:network/certificate?type=server|client|signing
func (h *ApiHandler) PostNetworkCertificate(c *gin.Context) {
	name := c.Param("network")
	ct, ok := certTypeFromQuery(c)
	if !ok {
		return
	}

	var req struct {
		PEMData string `json:"pemData" binding:"required"`
		KeyPEM  string `json:"keyPem"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse certificate PEM
	block, _ := pem.Decode([]byte(req.PEMData))
	if block == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PEM certificate data"})
		return
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid certificate: %v", err)})
		return
	}

	// Validate private key PEM if provided
	hasKey := false
	if req.KeyPEM != "" {
		keyBlock, _ := pem.Decode([]byte(req.KeyPEM))
		if keyBlock == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PEM private key data"})
			return
		}
		// Attempt to parse as PKCS8, EC, or PKCS1 to validate
		if _, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes); err != nil {
			if _, err := x509.ParseECPrivateKey(keyBlock.Bytes); err != nil {
				if _, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "cannot parse private key — expected PKCS8, EC, or RSA PKCS1"})
					return
				}
			}
		}
		hasKey = true
	}

	fingerprint := sha256.Sum256(cert.Raw)

	info := &NetworkCertInfo{
		Network:      name,
		CertType:     ct,
		Subject:      cert.Subject.String(),
		Issuer:       cert.Issuer.String(),
		NotBefore:    cert.NotBefore,
		NotAfter:     cert.NotAfter,
		SerialNumber: cert.SerialNumber.Text(16),
		Fingerprint:  hex.EncodeToString(fingerprint[:]),
		PEMData:      req.PEMData,
		KeyPEM:       req.KeyPEM,
		HasKey:       hasKey,
		UploadedAt:   time.Now().UTC(),
	}

	h.certsMu.Lock()
	if h.networkCerts[name] == nil {
		h.networkCerts[name] = make(map[string]*NetworkCertInfo)
	}
	h.networkCerts[name][ct] = info
	h.certsMu.Unlock()

	h.log.Info("network certificate uploaded",
		zap.String("network", name),
		zap.String("certType", ct),
		zap.String("subject", info.Subject),
		zap.Bool("hasKey", hasKey),
		zap.Time("notAfter", info.NotAfter),
	)
	c.JSON(http.StatusCreated, gin.H{"certificate": info})
}

// DeleteNetworkCertificate handles DELETE /api/config/network/:network/certificate?type=server|client|signing
func (h *ApiHandler) DeleteNetworkCertificate(c *gin.Context) {
	name := c.Param("network")
	ct, ok := certTypeFromQuery(c)
	if !ok {
		return
	}

	h.certsMu.Lock()
	if m, exists := h.networkCerts[name]; exists {
		delete(m, ct)
		if len(m) == 0 {
			delete(h.networkCerts, name)
		}
	}
	h.certsMu.Unlock()

	h.log.Info("network certificate removed", zap.String("network", name), zap.String("certType", ct))
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%s certificate removed", ct)})
}

// GetNetworkCert returns the cert info for a given network and type (used by other services).
func (h *ApiHandler) GetNetworkCert(network, certType string) *NetworkCertInfo {
	h.certsMu.RLock()
	defer h.certsMu.RUnlock()
	if m, ok := h.networkCerts[network]; ok {
		return m[certType]
	}
	return nil
}

// ── Network Rules (BIN-level flow control) ──────────────────────────────────

// GetNetworkRules handles GET /api/network-rules
func (h *ApiHandler) GetNetworkRules(c *gin.Context) {
	rules, err := h.networkRuleRepo.ListNetworkRules(c.Request.Context())
	if err != nil {
		h.log.Error("list network rules failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rules})
}

// PutNetworkRule handles PUT /api/network-rules/:network
func (h *ApiHandler) PutNetworkRule(c *gin.Context) {
	network := c.Param("network")
	var req struct {
		AllowFrictionless       bool   `json:"allowFrictionless"`
		AllowOTP                bool   `json:"allowOtp"`
		AllowOOB                bool   `json:"allowOob"`
		DefaultChallengeChannel string `json:"defaultChallengeChannel"`
		SecondFactorMode        string `json:"secondFactorMode"`
		MFAProviderID           *int64 `json:"mfaProviderId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.DefaultChallengeChannel != "otp" && req.DefaultChallengeChannel != "oob" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "defaultChallengeChannel must be 'otp' or 'oob'"})
		return
	}
	if req.SecondFactorMode == "" {
		req.SecondFactorMode = "none"
	}

	rule := &entity.NetworkRule{
		Network:                 entity.Network(network),
		AllowFrictionless:       req.AllowFrictionless,
		AllowOTP:                req.AllowOTP,
		AllowOOB:                req.AllowOOB,
		DefaultChallengeChannel: req.DefaultChallengeChannel,
		SecondFactorMode:        req.SecondFactorMode,
		MFAProviderID:           req.MFAProviderID,
	}
	if err := h.networkRuleRepo.UpsertNetworkRule(c.Request.Context(), rule); err != nil {
		h.log.Error("upsert network rule failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Also sync the legacy policy store
	h.policyStore.Set(entity.Network(network), usecase.NetworkPolicy{
		AllowFrictionless: req.AllowFrictionless,
	})

	h.log.Info("network rule updated", zap.String("network", network))
	c.JSON(http.StatusOK, rule)
}

// ── BIN Exclusions ──────────────────────────────────────────────────────────

// GetBinExclusions handles GET /api/bin-exclusions?network=...
func (h *ApiHandler) GetBinExclusions(c *gin.Context) {
	network := c.Query("network")
	exclusions, err := h.networkRuleRepo.ListBinExclusions(c.Request.Context(), network)
	if err != nil {
		h.log.Error("list bin exclusions failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": exclusions})
}

// PostBinExclusion handles POST /api/bin-exclusions
func (h *ApiHandler) PostBinExclusion(c *gin.Context) {
	var req entity.BinExclusion
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.StartRange == "" || req.EndRange == "" || req.Network == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "network, startRange, endRange are required"})
		return
	}
	if req.DefaultChallengeChannel == "" {
		req.DefaultChallengeChannel = "otp"
	}
	req.Active = true

	if err := h.networkRuleRepo.CreateBinExclusion(c.Request.Context(), &req); err != nil {
		h.log.Error("create bin exclusion failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.Info("bin exclusion created", zap.String("network", string(req.Network)), zap.String("range", req.StartRange+"-"+req.EndRange))
	c.JSON(http.StatusCreated, req)
}

// PutBinExclusion handles PUT /api/bin-exclusions/:id
func (h *ApiHandler) PutBinExclusion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req entity.BinExclusion
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.ID = id
	if req.DefaultChallengeChannel == "" {
		req.DefaultChallengeChannel = "otp"
	}

	if err := h.networkRuleRepo.UpdateBinExclusion(c.Request.Context(), &req); err != nil {
		h.log.Error("update bin exclusion failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.Info("bin exclusion updated", zap.Int64("id", id))
	c.JSON(http.StatusOK, req)
}

// DeleteBinExclusion handles DELETE /api/bin-exclusions/:id
func (h *ApiHandler) DeleteBinExclusion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.networkRuleRepo.DeleteBinExclusion(c.Request.Context(), id); err != nil {
		h.log.Error("delete bin exclusion failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.log.Info("bin exclusion deleted", zap.Int64("id", id))
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ── Networks CRUD ────────────────────────────────────────────────────────────

// GetNetworks handles GET /api/networks — returns all networks from DB.
func (h *ApiHandler) GetNetworks(c *gin.Context) {
	networks, err := h.networkRepo.ListAll(c.Request.Context())
	if err != nil {
		h.log.Error("list networks failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": networks})
}

// PostNetwork handles POST /api/networks — create a new network.
func (h *ApiHandler) PostNetwork(c *gin.Context) {
	var req entity.NetworkEntry
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" || req.DisplayName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and displayName are required"})
		return
	}
	if req.Color == "" {
		req.Color = "#555d70"
	}
	if req.ECIFullAuth == "" {
		req.ECIFullAuth = "05"
	}
	if req.ECIAttempt == "" {
		req.ECIAttempt = "06"
	}
	if req.ECINotAuth == "" {
		req.ECINotAuth = "07"
	}

	if err := h.networkRepo.Create(c.Request.Context(), &req); err != nil {
		h.log.Error("create network failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.Info("network created", zap.String("name", req.Name))
	c.JSON(http.StatusCreated, req)
}

// PutNetwork handles PUT /api/networks/:name — update a network.
func (h *ApiHandler) PutNetwork(c *gin.Context) {
	name := c.Param("name")
	var req entity.NetworkEntry
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Name = name // enforce URL param

	if err := h.networkRepo.Update(c.Request.Context(), &req); err != nil {
		h.log.Error("update network failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Sync legacy policy store active state
	if !req.Active {
		// deactivated network = no frictionless
		h.policyStore.Set(entity.Network(name), usecase.NetworkPolicy{AllowFrictionless: false})
	}

	h.log.Info("network updated", zap.String("name", name))
	c.JSON(http.StatusOK, req)
}

// DeleteNetwork handles DELETE /api/networks/:name — delete a network.
func (h *ApiHandler) DeleteNetwork(c *gin.Context) {
	name := c.Param("name")
	if err := h.networkRepo.Delete(c.Request.Context(), name); err != nil {
		h.log.Error("delete network failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.log.Info("network deleted", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "network deleted"})
}

// ── Channel Resolvers ────────────────────────────────────────────────────────

// GetChannelResolvers handles GET /api/config/channel-resolvers
func (h *ApiHandler) GetChannelResolvers(c *gin.Context) {
	otpAvail := true
	oobAvail := true
	if h.otpResolver != nil {
		otpAvail = h.otpResolver.IsAvailable()
	}
	if h.oobResolver != nil {
		oobAvail = h.oobResolver.IsAvailable()
	}
	c.JSON(http.StatusOK, gin.H{
		"otpAvailable": otpAvail,
		"oobAvailable": oobAvail,
	})
}

// ── MFA Providers ─────────────────────────────────────────────────────────

// GetMFAProviders handles GET /api/mfa-providers
func (h *ApiHandler) GetMFAProviders(c *gin.Context) {
	providers, err := h.mfaProviderRepo.List(c.Request.Context())
	if err != nil {
		h.log.Error("list MFA providers failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": providers})
}

// PostMFAProvider handles POST /api/mfa-providers
func (h *ApiHandler) PostMFAProvider(c *gin.Context) {
	var p entity.MFAProvider
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if p.Name == "" || p.DisplayName == "" || p.Type == "" || p.FactorType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, displayName, type, factorType are required"})
		return
	}
	if p.Config == nil {
		p.Config = []byte("{}")
	}
	if err := h.mfaProviderRepo.Create(c.Request.Context(), &p); err != nil {
		h.log.Error("create MFA provider failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.log.Info("MFA provider created", zap.String("name", p.Name))
	c.JSON(http.StatusCreated, p)
}

// PutMFAProvider handles PUT /api/mfa-providers/:id
func (h *ApiHandler) PutMFAProvider(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var p entity.MFAProvider
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p.ID = id
	if p.Config == nil {
		p.Config = []byte("{}")
	}
	if err := h.mfaProviderRepo.Update(c.Request.Context(), &p); err != nil {
		h.log.Error("update MFA provider failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.log.Info("MFA provider updated", zap.Int64("id", id))
	c.JSON(http.StatusOK, p)
}

// DeleteMFAProvider handles DELETE /api/mfa-providers/:id
func (h *ApiHandler) DeleteMFAProvider(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.mfaProviderRepo.Delete(c.Request.Context(), id); err != nil {
		h.log.Error("delete MFA provider failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.log.Info("MFA provider deleted", zap.Int64("id", id))
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// PatchChannelResolvers handles PATCH /api/config/channel-resolvers
func (h *ApiHandler) PatchChannelResolvers(c *gin.Context) {
	var req struct {
		OTPAvailable *bool `json:"otpAvailable"`
		OOBAvailable *bool `json:"oobAvailable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.OTPAvailable != nil && h.otpResolver != nil {
		h.otpResolver.SetAvailable(*req.OTPAvailable)
		h.log.Info("OTP resolver toggled", zap.Bool("available", *req.OTPAvailable))
	}
	if req.OOBAvailable != nil && h.oobResolver != nil {
		h.oobResolver.SetAvailable(*req.OOBAvailable)
		h.log.Info("OOB resolver toggled", zap.Bool("available", *req.OOBAvailable))
	}
	h.GetChannelResolvers(c)
}

// ── OReq/ORes (DS Operational Messages) ─────────────────────────────────────

// GetOReqStatus handles GET /api/oreq-status — current ACS operational config for OReq.
func (h *ApiHandler) GetOReqStatus(c *gin.Context) {
	h.cfgMu.RLock()
	acsCfg := h.cfg.ACS
	h.cfgMu.RUnlock()

	ranges, _ := h.rangeRepo.ListAll(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"acsOperatorID":      acsCfg.OperatorID,
		"acsReferenceNumber": acsCfg.ReferenceNumber,
		"dsURL":              acsCfg.DSURL,
		"supportedVersions":  acsCfg.SupportedVersions,
		"cardRangeCount":     len(ranges),
	})
}

// GetOReqHistory handles GET /api/oreq-history — paginated OReq/ORes message log.
func (h *ApiHandler) GetOReqHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	msgs, total, err := h.msgRepo.ListOperationalMessages(c.Request.Context(), page, perPage)
	if err != nil {
		h.log.Error("failed to list operational messages", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch OReq history"})
		return
	}

	type msgItem struct {
		ID          int64       `json:"id"`
		MessageType string      `json:"messageType"`
		Direction   string      `json:"direction"`
		Content     interface{} `json:"content"`
		CreatedAt   string      `json:"createdAt"`
	}
	items := make([]msgItem, 0, len(msgs))
	for _, m := range msgs {
		var parsed interface{}
		if err := json.Unmarshal(m.Content, &parsed); err != nil {
			parsed = string(m.Content)
		}
		items = append(items, msgItem{
			ID:          m.ID,
			MessageType: m.MessageType,
			Direction:   m.Direction,
			Content:     parsed,
			CreatedAt:   m.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     items,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

