package port

import (
	"context"
	"time"
)

const (
	KeyStatusActive   = "active"
	KeyStatusInactive = "inactive"
	KeyStatusRetired  = "retired"
)

// KeyGenerator defines the cryptographic utility port for generating RSA-2048 key pairs.
type KeyGenerator interface {
	Generate(ctx context.Context) (privateKeyPEM string, publicKeyPEM string, err error)
}

// JWKRecord is a plain data transfer object representing an RSA key pair log entry.
type JWKRecord struct {
	ID            string
	PrivateKeyPEM string
	PublicKeyPEM  string
	Algorithm     string
	Status        string
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// KeyRepository defines the persistence port for JSONWebKey entities.
type KeyRepository interface {
	Save(ctx context.Context, key *JWKRecord) error
	GetActiveKey(ctx context.Context) (*JWKRecord, error)
	GetAllActiveAndInactiveKeys(ctx context.Context) ([]*JWKRecord, error)
	UpdateStatus(ctx context.Context, id string, status string) error
	DeleteExpiredKeys(ctx context.Context) error
}
