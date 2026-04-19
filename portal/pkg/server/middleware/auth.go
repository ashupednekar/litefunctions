package middleware

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"

	"github.com/ashupednekar/litefunctions/portal/internal/auth"
	"github.com/ashupednekar/litefunctions/portal/pkg"
	"github.com/gin-gonic/gin"
)

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func parseBearerToken(h string) (string, bool) {
	h = strings.TrimSpace(h)
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	tok := strings.TrimSpace(h[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
}

func authRequiredFailure(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	} else {
		c.Redirect(http.StatusFound, "/?redirect="+c.Request.URL.Path)
	}
	c.Abort()
}

// tryAPIBearerAuth handles Authorization: Bearer when API_TOKEN_ENABLED is set.
// Returns true if the request was fully handled (Next or Abort).
func tryAPIBearerAuth(c *gin.Context, store auth.PasskeyStore) bool {
	if !pkg.Cfg.APITokenEnabled || pkg.Cfg.APIToken == "" {
		return false
	}
	tok, hasBearer := parseBearerToken(c.GetHeader("Authorization"))
	if !hasBearer {
		return false
	}
	if !constantTimeEqual(tok, pkg.Cfg.APIToken) {
		log.Printf("[WARN] invalid API token attempt for %s", c.Request.URL.Path)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		c.Abort()
		return true
	}
	userName := pkg.Cfg.APITokenUser
	if userName == "" {
		userName = "system"
	}
	name, userID, err := store.GetUserByName(userName)
	if err != nil {
		log.Printf("[ERROR] API token user %q not found: %v", userName, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "api token user not found"})
		c.Abort()
		return true
	}
	c.Set("userID", userID)
	c.Set("userName", name)
	c.Next()
	return true
}

func AuthMiddleware(store auth.PasskeyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tryAPIBearerAuth(c, store) {
			return
		}

		sessionID, err := c.Cookie(auth.SessionCookieName)
		if err != nil {
			log.Printf("[DEBUG] No session cookie found: %v", err)
			authRequiredFailure(c)
			return
		}

		userName, userID, found, err := store.GetUserSession(sessionID)
		if err != nil {
			log.Printf("[ERROR] Error retrieving session: %v", err)
			authRequiredFailure(c)
			return
		}

		if !found {
			log.Printf("[DEBUG] Session not found or expired")
			c.SetCookie(auth.SessionCookieName, "", -1, "/", "", false, true)
			authRequiredFailure(c)
			return
		}

		c.Set("userID", userID)
		c.Set("userName", userName)
		c.Next()
	}
}

func OptionalAuthMiddleware(store auth.PasskeyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if pkg.Cfg.APITokenEnabled && pkg.Cfg.APIToken != "" {
			tok, ok := parseBearerToken(c.GetHeader("Authorization"))
			if ok && constantTimeEqual(tok, pkg.Cfg.APIToken) {
				userName := pkg.Cfg.APITokenUser
				if userName == "" {
					userName = "system"
				}
				if name, userID, err := store.GetUserByName(userName); err == nil {
					c.Set("userID", userID)
					c.Set("userName", name)
					c.Set("authenticated", true)
				}
				c.Next()
				return
			}
		}
		sessionID, err := c.Cookie(auth.SessionCookieName)
		if err == nil {
			userName, userID, found, err := store.GetUserSession(sessionID)
			if err == nil && found {
				c.Set("userID", userID)
				c.Set("userName", userName)
				c.Set("authenticated", true)
			}
		}
		c.Next()
	}
}
