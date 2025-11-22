package adaptors

import (
	"log"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WebauthnStore struct {
	queries *Queries
}

func NewWebauthnStore(pool *pgxpool.Pool) *WebauthnStore {
	return &WebauthnStore{queries: New(pool)}
}

type Credential struct {
	ID              []byte
	UserID          []byte
	PublicKey       []byte
	AttestationType pgtype.Text
	Aaguid          []byte
	SignCount       int64
	Transports      []string
	Flags           int32
	CreatedAt       pgtype.Timestamptz
	UpdatedAt       pgtype.Timestamptz
}

type User struct {
	ID          []byte
	Name        string
	DisplayName string
	Icon        pgtype.Text
}

type WebauthnSession struct {
	SessionID          string
	UserName           string
	Challenge          []byte
	UserID             []byte
	AllowedCredentials [][]byte
	ExpiresAt          pgtype.Timestamptz
}

func (db *WebauthnStore) GetOrCreateUser(userName string) (auth.PasskeyUser, error) {
	log.Printf("[DEBUG] GetOrCreateUser: %v", userName)

	existingUser, err := db.GetUser(userName)
	if err == nil {
		return existingUser, nil
	}
	return db.CreateUser(userName)
}
