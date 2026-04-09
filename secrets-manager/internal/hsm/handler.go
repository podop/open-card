package hsm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SecretsProviderLinker allows the HSM handler to create/update/delete security providers.
type SecretsProviderLinker interface {
	CreateLinkedProvider(ctx context.Context, name string, config json.RawMessage, createdBy int) (int, error)
	UpdateLinkedProvider(ctx context.Context, providerID int, name string, config json.RawMessage) error
	DeleteLinkedProvider(ctx context.Context, providerID int) error
}

type Handler struct {
	store        *Store
	gateway      *GatewayManager
	builder      *CommandBuilder
	parser       *ResponseParser
	log          *zap.Logger
	secretsStore SecretsProviderLinker
}

func NewHandler(store *Store, gateway *GatewayManager, log *zap.Logger) *Handler {
	return &Handler{
		store:   store,
		gateway: gateway,
		builder: &CommandBuilder{},
		parser:  &ResponseParser{},
		log:     log,
	}
}

func (h *Handler) SetSecretsLinker(linker SecretsProviderLinker) {
	h.secretsStore = linker
}

// ── Endpoints ───────────────────────────────────────────────────

func (h *Handler) ListEndpoints(c *gin.Context) {
	endpoints, err := h.store.ListEndpoints(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Enrich with pool stats
	var result []EndpointStatus
	for _, ep := range endpoints {
		status, ok := h.gateway.GetEndpointStatus(ep.ID)
		if ok {
			result = append(result, *status)
		} else {
			result = append(result, EndpointStatus{HSMEndpoint: ep})
		}
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetEndpoint(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ep, err := h.store.GetEndpoint(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
		return
	}
	status, ok := h.gateway.GetEndpointStatus(id)
	if ok {
		c.JSON(http.StatusOK, status)
	} else {
		c.JSON(http.StatusOK, EndpointStatus{HSMEndpoint: *ep})
	}
}

func (h *Handler) CreateEndpoint(c *gin.Context) {
	var ep HSMEndpoint
	if err := c.ShouldBindJSON(&ep); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if ep.Config == nil {
		ep.Config = json.RawMessage(`{}`)
	}
	if len(ep.Capabilities) == 0 {
		ep.Capabilities = StringArray(GetDefaultCapabilities(ep.Model))
	}
	ep.CreatedBy = c.GetInt("authUserID")

	id, err := h.store.CreateEndpoint(c.Request.Context(), &ep)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	created, _ := h.store.GetEndpoint(c.Request.Context(), id)
	if created != nil && created.Enabled {
		h.gateway.AddEndpoint(*created)
	}

	// Auto-create linked security provider
	var linkedProviderID int
	if h.secretsStore != nil {
		cfg, _ := json.Marshal(map[string]interface{}{
			"model":              ep.Model,
			"address":            ep.Address,
			"port":               ep.Port,
			"protocol":           ep.Protocol,
			"headerLength":       ep.HeaderLength,
			"hsmEndpointId":      id,
			"firmwareVersion":    ep.FirmwareVersion,
			"hostCommandTimeout": ep.HostCommandTimeout,
			"connectionTimeout":  ep.ConnectionTimeout,
			"maxConnections":     ep.MaxConnections,
			"lmkIdentifier":      ep.LMKIdentifier,
		})
		provID, provErr := h.secretsStore.CreateLinkedProvider(c.Request.Context(), ep.Name, cfg, ep.CreatedBy)
		if provErr != nil {
			h.log.Warn("auto-create security provider failed", zap.Error(provErr))
		} else {
			linkedProviderID = provID
		}
	}

	resp := gin.H{"endpoint": created}
	if linkedProviderID > 0 {
		resp["linkedProviderId"] = linkedProviderID
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) UpdateEndpoint(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var ep HSMEndpoint
	if err := c.ShouldBindJSON(&ep); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if ep.Config == nil {
		ep.Config = json.RawMessage(`{}`)
	}
	if err := h.store.UpdateEndpoint(c.Request.Context(), id, &ep); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Refresh in gateway
	h.gateway.RemoveEndpoint(id)
	updated, _ := h.store.GetEndpoint(c.Request.Context(), id)
	if updated != nil && updated.Enabled {
		h.gateway.AddEndpoint(*updated)
	}

	// Sync linked security provider
	if h.secretsStore != nil && updated != nil {
		var cfg map[string]interface{}
		if err := json.Unmarshal(updated.Config, &cfg); err == nil {
			if provID, ok := cfg["linkedProviderId"].(float64); ok && provID > 0 {
				epCfg, _ := json.Marshal(map[string]interface{}{
					"model": updated.Model, "address": updated.Address,
					"port": updated.Port, "protocol": updated.Protocol,
					"headerLength": updated.HeaderLength, "hsmEndpointId": id,
				})
				h.secretsStore.UpdateLinkedProvider(c.Request.Context(), int(provID), updated.Name, epCfg) //nolint:errcheck
			}
		}
	}

	c.JSON(http.StatusOK, updated)
}

func (h *Handler) DeleteEndpoint(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// Delete linked security provider
	if h.secretsStore != nil {
		ep, _ := h.store.GetEndpoint(c.Request.Context(), id)
		if ep != nil {
			var cfg map[string]interface{}
			if err := json.Unmarshal(ep.Config, &cfg); err == nil {
				if provID, ok := cfg["linkedProviderId"].(float64); ok && provID > 0 {
					h.secretsStore.DeleteLinkedProvider(c.Request.Context(), int(provID)) //nolint:errcheck
				}
			}
		}
	}

	h.gateway.RemoveEndpoint(id)
	if err := h.store.DeleteEndpoint(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) GetEndpointHealth(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ctx := c.Request.Context()

	start := time.Now()
	cmdCode, data, _ := h.builder.BuildDiagnostics()
	resp, cmdErr := h.gateway.SendCommand(ctx, id, cmdCode, data)
	duration := time.Since(start)

	result := gin.H{
		"endpointId": id,
		"durationMs": duration.Milliseconds(),
	}

	if cmdErr != nil {
		result["healthy"] = false
		result["error"] = cmdErr.Error()
		h.logAudit(ctx, c, id, CmdNC, "/endpoints/health", 0, 0, "", int(duration.Milliseconds()), cmdErr.Error())
		c.JSON(http.StatusOK, result)
		return
	}

	_, respCode, respData, parseErr := DecodeResponse(4, resp) // will auto-detect from endpoint
	if parseErr != nil {
		result["healthy"] = false
		result["error"] = parseErr.Error()
	} else {
		result["healthy"] = respCode == RespOK
		result["responseCode"] = respCode
		diagData, _ := h.parser.ParseDiagnostics(respCode, respData)
		if diagData != nil {
			result["diagnostics"] = json.RawMessage(diagData)
		}
	}

	respCodeStr := ""
	if parseErr == nil {
		respCodeStr = respCode
	}
	h.logAudit(ctx, c, id, CmdNC, "/endpoints/health", 0, len(resp), respCodeStr, int(duration.Milliseconds()), "")
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ReconnectEndpoint(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.gateway.ReconnectEndpoint(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) DrainEndpoint(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.gateway.DrainEndpoint(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) ResumeEndpoint(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.gateway.ResumeEndpoint(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ── Capabilities ────────────────────────────────────────────────

func (h *Handler) ListCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"capabilities": AllCapabilities,
		"models":       ModelDescriptions,
	})
}

func (h *Handler) GetModelCapabilities(c *gin.Context) {
	model := c.Param("model")
	info, ok := ModelDescriptions[model]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown model"})
		return
	}
	c.JSON(http.StatusOK, info)
}

// ── Endpoint Groups ────────────────────────────────────────────

func (h *Handler) ListEndpointGroups(c *gin.Context) {
	groups, err := h.store.ListEndpointGroups(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list endpoint groups"})
		return
	}
	// Enrich each group with its endpoint IDs
	type groupWithEndpoints struct {
		HSMEndpointGroup
		EndpointIDs []int `json:"endpointIds"`
	}
	var result []groupWithEndpoints
	for _, g := range groups {
		eps, _ := h.store.GetEndpointsByGroup(c.Request.Context(), g.ID)
		ids := make([]int, 0, len(eps))
		for _, ep := range eps {
			ids = append(ids, ep.ID)
		}
		result = append(result, groupWithEndpoints{HSMEndpointGroup: g, EndpointIDs: ids})
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetEndpointGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	g, err := h.store.GetEndpointGroup(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint group not found"})
		return
	}
	eps, _ := h.store.GetEndpointsByGroup(c.Request.Context(), id)
	ids := make([]int, 0, len(eps))
	for _, ep := range eps {
		ids = append(ids, ep.ID)
	}
	c.JSON(http.StatusOK, gin.H{"group": g, "endpointIds": ids})
}

func (h *Handler) CreateEndpointGroup(c *gin.Context) {
	var g HSMEndpointGroup
	if err := c.ShouldBindJSON(&g); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if g.Strategy == "" {
		g.Strategy = "round-robin"
	}
	g.CreatedBy = c.GetInt("authUserID")

	id, err := h.store.CreateEndpointGroup(c.Request.Context(), &g)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	created, _ := h.store.GetEndpointGroup(c.Request.Context(), id)
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) UpdateEndpointGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var g HSMEndpointGroup
	if err := c.ShouldBindJSON(&g); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.store.UpdateEndpointGroup(c.Request.Context(), id, &g); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	updated, _ := h.store.GetEndpointGroup(c.Request.Context(), id)
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) DeleteEndpointGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.store.DeleteEndpointGroup(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) AssignEndpointToGroup(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}
	var req struct {
		EndpointID int `json:"endpointId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.store.SetEndpointGroup(c.Request.Context(), req.EndpointID, &groupID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Refresh endpoint in gateway so GroupID is updated
	ep, _ := h.store.GetEndpoint(c.Request.Context(), req.EndpointID)
	if ep != nil {
		h.gateway.RemoveEndpoint(req.EndpointID)
		if ep.Enabled {
			h.gateway.AddEndpoint(*ep)
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) RemoveEndpointFromGroup(c *gin.Context) {
	var req struct {
		EndpointID int `json:"endpointId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.store.SetEndpointGroup(c.Request.Context(), req.EndpointID, nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Refresh endpoint in gateway
	ep, _ := h.store.GetEndpoint(c.Request.Context(), req.EndpointID)
	if ep != nil {
		h.gateway.RemoveEndpoint(req.EndpointID)
		if ep.Enabled {
			h.gateway.AddEndpoint(*ep)
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ── Crypto: Keys ────────────────────────────────────────────────

func (h *Handler) GenerateKey(c *gin.Context) {
	var req KeyGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapKeys, func() (string, []byte, error) {
		return h.builder.BuildGenerateKey(req)
	}, "/keys/generate", func(respCode string, data []byte) (json.RawMessage, error) {
		resp, err := h.parser.ParseGenerateKey(respCode, data)
		if err != nil {
			return nil, err
		}
		return json.Marshal(resp)
	})
}

func (h *Handler) ImportKey(c *gin.Context) {
	var req KeyImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapKeys, func() (string, []byte, error) {
		return h.builder.BuildImportKey(req)
	}, "/keys/import", func(respCode string, data []byte) (json.RawMessage, error) {
		return h.parser.ParseKeyImport(respCode, data)
	})
}

func (h *Handler) ExportKey(c *gin.Context) {
	var req KeyExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapKeys, func() (string, []byte, error) {
		return h.builder.BuildExportKey(req)
	}, "/keys/export", func(respCode string, data []byte) (json.RawMessage, error) {
		return h.parser.ParseKeyExport(respCode, data)
	})
}

func (h *Handler) TranslateKey(c *gin.Context) {
	var req KeyTranslateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapKeys, func() (string, []byte, error) {
		return h.builder.BuildTranslateKey(req)
	}, "/keys/translate", func(respCode string, data []byte) (json.RawMessage, error) {
		return h.parser.ParseRawResponse(respCode, data)
	})
}

func (h *Handler) ValidateCheckValue(c *gin.Context) {
	var req CheckValueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapKeys, func() (string, []byte, error) {
		return h.builder.BuildCheckValue(req)
	}, "/keys/validate-check-value", func(respCode string, data []byte) (json.RawMessage, error) {
		return h.parser.ParseCheckValue(respCode, data)
	})
}

func (h *Handler) TR31Import(c *gin.Context) {
	h.stubCommand(c, "/keys/tr31/import", CapKeys)
}

func (h *Handler) TR31Export(c *gin.Context) {
	h.stubCommand(c, "/keys/tr31/export", CapKeys)
}

func (h *Handler) TR34Import(c *gin.Context) {
	h.stubCommand(c, "/keys/tr34/import", CapKeys)
}

func (h *Handler) GenerateRSAKey(c *gin.Context) {
	var req RSAGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapKeys, func() (string, []byte, error) {
		return h.builder.BuildGenerateRSA(req)
	}, "/keys/generate-rsa", func(respCode string, data []byte) (json.RawMessage, error) {
		resp, err := h.parser.ParseGenerateRSA(respCode, data)
		if err != nil {
			return nil, err
		}
		return json.Marshal(resp)
	})
}

// ── Crypto: PIN ─────────────────────────────────────────────────

func (h *Handler) VerifyPIN(c *gin.Context) {
	var req PINVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapPIN, func() (string, []byte, error) {
		return h.builder.BuildVerifyPIN(req)
	}, "/pin/verify", func(respCode string, data []byte) (json.RawMessage, error) {
		return h.parser.ParsePINVerify(respCode, data)
	})
}

func (h *Handler) TranslatePIN(c *gin.Context) {
	var req PINTranslateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapPIN, func() (string, []byte, error) {
		return h.builder.BuildTranslatePIN(req)
	}, "/pin/translate", func(respCode string, data []byte) (json.RawMessage, error) {
		return h.parser.ParsePINTranslate(respCode, data)
	})
}

func (h *Handler) GeneratePINOffset(c *gin.Context) {
	var req PINOffsetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapPIN, func() (string, []byte, error) {
		return h.builder.BuildPINOffset(req)
	}, "/pin/generate-offset", func(respCode string, data []byte) (json.RawMessage, error) {
		return h.parser.ParsePINOffset(respCode, data)
	})
}

