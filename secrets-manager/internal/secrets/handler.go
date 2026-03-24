package secrets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Handler struct {
	store        *Store
	log          *zap.Logger
	masterKeyHex string
	hsmStore     HsmEndpointLinker
}

// HsmEndpointLinker allows the secrets handler to create/update/delete HSM endpoints.
type HsmEndpointLinker interface {
	CreateLinkedEndpoint(ctx context.Context, name string, config json.RawMessage, createdBy int) (int, error)
	UpdateLinkedEndpoint(ctx context.Context, endpointID int, name string, config json.RawMessage) error
	DeleteLinkedEndpoint(ctx context.Context, endpointID int) error
}

func NewHandler(store *Store, log *zap.Logger, masterKeyHex string) *Handler {
	return &Handler{store: store, log: log, masterKeyHex: masterKeyHex}
}

func (h *Handler) SetHsmLinker(linker HsmEndpointLinker) {
	h.hsmStore = linker
}

// ── Keys ────────────────────────────────────────────────────────

func (h *Handler) ListKeys(c *gin.Context) {
	keys, err := h.store.ListKeys(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, keys)
}

func (h *Handler) GetKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	key, err := h.store.GetKey(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}
	c.JSON(http.StatusOK, key)
}

func (h *Handler) GenerateKey(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		KeyType     string `json:"keyType"`
		KeyUsage    string `json:"keyUsage"`
		ProviderID  int    `json:"providerId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.KeyUsage == "" {
		req.KeyUsage = "general"
	}

	ctx := c.Request.Context()
	provider, err := h.store.GetProvider(ctx, req.ProviderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider not found"})
		return
	}

	backend := GetProviderBackend(provider, h.masterKeyHex)
	if !backend.SupportsKeyType(req.KeyType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider does not support key type " + req.KeyType})
		return
	}

	var generated *GeneratedKey
	bits := keySizeBits(req.KeyType)

	if isAsymmetric(req.KeyType) {
		generated, err = backend.GenerateKeyPair(ctx, req.KeyType, bits)
	} else {
		generated, err = backend.GenerateSymmetricKey(ctx, req.KeyType, bits)
	}
	if err != nil {
		h.log.Error("key generation failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "key generation failed"})
		return
	}

	userID := c.GetInt("authUserID")
	key := &Key{
		Name:        req.Name,
		Description: req.Description,
		KeyType:     req.KeyType,
		KeyUsage:    req.KeyUsage,
		ProviderID:  req.ProviderID,
		PublicKeyPEM: NullString{String: generated.PublicKeyPEM, Valid: generated.PublicKeyPEM != ""},
		PrivateKeyEnc: NullString{String: generated.PrivateKeyEnc, Valid: generated.PrivateKeyEnc != ""},
		HSMKeyHandle: NullString{String: generated.HSMKeyHandle, Valid: generated.HSMKeyHandle != ""},
		Fingerprint: generated.Fingerprint,
		KeySizeBits: generated.KeySizeBits,
		Version:     1,
		OwnerUserID: userID,
		Active:      true,
	}

	id, err := h.store.CreateKey(ctx, key)
	if err != nil {
		h.log.Error("save key failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save key failed"})
		return
	}

	h.logUsage(ctx, c, id, userID, "generate")

	created, _ := h.store.GetKey(ctx, id)
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) ImportKey(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		KeyType     string `json:"keyType"`
		KeyUsage    string `json:"keyUsage"`
		ProviderID  int    `json:"providerId"`
		PublicPEM   string `json:"publicPem"`
		PrivatePEM  string `json:"privatePem"`
		CertPEM     string `json:"certPem"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.KeyUsage == "" {
		req.KeyUsage = "general"
	}

	ctx := c.Request.Context()
	userID := c.GetInt("authUserID")

	var privEnc NullString
	if req.PrivatePEM != "" {
		enc, err := encryptPrivateKey(req.PrivatePEM, h.masterKeyHex)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "encrypt failed"})
			return
		}
		privEnc = NullString{String: enc, Valid: true}
	}

	fingerprint := computePEMFingerprint(req.PublicPEM + req.CertPEM + req.PrivatePEM)

	key := &Key{
		Name:          req.Name,
		Description:   req.Description,
		KeyType:       req.KeyType,
		KeyUsage:      req.KeyUsage,
		ProviderID:    req.ProviderID,
		PublicKeyPEM:  NullString{String: req.PublicPEM, Valid: req.PublicPEM != ""},
		PrivateKeyEnc: privEnc,
		CertificatePEM: NullString{String: req.CertPEM, Valid: req.CertPEM != ""},
		Fingerprint:   fingerprint,
		KeySizeBits:   keySizeBits(req.KeyType),
		Version:       1,
		OwnerUserID:   userID,
		Active:        true,
	}

	id, err := h.store.CreateKey(ctx, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save key failed"})
		return
	}

	h.logUsage(ctx, c, id, userID, "import")

	created, _ := h.store.GetKey(ctx, id)
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) GenerateCertificate(c *gin.Context) {
	var req struct {
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		ProviderID   int      `json:"providerId"`
		CommonName   string   `json:"commonName"`
		Organization string   `json:"organization"`
		Country      string   `json:"country"`
		ValidityDays int      `json:"validityDays"`
		SANs         []string `json:"sans"`
		KeyBits      int      `json:"keyBits"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()
	provider, err := h.store.GetProvider(ctx, req.ProviderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider not found"})
		return
	}

	backend := GetProviderBackend(provider, h.masterKeyHex)
	opts := CertificateOptions{
		CommonName:   req.CommonName,
		Organization: req.Organization,
		Country:      req.Country,
		ValidityDays: req.ValidityDays,
		SANs:         req.SANs,
		KeyBits:      req.KeyBits,
	}

	generated, err := backend.GenerateCertificate(ctx, opts)
	if err != nil {
		h.log.Error("certificate generation failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "certificate generation failed"})
		return
	}

	userID := c.GetInt("authUserID")
	bits := req.KeyBits
	if bits == 0 {
		bits = 2048
	}

	validityDays := req.ValidityDays
	if validityDays <= 0 {
		validityDays = 365
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(time.Duration(validityDays) * 24 * time.Hour)

	key := &Key{
		Name:           req.Name,
		Description:    req.Description,
		KeyType:        "X509",
		KeyUsage:       "tls",
		ProviderID:     req.ProviderID,
		CertificatePEM: NullString{String: generated.CertificatePEM, Valid: generated.CertificatePEM != ""},
		PrivateKeyEnc:  NullString{String: generated.PrivateKeyEnc, Valid: generated.PrivateKeyEnc != ""},
		HSMKeyHandle:   NullString{String: generated.HSMKeyHandle, Valid: generated.HSMKeyHandle != ""},
		Fingerprint:    generated.Fingerprint,
		KeySizeBits:    bits,
		NotBefore:      NullTime{Time: notBefore, Valid: true},
		NotAfter:       NullTime{Time: notAfter, Valid: true},
		Version:        1,
		OwnerUserID:    userID,
		Active:         true,
	}

	id, err := h.store.CreateKey(ctx, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save certificate failed"})
		return
	}

	h.logUsage(ctx, c, id, userID, "generate_certificate")

	created, _ := h.store.GetKey(ctx, id)
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) UpdateKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		KeyUsage    string `json:"keyUsage"`
		Active      bool   `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.store.UpdateKey(c.Request.Context(), id, req.Name, req.Description, req.KeyUsage, req.Active); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	key, _ := h.store.GetKey(c.Request.Context(), id)
	c.JSON(http.StatusOK, key)
}

func (h *Handler) DeleteKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.store.DeleteKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) RotateKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	userID := c.GetInt("authUserID")

	newKey, err := h.rotateKeyInternal(ctx, id, NullInt32{Int32: int32(userID), Valid: true}, "manual")
	if err != nil {
		h.log.Error("key rotation failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, newKey)
}

func (h *Handler) ExportPublicKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	key, err := h.store.GetKey(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}

	userID := c.GetInt("authUserID")
	h.logUsage(ctx, c, id, userID, "export_public")

	result := gin.H{"id": key.ID, "fingerprint": key.Fingerprint}
	if key.PublicKeyPEM.Valid {
		result["publicKeyPem"] = key.PublicKeyPEM.String
	}
	if key.CertificatePEM.Valid {
		result["certificatePem"] = key.CertificatePEM.String
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetKeyUsageLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	logs, err := h.store.GetKeyUsageLog(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, logs)
}

func (h *Handler) AssignPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		PolicyID int `json:"policyId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()
	policy, err := h.store.GetPolicy(ctx, req.PolicyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "policy not found"})
		return
	}

	nextRotation := parseIntervalToTime(policy.RotationInterval)

	if err := h.store.AssignPolicy(ctx, id, req.PolicyID, nextRotation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	key, _ := h.store.GetKey(ctx, id)
	c.JSON(http.StatusOK, key)
}

// ── Key Access ──────────────────────────────────────────────────

func (h *Handler) ListKeyAccess(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	access, err := h.store.ListKeyAccess(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, access)
}

func (h *Handler) GrantKeyAccess(c *gin.Context) {
	keyID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		UserID     int    `json:"userId"`
		Permission string `json:"permission"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	grantedBy := c.GetInt("authUserID")

	access := &KeyAccess{
		KeyID:      keyID,
		UserID:     req.UserID,
		Permission: req.Permission,
		GrantedBy:  grantedBy,
	}
	id, err := h.store.GrantKeyAccess(c.Request.Context(), access)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) RevokeKeyAccess(c *gin.Context) {
	keyID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key id"})
		return
	}
	userID, err := strconv.Atoi(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if err := h.store.RevokeKeyAccess(c.Request.Context(), keyID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ── Rotation Policies ───────────────────────────────────────────

func (h *Handler) ListPolicies(c *gin.Context) {
	policies, err := h.store.ListPolicies(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policies)
}

func (h *Handler) CreatePolicy(c *gin.Context) {
	var req struct {
		Name             string `json:"name"`
		Description      string `json:"description"`
		KeyType          string `json:"keyType"`
		RotationInterval string `json:"rotationInterval"`
		NotifyBefore     string `json:"notifyBefore"`
		AutoRotate       bool   `json:"autoRotate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	userID := c.GetInt("authUserID")

	policy := &RotationPolicy{
		Name:             req.Name,
		Description:      req.Description,
		KeyType:          req.KeyType,
		RotationInterval: req.RotationInterval,
		NotifyBefore:     req.NotifyBefore,
		AutoRotate:       req.AutoRotate,
		CreatedBy:        userID,
	}

	id, err := h.store.CreatePolicy(c.Request.Context(), policy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	created, _ := h.store.GetPolicy(c.Request.Context(), id)
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) UpdatePolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Name             string `json:"name"`
		Description      string `json:"description"`
		KeyType          string `json:"keyType"`
		RotationInterval string `json:"rotationInterval"`
		NotifyBefore     string `json:"notifyBefore"`
		AutoRotate       bool   `json:"autoRotate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	policy := &RotationPolicy{
		Name:             req.Name,
		Description:      req.Description,
		KeyType:          req.KeyType,
		RotationInterval: req.RotationInterval,
		NotifyBefore:     req.NotifyBefore,
		AutoRotate:       req.AutoRotate,
	}
	if err := h.store.UpdatePolicy(c.Request.Context(), id, policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	updated, _ := h.store.GetPolicy(c.Request.Context(), id)
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) DeletePolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.store.DeletePolicy(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ── Security Providers ──────────────────────────────────────────

func (h *Handler) ListProviders(c *gin.Context) {
	providers, err := h.store.ListProviders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type providerWithCount struct {
		SecurityProvider
		KeysCount int `json:"keysCount"`
	}

	result := make([]providerWithCount, len(providers))
	for i, p := range providers {
		count, _ := h.store.CountKeysByProvider(c.Request.Context(), p.ID)
		result[i] = providerWithCount{SecurityProvider: p, KeysCount: count}
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetProvider(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	provider, err := h.store.GetProvider(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}
	c.JSON(http.StatusOK, provider)
}

func (h *Handler) CreateProvider(c *gin.Context) {
	var req struct {
		Name         string          `json:"name"`
		ProviderType string          `json:"providerType"`
		Config       json.RawMessage `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Config == nil {
		req.Config = json.RawMessage(`{}`)
	}
	userID := c.GetInt("authUserID")

	provider := &SecurityProvider{
		Name:         req.Name,
		ProviderType: req.ProviderType,
		Config:       req.Config,
		Active:       true,
		CreatedBy:    userID,
	}

	id, err := h.store.CreateProvider(c.Request.Context(), provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Auto-create linked HSM endpoint for HSM providers
	created, _ := h.store.GetProvider(c.Request.Context(), id)
	if req.ProviderType == "hsm" && h.hsmStore != nil {
		epID, epErr := h.hsmStore.CreateLinkedEndpoint(c.Request.Context(), req.Name, req.Config, userID)
		if epErr != nil {
			h.log.Warn("auto-create HSM endpoint failed", zap.Error(epErr))
		} else {
			// Store the endpoint ID in provider config
			var cfg map[string]interface{}
			if err := json.Unmarshal(req.Config, &cfg); err == nil {
				cfg["hsmEndpointId"] = epID
				cfg["linkedProviderId"] = id
				if newCfg, err := json.Marshal(cfg); err == nil {
					h.store.UpdateProvider(c.Request.Context(), id, req.Name, newCfg, true) //nolint:errcheck
					created, _ = h.store.GetProvider(c.Request.Context(), id)
				}
			}
		}
	}

	c.JSON(http.StatusCreated, created)
}

func (h *Handler) UpdateProvider(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
		Active bool            `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.store.UpdateProvider(c.Request.Context(), id, req.Name, req.Config, req.Active); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Sync linked HSM endpoint
	provider, _ := h.store.GetProvider(c.Request.Context(), id)
	if provider != nil && provider.ProviderType == "hsm" && h.hsmStore != nil {
		var cfg map[string]interface{}
		if err := json.Unmarshal(provider.Config, &cfg); err == nil {
			if epID, ok := cfg["hsmEndpointId"].(float64); ok && epID > 0 {
				h.hsmStore.UpdateLinkedEndpoint(c.Request.Context(), int(epID), req.Name, req.Config) //nolint:errcheck
			}
		}
	}

	c.JSON(http.StatusOK, provider)
}

func (h *Handler) DeleteProvider(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	count, _ := h.store.CountKeysByProvider(c.Request.Context(), id)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "provider has associated keys; remove them first"})
		return
	}

	// Delete linked HSM endpoint
	provider, _ := h.store.GetProvider(c.Request.Context(), id)
	if provider != nil && provider.ProviderType == "hsm" && h.hsmStore != nil {
		var cfg map[string]interface{}
		if err := json.Unmarshal(provider.Config, &cfg); err == nil {
			if epID, ok := cfg["hsmEndpointId"].(float64); ok && epID > 0 {
				h.hsmStore.DeleteLinkedEndpoint(c.Request.Context(), int(epID)) //nolint:errcheck
			}
		}
	}

	if err := h.store.DeleteProvider(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) TestProvider(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	provider, err := h.store.GetProvider(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	backend := GetProviderBackend(provider, h.masterKeyHex)
	if err := backend.TestConnection(c.Request.Context()); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ── Key Export ──────────────────────────────────────────────────

func (h *Handler) ExportKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	key, err := h.store.GetKey(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}

	userID := c.GetInt("authUserID")
	h.logUsage(ctx, c, id, userID, "export")

	result := gin.H{"id": key.ID, "keyType": key.KeyType, "fingerprint": key.Fingerprint}

	// For asymmetric keys, return PEMs
	if isAsymmetric(key.KeyType) || key.KeyType == "X509" {
		if key.PublicKeyPEM.Valid {
			result["publicKeyPem"] = key.PublicKeyPEM.String
		}
		if key.CertificatePEM.Valid {
			result["certificatePem"] = key.CertificatePEM.String
		}
		if key.PrivateKeyEnc.Valid {
			plaintext, err := decryptPrivateKey(key.PrivateKeyEnc.String, h.masterKeyHex)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "decrypt failed"})
				return
			}
			result["privateKeyPem"] = plaintext
			result["material"] = plaintext
		}
	} else {
		// For symmetric keys, the private key enc field holds the encrypted key material (hex)
		if key.PrivateKeyEnc.Valid {
			plaintext, err := decryptPrivateKey(key.PrivateKeyEnc.String, h.masterKeyHex)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "decrypt failed"})
				return
			}
			result["material"] = plaintext
		}
	}

	c.JSON(http.StatusOK, result)
}

