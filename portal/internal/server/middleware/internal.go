package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// InternalToken guards routes used by other cluster services. An empty token
// leaves the route open, which is the default for cluster-internal installs.
func InternalToken(token string) gin.HandlerFunc {
	expected := strings.TrimSpace(token)
	return func(c *gin.Context) {
		if expected == "" {
			c.Next()
			return
		}
		provided := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		if provided == "" {
			provided = strings.TrimSpace(c.GetHeader("X-MCP-Token"))
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
