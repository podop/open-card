package identity

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// NewRouter creates the Gin engine for the Identity Server.
func NewRouter(h *Handler, log *zap.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	auth := r.Group("/auth")
	{
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
		auth.POST("/verify", h.Verify)
		auth.GET("/me", h.Me)
		auth.POST("/logout", h.Logout)

		auth.GET("/settings", h.GetSettings)
		auth.PUT("/settings", h.PutSettings)

		auth.GET("/groups", h.ListGroups)
		auth.POST("/groups", h.CreateGroup)
		auth.PUT("/groups/:id", h.UpdateGroup)
		auth.DELETE("/groups/:id", h.DeleteGroup)

		auth.GET("/users", h.ListUsers)
		auth.POST("/users", h.CreateUser)
		auth.PUT("/users/:id", h.UpdateUser)
		auth.DELETE("/users/:id", h.DeleteUser)
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
