package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/identity-server/internal/actions"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	store *Store
	jwt   *JWTService
	log   *zap.Logger
}

func NewHandler(store *Store, jwt *JWTService, log *zap.Logger) *Handler {
	return &Handler{store: store, jwt: jwt, log: log}
}

// ReloadJWT re-creates the JWT service from current DB keys.
func (h *Handler) ReloadJWT() error {
	settings, err := h.store.GetAllSettings(nil)
	if err != nil {
		return err
	}
	svc, err := NewJWTService(settings["rsa_private_key"], settings["rsa_public_key"])
	if err != nil {
		return err
	}
	h.jwt = svc
	return nil
}

// ── Auth ─────────────────────────────────────────────────────────

func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	user, err := h.store.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if !user.Active {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "account disabled"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	accessToken, refreshToken, err := h.issueTokens(c, user)
	if err != nil {
		h.log.Error("token issue failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	userActions, _ := h.store.GetUserActions(c.Request.Context(), user.ID)
	c.JSON(http.StatusOK, gin.H{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"user": gin.H{
			"id":          user.ID,
			"username":    user.Username,
			"displayName": user.DisplayName,
			"actions":     userActions,
		},
	})
}

func (h *Handler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	rt, err := h.store.GetRefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}
	if time.Now().After(rt.ExpiresAt) {
		h.store.DeleteRefreshToken(c.Request.Context(), rt.ID) //nolint:errcheck
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token expired"})
		return
	}

	// Delete old refresh token (rotation)
	h.store.DeleteRefreshToken(c.Request.Context(), rt.ID) //nolint:errcheck

	user, err := h.store.GetUserByID(c.Request.Context(), rt.UserID)
	if err != nil || !user.Active {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found or disabled"})
		return
	}

	accessToken, refreshToken, err := h.issueTokens(c, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	userActions, _ := h.store.GetUserActions(c.Request.Context(), user.ID)
	c.JSON(http.StatusOK, gin.H{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"user": gin.H{
			"id":          user.ID,
			"username":    user.Username,
			"displayName": user.DisplayName,
			"actions":     userActions,
		},
	})
}

func (h *Handler) Verify(c *gin.Context) {
	var req struct {
		Token  string `json:"token"`
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Check if identity is active
	active, _ := h.store.GetSetting(c.Request.Context(), "active")
	if active == "false" {
		// Identity disabled — allow all
		c.JSON(http.StatusOK, gin.H{"allowed": true, "user": "system"})
		return
	}

	claims, err := h.jwt.Verify(req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"allowed": false, "error": err.Error()})
		return
	}

	// Check action permission
	allowed := false
	for _, a := range claims.Actions {
		if a == req.Action {
			allowed = true
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"allowed": allowed,
		"user":    claims.Sub,
		"userId":  claims.UserID,
	})
}

func (h *Handler) Me(c *gin.Context) {
	claims, ok := h.extractClaims(c)
	if !ok {
		return
	}
	user, err := h.store.GetUserByID(c.Request.Context(), claims.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	userActions, _ := h.store.GetUserActions(c.Request.Context(), user.ID)
	groups, _ := h.store.GetUserGroups(c.Request.Context(), user.ID)
	c.JSON(http.StatusOK, gin.H{
		"id":          user.ID,
		"username":    user.Username,
		"displayName": user.DisplayName,
		"active":      user.Active,
		"actions":     userActions,
		"groups":      groups,
	})
}

func (h *Handler) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	h.store.DeleteRefreshToken(c.Request.Context(), req.RefreshToken) //nolint:errcheck
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ── Settings ─────────────────────────────────────────────────────

func (h *Handler) GetSettings(c *gin.Context) {
	settings, err := h.store.GetAllSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"tokenExpiryMinutes": settings["token_expiry_minutes"],
		"refreshExpiryHours": settings["refresh_expiry_hours"],
		"active":             settings["active"],
		"rsaPublicKey":       settings["rsa_public_key"],
		"allActions":         actions.AllActions,
	})
}

