package secrets

import (
	"github.com/gin-gonic/gin"
	"github.com/your-org/secrets-manager/internal/actions"
	"github.com/your-org/secrets-manager/internal/middleware"
	"go.uber.org/zap"
)

// NewRouter creates the Gin engine with secrets routes. The returned engine
// can be extended with additional route groups (e.g., HSM gateway routes).
func NewRouter(h *Handler, identityURL string, log *zap.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	keys := r.Group("/secrets/keys")
	{
		keys.GET("", middleware.RequireAction(identityURL, actions.ReadKeys, log), h.ListKeys)
		keys.GET("/:id", middleware.RequireAction(identityURL, actions.ReadKeys, log), h.GetKey)
		keys.POST("/generate", middleware.RequireAction(identityURL, actions.WriteKeys, log), h.GenerateKey)
		keys.POST("/import", middleware.RequireAction(identityURL, actions.WriteKeys, log), h.ImportKey)
		keys.POST("/certificate", middleware.RequireAction(identityURL, actions.WriteKeys, log), h.GenerateCertificate)
		keys.PUT("/:id", middleware.RequireAction(identityURL, actions.WriteKeys, log), h.UpdateKey)
		keys.DELETE("/:id", middleware.RequireAction(identityURL, actions.WriteKeys, log), h.DeleteKey)
		keys.POST("/:id/rotate", middleware.RequireAction(identityURL, actions.WriteKeys, log), h.RotateKey)
		keys.GET("/:id/public", middleware.RequireAction(identityURL, actions.UseKey, log), h.ExportPublicKey)
		keys.POST("/:id/export", middleware.RequireAction(identityURL, actions.WriteKeys, log), h.ExportKey)
		keys.GET("/:id/usage", middleware.RequireAction(identityURL, actions.ReadKeyUsageLog, log), h.GetKeyUsageLog)
		keys.POST("/:id/policy", middleware.RequireAction(identityURL, actions.WriteKeys, log), h.AssignPolicy)
		keys.GET("/:id/access", middleware.RequireAction(identityURL, actions.ManageKeyAccess, log), h.ListKeyAccess)
		keys.POST("/:id/access", middleware.RequireAction(identityURL, actions.ManageKeyAccess, log), h.GrantKeyAccess)
		keys.DELETE("/:id/access/:userId", middleware.RequireAction(identityURL, actions.ManageKeyAccess, log), h.RevokeKeyAccess)
	}

	policies := r.Group("/secrets/rotation-policies")
	{
		policies.GET("", middleware.RequireAction(identityURL, actions.ReadRotationPolicies, log), h.ListPolicies)
		policies.POST("", middleware.RequireAction(identityURL, actions.WriteRotationPolicies, log), h.CreatePolicy)
		policies.PUT("/:id", middleware.RequireAction(identityURL, actions.WriteRotationPolicies, log), h.UpdatePolicy)
		policies.DELETE("/:id", middleware.RequireAction(identityURL, actions.WriteRotationPolicies, log), h.DeletePolicy)
	}

	providers := r.Group("/secrets/providers")
	{
		providers.GET("", middleware.RequireAction(identityURL, actions.ReadProviders, log), h.ListProviders)
		providers.GET("/:id", middleware.RequireAction(identityURL, actions.ReadProviders, log), h.GetProvider)
		providers.POST("", middleware.RequireAction(identityURL, actions.WriteProviders, log), h.CreateProvider)
		providers.PUT("/:id", middleware.RequireAction(identityURL, actions.WriteProviders, log), h.UpdateProvider)
		providers.DELETE("/:id", middleware.RequireAction(identityURL, actions.WriteProviders, log), h.DeleteProvider)
		providers.POST("/:id/test", middleware.RequireAction(identityURL, actions.WriteProviders, log), h.TestProvider)
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
