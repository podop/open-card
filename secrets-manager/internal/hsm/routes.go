package hsm

import (
	"github.com/gin-gonic/gin"
	"github.com/your-org/secrets-manager/internal/actions"
	"github.com/your-org/secrets-manager/internal/middleware"
	"go.uber.org/zap"
)

// RegisterRoutes registers all HSM gateway API routes on the given engine.
func RegisterRoutes(r *gin.Engine, h *Handler, identityURL string, log *zap.Logger) {
	mw := func(action string) gin.HandlerFunc {
		return middleware.RequireAction(identityURL, action, log)
	}

	v1 := r.Group("/secrets/hsm/v1")

	// A. Endpoints / topology
	ep := v1.Group("/endpoints")
	{
		ep.GET("", mw(actions.ReadHSMEndpoints), h.ListEndpoints)
		ep.GET("/:id", mw(actions.ReadHSMEndpoints), h.GetEndpoint)
		ep.POST("", mw(actions.WriteHSMEndpoints), h.CreateEndpoint)
		ep.PUT("/:id", mw(actions.WriteHSMEndpoints), h.UpdateEndpoint)
		ep.DELETE("/:id", mw(actions.WriteHSMEndpoints), h.DeleteEndpoint)
		ep.GET("/:id/health", mw(actions.ReadHSMEndpoints), h.GetEndpointHealth)
		ep.POST("/:id/reconnect", mw(actions.WriteHSMEndpoints), h.ReconnectEndpoint)
		ep.POST("/:id/drain", mw(actions.WriteHSMEndpoints), h.DrainEndpoint)
		ep.POST("/:id/resume", mw(actions.WriteHSMEndpoints), h.ResumeEndpoint)
	}

	// Capabilities
	v1.GET("/capabilities", mw(actions.ReadHSMEndpoints), h.ListCapabilities)
	v1.GET("/capabilities/:model", mw(actions.ReadHSMEndpoints), h.GetModelCapabilities)

	// B. Sessions
	sess := v1.Group("/sessions")
	{
		sess.GET("", mw(actions.UseHSM), h.ListSessions)
		sess.POST("", mw(actions.UseHSM), h.CreateSession)
		sess.GET("/:id", mw(actions.UseHSM), h.GetSession)
		sess.DELETE("/:id", mw(actions.UseHSM), h.DeleteSession)
		sess.POST("/:id/ping", mw(actions.UseHSM), h.PingSession)
	}

	// C. Crypto: Keys
	keys := v1.Group("/keys")
	{
		keys.POST("/generate", mw(actions.UseHSM), h.GenerateKey)
		keys.POST("/generate-rsa", mw(actions.UseHSM), h.GenerateRSAKey)
		keys.POST("/import", mw(actions.UseHSM), h.ImportKey)
		keys.POST("/export", mw(actions.UseHSM), h.ExportKey)
		keys.POST("/translate", mw(actions.UseHSM), h.TranslateKey)
		keys.POST("/validate-check-value", mw(actions.UseHSM), h.ValidateCheckValue)
		keys.POST("/tr31/import", mw(actions.UseHSM), h.TR31Import)
		keys.POST("/tr31/export", mw(actions.UseHSM), h.TR31Export)
		keys.POST("/tr34/import", mw(actions.UseHSM), h.TR34Import)
	}

	// C. Crypto: PIN
	pin := v1.Group("/pin")
	{
		pin.POST("/verify", mw(actions.UseHSM), h.VerifyPIN)
		pin.POST("/translate", mw(actions.UseHSM), h.TranslatePIN)
		pin.POST("/generate-offset", mw(actions.UseHSM), h.GeneratePINOffset)
		pin.POST("/change", mw(actions.UseHSM), h.ChangePIN)
	}

	// C. Crypto: EMV
	emv := v1.Group("/emv")
	{
		emv.POST("/arqc/validate", mw(actions.UseHSM), h.ValidateARQC)
		emv.POST("/arpc/generate", mw(actions.UseHSM), h.GenerateARPC)
		emv.POST("/mac/generate", mw(actions.UseHSM), h.GenerateEMVMAC)
		emv.POST("/data/prepare", mw(actions.UseHSM), h.PrepareEMVData)
	}

	// C. Crypto: Cards
	cards := v1.Group("/cards")
	{
		cards.POST("/verify-cvv", mw(actions.UseHSM), h.VerifyCVV)
		cards.POST("/generate-cvv", mw(actions.UseHSM), h.GenerateCVV)
		cards.POST("/verify-pvv", mw(actions.UseHSM), h.VerifyPVV)
	}

	// C. Crypto: Tokens, P2PE, RKL
	v1.POST("/tokens/provision", mw(actions.UseHSM), h.ProvisionToken)

	p2pe := v1.Group("/p2pe")
	{
		p2pe.POST("/encrypt", mw(actions.UseHSM), h.EncryptP2PE)
		p2pe.POST("/decrypt", mw(actions.UseHSM), h.DecryptP2PE)
	}

	rkl := v1.Group("/rkl")
	{
		rkl.POST("/atm/load-key", mw(actions.UseHSM), h.LoadATMKey)
		rkl.POST("/pos/load-key", mw(actions.UseHSM), h.LoadPOSKey)
	}

	// D. Admin
	admin := v1.Group("/admin")
	{
		admin.GET("/audit", mw(actions.ReadHSMAudit), h.GetAuditLog)
		admin.GET("/metrics", mw(actions.ReadHSMMetrics), h.GetMetrics)
		admin.POST("/test-command", mw(actions.AdminHSM), h.TestCommand)
		admin.POST("/raw-command", mw(actions.AdminHSM), h.RawCommand)
		admin.POST("/generate-random", mw(actions.AdminHSM), h.GenerateRandom)
	}
}
