package middleware

import (
	"log"
	"net/http"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware(sessionStore auth.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {

		sessionID, err := c.Cookie(auth.SessionCookieName)
		if err != nil {
			log.Printf("[DEBUG] No session cookie found: %v", err)

			c.Redirect(http.StatusFound, "/?redirect="+c.Request.URL.Path)
			c.Abort()
			return
		}

		userID, found, err := sessionStore.GetUserSession(sessionID)
		if err != nil {
			log.Printf("[ERROR] Error retrieving session: %v", err)
			c.Redirect(http.StatusFound, "/?redirect="+c.Request.URL.Path)
			c.Abort()
			return
		}

		if !found {
			log.Printf("[DEBUG] Session not found or expired")

			c.SetCookie(auth.SessionCookieName, "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/?redirect="+c.Request.URL.Path)
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

func OptionalAuthMiddleware(sessionStore auth.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie(auth.SessionCookieName)
		if err == nil {
			userID, found, err := sessionStore.GetUserSession(sessionID)
			if err == nil && found {
				c.Set("userID", userID)
				c.Set("authenticated", true)
			}
		}
		c.Next()
	}
}
