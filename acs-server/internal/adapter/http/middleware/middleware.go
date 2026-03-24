package middleware

import (
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestLogger returns a Gin middleware that logs each request with zap.
func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("clientIP", c.ClientIP()),
		)
	}
}

// EnforceContentType returns a middleware that rejects POST requests whose
// Content-Type media type is not in the allowedTypes list.
// If the Content-Type header includes a charset parameter it must be utf-8.
// Returns 415 Unsupported Media Type on violation.
func EnforceContentType(allowedTypes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		raw := c.GetHeader("Content-Type")
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
				"errorCode":        "202",
				"errorDescription": "Content-Type header is required",
			})
			return
		}

		mediaType, params, err := mime.ParseMediaType(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
				"errorCode":        "202",
				"errorDescription": "invalid Content-Type header",
			})
			return
		}

		// EMVCo: if charset parameter is present it must be UTF-8.
		if charset, ok := params["charset"]; ok && strings.ToLower(charset) != "utf-8" {
			c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
				"errorCode":        "202",
				"errorDescription": "charset must be UTF-8",
			})
			return
		}

		for _, allowed := range allowedTypes {
			if mediaType == allowed {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
			"errorCode":        "202",
			"errorDescription": "Content-Type must be one of: " + strings.Join(allowedTypes, ", "),
		})
	}
}

// CORS returns a middleware that adds permissive CORS headers for the admin UI.
// Only use this on /api/* routes.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// MTLSClientAuth is a placeholder for mutual TLS enforcement.
// In production, configure this at the TLS layer (Nginx/Envoy) or via Go's tls.Config.
func MTLSClientAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: verify c.Request.TLS.PeerCertificates against DS certificate
		c.Next()
	}
}
