package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequireAction returns a middleware that verifies the caller's JWT token
// against the Identity Server for the given action.
// If identityURL is empty, the middleware is a no-op (dev/test bypass).
func RequireAction(identityURL, action string, log *zap.Logger) gin.HandlerFunc {
	if identityURL == "" {
		return func(c *gin.Context) { c.Next() }
	}

	client := &http.Client{Timeout: 5 * time.Second}
	verifyURL := identityURL + "/auth/verify"

	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if len(auth) < 8 || auth[:7] != "Bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		token := auth[7:]

		body, _ := json.Marshal(map[string]string{"token": token, "action": action})
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, verifyURL, bytes.NewReader(body))
		if err != nil {
			log.Error("auth verify request build failed", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "auth service error"})
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Error("auth verify request failed", zap.Error(err), zap.String("url", verifyURL))
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "auth service unavailable"})
			return
		}
		defer resp.Body.Close()

		var result struct {
			Allowed bool   `json:"allowed"`
			User    string `json:"user"`
			UserID  int    `json:"userId"`
			Error   string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "auth response parse error"})
			return
		}

		if resp.StatusCode == http.StatusUnauthorized {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "detail": result.Error})
			return
		}

		if !result.Allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":  "forbidden",
				"action": action,
				"detail": fmt.Sprintf("user %q lacks permission %q", result.User, action),
			})
			return
		}

		c.Set("authUser", result.User)
		c.Set("authUserID", result.UserID)
		c.Next()
	}
}