func (h *Handler) ChangePIN(c *gin.Context) {
	h.stubCommand(c, "/pin/change", CapPIN)
}

// ── Crypto: EMV ─────────────────────────────────────────────────

func (h *Handler) ValidateARQC(c *gin.Context) {
	var req ARQCValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapEMV, func() (string, []byte, error) {
		return h.builder.BuildValidateARQC(req)
	}, "/emv/arqc/validate", func(respCode string, data []byte) (json.RawMessage, error) {
		return h.parser.ParseARQCValidate(respCode, data)
	})
}

func (h *Handler) GenerateARPC(c *gin.Context) {
	var req ARPCGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapEMV, func() (string, []byte, error) {
		return h.builder.BuildGenerateARPC(req)
	}, "/emv/arpc/generate", func(respCode string, data []byte) (json.RawMessage, error) {
		return h.parser.ParseARQCValidate(respCode, data)
	})
}

func (h *Handler) GenerateEMVMAC(c *gin.Context) {
	var req EMVMACRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapEMV, func() (string, []byte, error) {
		return h.builder.BuildEMVMAC(req)
	}, "/emv/mac/generate", func(respCode string, data []byte) (json.RawMessage, error) {
		return h.parser.ParseEMVMAC(respCode, data)
	})
}

func (h *Handler) PrepareEMVData(c *gin.Context) {
	h.stubCommand(c, "/emv/data/prepare", CapEMV)
}