func (h *Handler) PutSettings(c *gin.Context) {
	var req struct {
		TokenExpiryMinutes    *string `json:"tokenExpiryMinutes"`
		RefreshExpiryHours    *string `json:"refreshExpiryHours"`
		Active                *string `json:"active"`
		RegenerateRSA         bool    `json:"regenerateRSA"`
		RSAFromSecretsManager bool    `json:"rsaFromSecretsManager"`
		SecretsKeyID          int     `json:"secretsKeyId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()
	if req.TokenExpiryMinutes != nil {
		h.store.SetSetting(ctx, "token_expiry_minutes", *req.TokenExpiryMinutes) //nolint:errcheck
	}
	if req.RefreshExpiryHours != nil {
		h.store.SetSetting(ctx, "refresh_expiry_hours", *req.RefreshExpiryHours) //nolint:errcheck
	}
	if req.Active != nil {
		h.store.SetSetting(ctx, "active", *req.Active) //nolint:errcheck
	}
	if req.RegenerateRSA {
		privPEM, pubPEM, err := GenerateRSAKeyPair()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "RSA generation failed"})
			return
		}
		h.store.SetSetting(ctx, "rsa_private_key", privPEM) //nolint:errcheck
		h.store.SetSetting(ctx, "rsa_public_key", pubPEM)   //nolint:errcheck
		svc, err := NewJWTService(privPEM, pubPEM)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT reload failed"})
			return
		}
		h.jwt = svc
		h.log.Info("RSA key pair regenerated")
	}
	if req.RSAFromSecretsManager && req.SecretsKeyID > 0 {
		privPEM, pubPEM, err := h.fetchRSAFromSecretsManager(ctx, req.SecretsKeyID, c.GetHeader("Authorization"))
		if err != nil {
			h.log.Error("fetch RSA from secrets-manager failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch key from secrets-manager: " + err.Error()})
			return
		}
		h.store.SetSetting(ctx, "rsa_private_key", privPEM) //nolint:errcheck
		h.store.SetSetting(ctx, "rsa_public_key", pubPEM)   //nolint:errcheck
		svc, err := NewJWTService(privPEM, pubPEM)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT reload failed"})
			return
		}
		h.jwt = svc
		h.log.Info("RSA key pair loaded from secrets-manager", zap.Int("keyId", req.SecretsKeyID))
	}

	h.GetSettings(c)
}

// ── Groups ───────────────────────────────────────────────────────

func (h *Handler) ListGroups(c *gin.Context) {
	groups, err := h.store.ListGroups(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

func (h *Handler) CreateGroup(c *gin.Context) {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Actions     []string `json:"actions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	id, err := h.store.CreateGroup(c.Request.Context(), req.Name, req.Description, req.Actions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	group, _ := h.store.GetGroup(c.Request.Context(), id)
	c.JSON(http.StatusCreated, group)
}

func (h *Handler) UpdateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Actions     []string `json:"actions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.store.UpdateGroup(c.Request.Context(), id, req.Name, req.Description, req.Actions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	group, _ := h.store.GetGroup(c.Request.Context(), id)
	c.JSON(http.StatusOK, group)
}

func (h *Handler) DeleteGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.store.DeleteGroup(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ── Users ────────────────────────────────────────────────────────

func (h *Handler) ListUsers(c *gin.Context) {
	users, err := h.store.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
		GroupIDs    []int  `json:"groupIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hash failed"})
		return
	}
	id, err := h.store.CreateUser(c.Request.Context(), req.Username, string(hash), req.DisplayName)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}
	if len(req.GroupIDs) > 0 {
		h.store.SetUserGroups(c.Request.Context(), id, req.GroupIDs) //nolint:errcheck
	}
	user, _ := h.store.GetUserByID(c.Request.Context(), id)
	groups, _ := h.store.GetUserGroups(c.Request.Context(), id)
	c.JSON(http.StatusCreated, UserWithGroups{User: *user, Groups: groups})
}

