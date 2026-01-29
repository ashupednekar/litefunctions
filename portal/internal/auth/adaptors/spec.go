package adaptors

import (
	"log"

	"github.com/ashupednekar/litefunctions/portal/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WebauthnStore struct {
	queries *Queries
}

func NewWebauthnStore(pool *pgxpool.Pool) *WebauthnStore {
	return &WebauthnStore{queries: New(pool)}
}

func (db *WebauthnStore) GetOrCreateUser(userName string) (auth.PasskeyUser, error) {
	existingUser, err := db.GetUser(userName)
	if err == nil {
		log.Printf("found existing user: %v\n", existingUser)
		return existingUser, nil
	}
	return db.CreateUser(userName)
}