// ── Crypto: Cards ───────────────────────────────────────────────

func (h *Handler) VerifyCVV(c *gin.Context) {
	var req CVVVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapCards, func() (string, []byte, error) {
		return h.builder.BuildVerifyCVV(req)
	}, "/cards/verify-cvv", func(respCode string, data []byte) (json.RawMessage, error) {
		return h.parser.ParseCVVVerify(respCode, data)
	})
}

func (h *Handler) GenerateCVV(c *gin.Context) {
	var req CVVGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.executeCommand(c, CapCards, func() (string, []byte, error) {
		return h.builder.BuildGenerateCVV(req)
	}, "/cards/generate-cvv", func(respCode string, data []byte) (json.RawMessage, error) {
		return h.parser.ParseCVVGenerate(respCode, data)
	})
}

func (h *Handler) VerifyPVV(c *gin.Context) {
	h.stubCommand(c, "/cards/verify-pvv", CapCards)
}

// ── Crypto: Tokens, P2PE, RKL ───────────────────────────────────

func (h *Handler) ProvisionToken(c *gin.Context) {
	h.stubCommand(c, "/tokens/provision", CapTokens)
}

func (h *Handler) EncryptP2PE(c *gin.Context) {
	h.stubCommand(c, "/p2pe/encrypt", CapP2PE)
}

