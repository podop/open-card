package http

import (
	"github.com/gin-gonic/gin"
	"github.com/your-org/acs-server/internal/adapter/http/handler"
	"github.com/your-org/acs-server/internal/adapter/http/middleware"
	"github.com/your-org/acs-server/internal/identity/actions"
	"go.uber.org/zap"
)

// NewRouter assembles the Gin engine with all ACS routes.
func NewRouter(
	authH      *handler.AuthHandler,
	methodH    *handler.MethodHandler,
	challengeH *handler.ChallengeHandler,
	apiH       *handler.ApiHandler,
	scenarioH  *handler.ScenarioHandler,
	oreqH      *handler.OReqHandler,
	identityURL string,
	log        *zap.Logger,
) *gin.Engine {
	const (
		ctJSON = "application/json"
		ctForm = "application/x-www-form-urlencoded"
	)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(log))

	// Health check (no Content-Type constraint on GET)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Helper to build auth middleware for a given action.
	mw := func(action string) gin.HandlerFunc {
		return middleware.RequireAction(identityURL, action, log)
	}

	v1 := r.Group("/v1")
	{
		// ── Authentication (AReq → ARes) ──────────────────────────────────
		// DS → ACS; EMVCo requires application/json.
		v1.POST("/auth",
			middleware.EnforceContentType(ctJSON),
			authH.HandleAReq,
		)

		// ── 3DS Method (device fingerprinting) ───────────────────────────
		// Browser iframe form-posts threeDSMethodData.
		v1.POST("/method",
			middleware.EnforceContentType(ctForm),
			methodH.HandleMethodInit,
		)
		v1.POST("/method/notification/:sessionID",
			middleware.EnforceContentType(ctForm),
			methodH.HandleMethodNotification,
		)

		// ── Challenge flow ───────────────────────────────────────────────
		v1.POST("/challenge/creq",
			middleware.EnforceContentType(ctJSON, ctForm),
			challengeH.HandleCReq,
		)
		v1.GET("/challenge/:acsTransID", challengeH.HandleChallengeInit)
		v1.POST("/challenge/:acsTransID/form",
			middleware.EnforceContentType(ctForm),
			challengeH.HandleChallengeFormSubmit,
		)
		// PIN (second factor)
		v1.GET("/challenge/:acsTransID/pin", challengeH.HandlePINInit)
		v1.POST("/challenge/:acsTransID/pin",
			middleware.EnforceContentType(ctForm),
			challengeH.HandlePINFormSubmit,
		)
		v1.POST("/challenge/pin/submit",
			middleware.EnforceContentType(ctJSON),
			challengeH.HandlePINSubmitJSON,
		)

		// Cancel / Timeout
		v1.POST("/challenge/:acsTransID/cancel", challengeH.HandleChallengeCancel)
		v1.POST("/challenge/:acsTransID/timeout", challengeH.HandleChallengeTimeout)

		// ── OOB / Decoupled authentication ────────────────────────
		v1.POST("/challenge/:acsTransID/oob",
			middleware.EnforceContentType(ctJSON),
			challengeH.HandleOOBInitiate,
		)
		v1.GET("/challenge/:acsTransID/oob/status", challengeH.HandleOOBStatus)
		v1.POST("/challenge/:acsTransID/oob/fallback",
			middleware.EnforceContentType(ctJSON),
			challengeH.HandleOOBFallback,
		)
		v1.POST("/challenge/:acsTransID/oob/result",
			middleware.EnforceContentType(ctJSON),
			challengeH.HandleOOBResult,
		)

		v1.POST("/challenge/submit",
			middleware.EnforceContentType(ctJSON),
			challengeH.HandleChallengeSubmit,
		)

		// ── OReq/ORes (DS → ACS operational messaging) ──────────────
		v1.POST("/oreq",
			middleware.EnforceContentType(ctJSON),
			oreqH.HandleOReq,
		)
	}

	// ── Admin / monitoring API (consumed by the React UI) ────────────────
	api := r.Group("/api")
	api.Use(middleware.CORS())
	{
		// Dashboard & analytics
		api.GET("/dashboard", mw(actions.ReadDashboard), apiH.GetDashboard)
		api.GET("/transactions", mw(actions.ReadTransactions), apiH.GetTransactions)
		api.GET("/transactions/:acsTransID", mw(actions.ReadTransactions), apiH.GetTransaction)
		api.GET("/audit-logs", mw(actions.ReadAuditLogs), apiH.GetAuditLogs)

		// Card ranges
		api.GET("/card-ranges", mw(actions.ReadCardRanges), apiH.GetCardRanges)
		api.POST("/card-ranges", mw(actions.WriteCardRanges), apiH.PostCardRange)
		api.DELETE("/card-ranges", mw(actions.WriteCardRanges), apiH.DeleteCardRange)

		// Config
		api.GET("/config", mw(actions.ReadConfig), apiH.GetConfig)
		api.PUT("/config", mw(actions.WriteConfig), apiH.PutConfig)

		// Network settings
		api.PATCH("/config/network/:network", mw(actions.WriteNetworkSettings), apiH.PatchNetworkPolicy)
		api.PUT("/config/network/:network", mw(actions.WriteNetworkSettings), apiH.PutNetworkConfig)

		// Certificates
		api.GET("/config/network/:network/certificate", mw(actions.ReadCertificates), apiH.GetNetworkCertificate)
		api.GET("/config/network/:network/certificates", mw(actions.ReadCertificates), apiH.GetNetworkCertificates)
		api.POST("/config/network/:network/certificate", mw(actions.WriteCertificates), apiH.PostNetworkCertificate)
		api.DELETE("/config/network/:network/certificate", mw(actions.WriteCertificates), apiH.DeleteNetworkCertificate)

		// Channel resolvers
		api.GET("/config/channel-resolvers", mw(actions.ReadChannelResolvers), apiH.GetChannelResolvers)
		api.PATCH("/config/channel-resolvers", mw(actions.WriteChannelResolvers), apiH.PatchChannelResolvers)

		// MFA Providers
		api.GET("/mfa-providers", mw(actions.ReadMFAProviders), apiH.GetMFAProviders)
		api.POST("/mfa-providers", mw(actions.WriteMFAProviders), apiH.PostMFAProvider)
		api.PUT("/mfa-providers/:id", mw(actions.WriteMFAProviders), apiH.PutMFAProvider)
		api.DELETE("/mfa-providers/:id", mw(actions.WriteMFAProviders), apiH.DeleteMFAProvider)

		// Networks CRUD
		api.GET("/networks", mw(actions.ReadNetworkSettings), apiH.GetNetworks)
		api.POST("/networks", mw(actions.WriteNetworkSettings), apiH.PostNetwork)
		api.PUT("/networks/:name", mw(actions.WriteNetworkSettings), apiH.PutNetwork)
		api.DELETE("/networks/:name", mw(actions.WriteNetworkSettings), apiH.DeleteNetwork)

		// Network rules & BIN exclusions
		api.GET("/network-rules", mw(actions.ReadNetworkRules), apiH.GetNetworkRules)
		api.PUT("/network-rules/:network", mw(actions.WriteNetworkRules), apiH.PutNetworkRule)
		api.GET("/bin-exclusions", mw(actions.ReadBINSettings), apiH.GetBinExclusions)
		api.POST("/bin-exclusions", mw(actions.WriteBINSettings), apiH.PostBinExclusion)
		api.PUT("/bin-exclusions/:id", mw(actions.WriteBINSettings), apiH.PutBinExclusion)
		api.DELETE("/bin-exclusions/:id", mw(actions.WriteBINSettings), apiH.DeleteBinExclusion)

		// Test scenarios
		api.GET("/scenarios", mw(actions.ReadTestScenario), scenarioH.ListScenarios)
		api.POST("/scenarios", mw(actions.WriteTestScenario), scenarioH.CreateScenario)
		api.GET("/scenarios/:id", mw(actions.ReadTestScenario), scenarioH.GetScenario)
		api.PUT("/scenarios/:id", mw(actions.WriteTestScenario), scenarioH.UpdateScenario)
		api.DELETE("/scenarios/:id", mw(actions.WriteTestScenario), scenarioH.DeleteScenario)
		api.POST("/scenarios/:id/run", mw(actions.RunTestScenario), scenarioH.RunScenario)

		// OReq/ORes history (DS → ACS operational messaging)
		api.GET("/oreq-status", mw(actions.ReadOReqHistory), apiH.GetOReqStatus)
		api.GET("/oreq-history", mw(actions.ReadOReqHistory), apiH.GetOReqHistory)

		api.GET("/scenario-runs", mw(actions.ReadTestScenario), scenarioH.ListRuns)
		api.GET("/scenario-runs/stats", mw(actions.ReadTestScenario), scenarioH.GetRunStats)
		api.GET("/scenario-runs/:id", mw(actions.ReadTestScenario), scenarioH.GetRun)
		api.POST("/scenario-runs/:id/complete", mw(actions.RunTestScenario), scenarioH.CompleteInteractiveRun)
		api.GET("/scenarios/otp/:acsTransID", mw(actions.RunTestScenario), scenarioH.GetOTP)
		api.GET("/scenarios/pin/:acsTransID", mw(actions.RunTestScenario), scenarioH.GetPIN)
	}

	return r
}