// ── Internal helpers ────────────────────────────────────────────

func (h *Handler) rotateKeyInternal(ctx context.Context, keyID int, rotatedBy NullInt32, trigger string) (*Key, error) {
	oldKey, err := h.store.GetKey(ctx, keyID)
	if err != nil {
		return nil, err
	}

	provider, err := h.store.GetProvider(ctx, oldKey.ProviderID)
	if err != nil {
		return nil, err
	}

	backend := GetProviderBackend(provider, h.masterKeyHex)
	bits := keySizeBits(oldKey.KeyType)

	var generated *GeneratedKey
	if oldKey.KeyType == "X509" {
		generated, err = backend.GenerateCertificate(ctx, CertificateOptions{KeyBits: bits})
	} else if isAsymmetric(oldKey.KeyType) {
		generated, err = backend.GenerateKeyPair(ctx, oldKey.KeyType, bits)
	} else {
		generated, err = backend.GenerateSymmetricKey(ctx, oldKey.KeyType, bits)
	}
	if err != nil {
		h.store.LogRotation(ctx, &RotationHistory{ //nolint:errcheck
			KeyID: keyID, OldVersion: oldKey.Version, NewVersion: oldKey.Version,
			RotatedBy: rotatedBy, Trigger: trigger, Status: "failed",
			ErrorMessage: NullString{String: err.Error(), Valid: true},
		})
		return nil, err
	}

	var nextRotation NullTime
	if oldKey.RotationPolicyID.Valid {
		policy, pErr := h.store.GetPolicy(ctx, int(oldKey.RotationPolicyID.Int32))
		if pErr == nil {
			nextRotation = parseIntervalToTime(policy.RotationInterval)
		}
	}

	newKey := &Key{
		Name:             oldKey.Name,
		Description:      oldKey.Description,
		KeyType:          oldKey.KeyType,
		KeyUsage:         oldKey.KeyUsage,
		ProviderID:       oldKey.ProviderID,
		PublicKeyPEM:     NullString{String: generated.PublicKeyPEM, Valid: generated.PublicKeyPEM != ""},
		PrivateKeyEnc:    NullString{String: generated.PrivateKeyEnc, Valid: generated.PrivateKeyEnc != ""},
		CertificatePEM:   NullString{String: generated.CertificatePEM, Valid: generated.CertificatePEM != ""},
		HSMKeyHandle:     NullString{String: generated.HSMKeyHandle, Valid: generated.HSMKeyHandle != ""},
		Fingerprint:      generated.Fingerprint,
		KeySizeBits:      generated.KeySizeBits,
		RotationPolicyID: oldKey.RotationPolicyID,
		LastRotatedAt:    NullTime{Time: time.Now(), Valid: true},
		NextRotationAt:   nextRotation,
		Version:          oldKey.Version + 1,
		PredecessorID:    NullInt32{Int32: int32(keyID), Valid: true},
		OwnerUserID:      oldKey.OwnerUserID,
		Active:           true,
	}

	newID, err := h.store.CreateKey(ctx, newKey)
	if err != nil {
		return nil, err
	}

	h.store.DeactivateKey(ctx, keyID) //nolint:errcheck
	h.store.CopyKeyAccess(ctx, keyID, newID) //nolint:errcheck

	h.store.LogRotation(ctx, &RotationHistory{ //nolint:errcheck
		KeyID: newID, OldVersion: oldKey.Version, NewVersion: oldKey.Version + 1,
		RotatedBy: rotatedBy, Trigger: trigger, Status: "success",
	})

	return h.store.GetKey(ctx, newID)
}

