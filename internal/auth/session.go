package auth

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/ashupednekar/litewebservices-portal/pkg"
)

const SessionCookieName = "lws_session"

// GenerateSessionID creates a cryptographically secure random session ID
func GenerateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GetSessionExpiry returns the expiration time for a new session
func GetSessionExpiry() (time.Time, error) {
	duration, err := time.ParseDuration(pkg.Cfg.SessionExpiry)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().Add(duration), nil
}

// SessionStore defines the interface for session management
type SessionStore interface {
	CreateUserSession(userID []byte, sessionID string, expiresAt time.Time, userAgent, ipAddress string) error
	GetUserSession(sessionID string) (userName string, userID []byte, found bool, err error)
	DeleteUserSession(sessionID string) error
	DeleteAllUserSessions(userID []byte) error
}
