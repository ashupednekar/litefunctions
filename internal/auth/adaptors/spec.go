package adaptors

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"sync"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/go-webauthn/webauthn/webauthn"
)


type InMemoryStore struct {
	mu       sync.Mutex
	users    map[string]auth.PasskeyUser
	sessions map[string]webauthn.SessionData
}

func (i *InMemoryStore) GenSessionID() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(b), nil

}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		users:    make(map[string]auth.PasskeyUser),
		sessions: make(map[string]webauthn.SessionData),
	}
}

func (i *InMemoryStore) GetSession(token string) (webauthn.SessionData, bool) {
	log.Printf("[DEBUG] GetSession: %v", i.sessions[token])
	val, ok := i.sessions[token]
	return val, ok
}

func (i *InMemoryStore) SaveSession(token string, data webauthn.SessionData) {
	log.Printf("[DEBUG] SaveSession: %s - %v", token, data)
	i.sessions[token] = data
}

func (i *InMemoryStore) DeleteSession(token string) {
	log.Printf("[DEBUG] DeleteSession: %v", token)
	delete(i.sessions, token)
}

func (i *InMemoryStore) GetOrCreateUser(userName string) auth.PasskeyUser {
	log.Printf("[DEBUG] GetOrCreateUser: %v", userName)
	if _, ok := i.users[userName]; !ok {
		log.Printf("[DEBUG] GetOrCreateUser: creating new user: %v", userName)
		i.users[userName] = &auth.User{
			ID:          []byte(userName),
			DisplayName: userName,
			Name:        userName,
		}
	}

	return i.users[userName]
}

func (i *InMemoryStore) SaveUser(user auth.PasskeyUser) {
	log.Printf("[DEBUG] SaveUser: %v", user.WebAuthnName())
	log.Printf("[DEBUG] SaveUser: %v", user)
	i.users[user.WebAuthnName()] = user
}