func (h *Handler) logUsage(ctx context.Context, c *gin.Context, keyID, userID int, action string) {
	h.store.LogKeyUsage(ctx, &KeyUsageLog{ //nolint:errcheck
		KeyID:     keyID,
		UserID:    userID,
		Action:    action,
		IPAddress: NullString{String: c.ClientIP(), Valid: true},
		UserAgent: NullString{String: c.GetHeader("User-Agent"), Valid: true},
		Details:   json.RawMessage(`{}`),
	})
}

func isAsymmetric(keyType string) bool {
	return strings.HasPrefix(keyType, "RSA-")
}

func keySizeBits(keyType string) int {
	switch keyType {
	case "RSA-1024":
		return 1024
	case "RSA-2048":
		return 2048
	case "RSA-3072":
		return 3072
	case "RSA-4096":
		return 4096
	case "AES-128":
		return 128
	case "AES-192":
		return 192
	case "AES-256":
		return 256
	case "3DES":
		return 192
	case "X509":
		return 2048
	}
	return 0
}

func computePEMFingerprint(data string) string {
	h := sha256Sum([]byte(data))
	return h
}

func sha256Sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func parseIntervalToTime(interval string) NullTime {
	dur := parsePostgresInterval(interval)
	if dur == 0 {
		return NullTime{}
	}
	return NullTime{Time: time.Now().Add(dur), Valid: true}
}

func parsePostgresInterval(s string) time.Duration {
	s = strings.TrimSpace(s)
	// Handle common formats: "2 years", "1 year", "6 months", "30 days"
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return 0
	}
	val, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	unit := strings.ToLower(parts[1])
	switch {
	case strings.HasPrefix(unit, "year"):
		return time.Duration(val) * 365 * 24 * time.Hour
	case strings.HasPrefix(unit, "month"):
		return time.Duration(val) * 30 * 24 * time.Hour
	case strings.HasPrefix(unit, "day"):
		return time.Duration(val) * 24 * time.Hour
	case strings.HasPrefix(unit, "hour"):
		return time.Duration(val) * time.Hour
	}
	return 0
}