func (h *Handler) UpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		DisplayName *string `json:"displayName"`
		Active      *bool   `json:"active"`
		Password    *string `json:"password"`
		GroupIDs    *[]int  `json:"groupIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()
	user, err := h.store.GetUserByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	displayName := user.DisplayName
	active := user.Active
	if req.DisplayName != nil {
		displayName = *req.DisplayName
	}
	if req.Active != nil {
		active = *req.Active
	}
	h.store.UpdateUser(ctx, id, displayName, active) //nolint:errcheck

	if req.Password != nil && *req.Password != "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		h.store.UpdateUserPassword(ctx, id, string(hash)) //nolint:errcheck
	}
	if req.GroupIDs != nil {
		h.store.SetUserGroups(ctx, id, *req.GroupIDs) //nolint:errcheck
	}

	updatedUser, _ := h.store.GetUserByID(ctx, id)
	groups, _ := h.store.GetUserGroups(ctx, id)
	c.JSON(http.StatusOK, UserWithGroups{User: *updatedUser, Groups: groups})
}

func (h *Handler) DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.store.DeleteUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ── Helpers ──────────────────────────────────────────────────────

func (h *Handler) issueTokens(c *gin.Context, user *User) (accessToken, refreshToken string, err error) {
	ctx := c.Request.Context()
	userActions, _ := h.store.GetUserActions(ctx, user.ID)

	expiryStr, _ := h.store.GetSetting(ctx, "token_expiry_minutes")
	expiryMin, _ := strconv.Atoi(expiryStr)
	if expiryMin <= 0 {
		expiryMin = 60
	}

	now := time.Now()
	claims := &Claims{
		Sub:     user.Username,
		Iss:     "acs-identity",
		Exp:     now.Add(time.Duration(expiryMin) * time.Minute).Unix(),
		Iat:     now.Unix(),
		Jti:     uuid.NewString(),
		UserID:  user.ID,
		Actions: userActions,
	}

	accessToken, err = h.jwt.Sign(claims)
	if err != nil {
		return "", "", err
	}

	// Refresh token
	refreshID := uuid.NewString()
	refreshHoursStr, _ := h.store.GetSetting(ctx, "refresh_expiry_hours")
	refreshHours, _ := strconv.Atoi(refreshHoursStr)
	if refreshHours <= 0 {
		refreshHours = 720
	}
	refreshExpiry := now.Add(time.Duration(refreshHours) * time.Hour)
	if err := h.store.SaveRefreshToken(ctx, refreshID, user.ID, refreshExpiry); err != nil {
		return "", "", err
	}

	return accessToken, refreshID, nil
}

func (h *Handler) fetchRSAFromSecretsManager(ctx context.Context, keyID int, authHeader string) (privPEM, pubPEM string, err error) {
	smURL := os.Getenv("IDENTITY_SECRETS_MANAGER_URL")
	if smURL == "" {
		smURL = "http://secrets:8445"
	}

	url := fmt.Sprintf("%s/secrets/keys/%d/export", smURL, keyID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("secrets-manager request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("secrets-manager returned status %d", resp.StatusCode)
	}

	var result struct {
		PrivateKeyPem string `json:"privateKeyPem"`
		PublicKeyPem  string `json:"publicKeyPem"`
		Material      string `json:"material"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("decode response: %w", err)
	}

	privPEM = result.PrivateKeyPem
	if privPEM == "" {
		privPEM = result.Material
	}
	pubPEM = result.PublicKeyPem

	if privPEM == "" || pubPEM == "" {
		return "", "", fmt.Errorf("key material incomplete from secrets-manager")
	}

	return privPEM, pubPEM, nil
}

func (h *Handler) extractClaims(c *gin.Context) (*Claims, bool) {
	auth := c.GetHeader("Authorization")
	if len(auth) < 8 || auth[:7] != "Bearer " {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return nil, false
	}
	claims, err := h.jwt.Verify(auth[7:])
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return nil, false
	}
	return claims, true
}