func (h *Handler) DecryptP2PE(c *gin.Context) {
	h.stubCommand(c, "/p2pe/decrypt", CapP2PE)
}

func (h *Handler) LoadATMKey(c *gin.Context) {
	h.stubCommand(c, "/rkl/atm/load-key", CapRKL)
}

func (h *Handler) LoadPOSKey(c *gin.Context) {
	h.stubCommand(c, "/rkl/pos/load-key", CapRKL)
}

// ── Generate Random ─────────────────────────────────────────

func (h *Handler) GenerateRandom(c *gin.Context) {
	var req struct {
		Length     int `json:"length"`
		EndpointID int `json:"endpointId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Length <= 0 {
		req.Length = 32
	}
	if req.Length > 1024 {
		req.Length = 1024
	}

	ctx := c.Request.Context()
	start := time.Now()

	// Try HSM first
	cmdCode, data, buildErr := h.builder.BuildGenerateRandom(req.Length)
	if buildErr == nil && req.EndpointID > 0 {
		resp, sendErr := h.gateway.SendCommand(ctx, req.EndpointID, cmdCode, data)
		duration := time.Since(start)
		if sendErr == nil {
			_, respCode, respData, parseErr := DecodeResponse(4, resp)
			if parseErr == nil && respCode == RespOK {
				h.logAudit(ctx, c, req.EndpointID, cmdCode, "/admin/generate-random", len(data), len(resp), respCode, int(duration.Milliseconds()), "")
				c.JSON(http.StatusOK, gin.H{
					"random":     hex.EncodeToString(respData[:req.Length]),
					"source":     "hsm",
					"endpointId": req.EndpointID,
					"durationMs": duration.Milliseconds(),
				})
				return
			}
		}
	}

	// Fallback to crypto/rand
	randomBytes := make([]byte, req.Length)
	if _, err := io.ReadFull(rand.Reader, randomBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "random generation failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"random": hex.EncodeToString(randomBytes),
		"source": "internal",
	})
}

// ── Admin ───────────────────────────────────────────────────────

func (h *Handler) GetAuditLog(c *gin.Context) {
	limit := 100
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	var epID *int
	if idStr := c.Query("endpointId"); idStr != "" {
		if id, err := strconv.Atoi(idStr); err == nil {
			epID = &id
		}
	}
	entries, err := h.store.GetAuditLog(c.Request.Context(), limit, epID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entries)
}

func (h *Handler) GetMetrics(c *gin.Context) {
	metrics, err := h.store.GetAllMetricsSummary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"endpoints": h.gateway.ListEndpointStatuses(),
		"metrics":   metrics,
	})
}

func (h *Handler) TestCommand(c *gin.Context) {
	var req struct {
		EndpointID int `json:"endpointId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()
	start := time.Now()
	cmdCode, data, _ := h.builder.BuildDiagnostics()
	resp, err := h.gateway.SendCommand(ctx, req.EndpointID, cmdCode, data)
	duration := time.Since(start)

	if err != nil {
		h.logAudit(ctx, c, req.EndpointID, CmdNC, "/admin/test-command", 0, 0, "", int(duration.Milliseconds()), err.Error())
		c.JSON(http.StatusOK, HSMCommandResponse{
			Success:      false,
			ErrorMessage: err.Error(),
			DurationMS:   int(duration.Milliseconds()),
			EndpointID:   req.EndpointID,
			CommandCode:  CmdNC,
		})
		return
	}

	_, respCode, respData, _ := DecodeResponse(4, resp)
	diagData, _ := h.parser.ParseDiagnostics(respCode, respData)
	h.logAudit(ctx, c, req.EndpointID, CmdNC, "/admin/test-command", 0, len(resp), respCode, int(duration.Milliseconds()), "")

	c.JSON(http.StatusOK, HSMCommandResponse{
		Success:     respCode == RespOK,
		ErrorCode:   respCode,
		Data:        diagData,
		DurationMS:  int(duration.Milliseconds()),
		EndpointID:  req.EndpointID,
		CommandCode: CmdNC,
	})
}

func (h *Handler) RawCommand(c *gin.Context) {
	var req RawCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	cmdCode, data, err := h.builder.BuildRawCommand(req.CommandHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	start := time.Now()
	resp, sendErr := h.gateway.SendCommand(ctx, req.EndpointID, cmdCode, data)
	duration := time.Since(start)

	if sendErr != nil {
		h.logAudit(ctx, c, req.EndpointID, cmdCode, "/admin/raw-command", len(data), 0, "", int(duration.Milliseconds()), sendErr.Error())
		c.JSON(http.StatusOK, HSMCommandResponse{
			Success:      false,
			ErrorMessage: sendErr.Error(),
			DurationMS:   int(duration.Milliseconds()),
			EndpointID:   req.EndpointID,
			CommandCode:  cmdCode,
		})
		return
	}

	_, respCode, respData, _ := DecodeResponse(4, resp)
	parsed, _ := h.parser.ParseRawResponse(respCode, respData)
	h.logAudit(ctx, c, req.EndpointID, cmdCode, "/admin/raw-command", len(data), len(resp), respCode, int(duration.Milliseconds()), "")

	c.JSON(http.StatusOK, HSMCommandResponse{
		Success:     respCode == RespOK,
		ErrorCode:   respCode,
		Data:        parsed,
		DurationMS:  int(duration.Milliseconds()),
		EndpointID:  req.EndpointID,
		CommandCode: cmdCode,
	})
}

// ── Internal helpers ────────────────────────────────────────────

func (h *Handler) executeCommand(
	c *gin.Context,
	capability string,
	buildFn func() (string, []byte, error),
	apiOp string,
	parseFn func(string, []byte) (json.RawMessage, error),
) {
	cmdCode, data, err := buildFn()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	var pool *ConnectionPool
	var endpointID int
	var routeErr error

	if gid := c.Query("groupId"); gid != "" {
		groupID, _ := strconv.Atoi(gid)
		pool, endpointID, routeErr = h.gateway.RouteRequestToGroup(ctx, groupID, capability)
	} else {
		pool, endpointID, routeErr = h.gateway.RouteRequest(ctx, capability)
	}
	if routeErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": routeErr.Error()})
		return
	}

	conn, acqErr := pool.Acquire(ctx)
	if acqErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no HSM connection available: " + acqErr.Error()})
		return
	}
	defer pool.Release(conn)

	timeout := time.Duration(pool.Endpoint().HostCommandTimeout) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	start := time.Now()
	resp, sendErr := conn.SendCommand(cmdCode, data, timeout)
	duration := time.Since(start)

	if sendErr != nil {
		h.logAudit(ctx, c, endpointID, cmdCode, apiOp, len(data), 0, "", int(duration.Milliseconds()), sendErr.Error())
		c.JSON(http.StatusBadGateway, gin.H{"error": "HSM command failed: " + sendErr.Error()})
		return
	}

	headerLen := pool.Endpoint().HeaderLength
	_, respCode, respData, parseErr := DecodeResponse(headerLen, resp)
	if parseErr != nil {
		h.logAudit(ctx, c, endpointID, cmdCode, apiOp, len(data), len(resp), "", int(duration.Milliseconds()), parseErr.Error())
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to parse HSM response: " + parseErr.Error()})
		return
	}

	result, resultErr := parseFn(respCode, respData)
	h.logAudit(ctx, c, endpointID, cmdCode, apiOp, len(data), len(resp), respCode, int(duration.Milliseconds()), "")

	if resultErr != nil {
		c.JSON(http.StatusOK, HSMCommandResponse{
			Success:      false,
			ErrorCode:    respCode,
			ErrorMessage: resultErr.Error(),
			DurationMS:   int(duration.Milliseconds()),
			EndpointID:   endpointID,
			CommandCode:  cmdCode,
		})
		return
	}

	c.JSON(http.StatusOK, HSMCommandResponse{
		Success:     respCode == RespOK,
		ErrorCode:   respCode,
		Data:        result,
		DurationMS:  int(duration.Milliseconds()),
		EndpointID:  endpointID,
		CommandCode: cmdCode,
	})
}

func (h *Handler) stubCommand(c *gin.Context, apiOp string, _ string) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":     "not yet implemented",
		"operation": apiOp,
	})
}

func (h *Handler) logAudit(ctx context.Context, c *gin.Context, endpointID int, cmdCode, apiOp string, reqSize, respSize int, respCode string, durationMS int, errMsg string) {
	entry := &HSMAuditEntry{
		EndpointID:   NullInt32{Int32: int32(endpointID), Valid: endpointID > 0},
		CommandCode:  cmdCode,
		APIOperation: apiOp,
		RequestSize:  reqSize,
		ResponseSize: respSize,
		ResponseCode: NullString{String: respCode, Valid: respCode != ""},
		DurationMS:   durationMS,
		ErrorMessage: NullString{String: errMsg, Valid: errMsg != ""},
		UserID:       c.GetInt("authUserID"),
		IPAddress:    NullString{String: c.ClientIP(), Valid: true},
	}
	h.store.LogAudit(ctx, entry) //nolint:errcheck
}
