package postgres

import (
	"database/sql"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

// UserModel is the GORM model mapping to auth.users table.
type UserModel struct {
	ID         string    `gorm:"primaryKey;column:id"`
	Email      string    `gorm:"column:email;not null;uniqueIndex"`
	GoogleID   string    `gorm:"column:google_id"`
	FacebookID string    `gorm:"column:facebook_id"`
	FullName   string    `gorm:"column:full_name"`
	Role       string    `gorm:"column:role;not null"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (UserModel) TableName() string {
	return "auth.users"
}

func toUserModel(u *aggregate.User) *UserModel {
	return &UserModel{
		ID:         u.ID(),
		Email:      u.Email(),
		GoogleID:   u.GoogleID(),
		FacebookID: u.FacebookID(),
		FullName:   u.FullName(),
		Role:       u.Role(),
		CreatedAt:  u.CreatedAt(),
		UpdatedAt:  u.UpdatedAt(),
	}
}

func (m *UserModel) ToDomain() (*aggregate.User, error) {
	userID, err := vo.NewUserID(m.ID)
	if err != nil {
		return nil, err
	}
	email, err := vo.NewEmail(m.Email)
	if err != nil {
		return nil, err
	}
	role, err := vo.NewRole(m.Role)
	if err != nil {
		return nil, err
	}
	return aggregate.NewUser(
		userID,
		email,
		m.GoogleID,
		m.FacebookID,
		m.FullName,
		role,
		m.CreatedAt,
		m.UpdatedAt,
	), nil
}

// JSONWebKeyModel is the GORM model mapping to auth.jwk_keys table.
type JSONWebKeyModel struct {
	ID            string    `gorm:"primaryKey;column:id"`
	PrivateKeyPEM string    `gorm:"column:private_key_pem;not null"`
	PublicKeyPEM  string    `gorm:"column:public_key_pem;not null"`
	Algorithm     string    `gorm:"column:algorithm;not null"`
	Status        string    `gorm:"column:status;not null"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	ExpiresAt     time.Time `gorm:"column:expires_at"`
}

func (JSONWebKeyModel) TableName() string {
	return "auth.jwk_keys"
}

func toJSONWebKeyModel(k *port.JWKRecord) *JSONWebKeyModel {
	return &JSONWebKeyModel{
		ID:            k.ID,
		PrivateKeyPEM: k.PrivateKeyPEM,
		PublicKeyPEM:  k.PublicKeyPEM,
		Algorithm:     k.Algorithm,
		Status:        k.Status,
		CreatedAt:     k.CreatedAt,
		ExpiresAt:     k.ExpiresAt,
	}
}

func (m *JSONWebKeyModel) ToRepositoryRecord() *port.JWKRecord {
	return &port.JWKRecord{
		ID:            m.ID,
		PrivateKeyPEM: m.PrivateKeyPEM,
		PublicKeyPEM:  m.PublicKeyPEM,
		Algorithm:     m.Algorithm,
		Status:        m.Status,
		CreatedAt:     m.CreatedAt,
		ExpiresAt:     m.ExpiresAt,
	}
}

// SessionModel is the GORM model mapping to auth.sessions table.
type SessionModel struct {
	Token     string    `gorm:"primaryKey;column:token"`
	UserID    string    `gorm:"column:user_id;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
}

func (SessionModel) TableName() string {
	return "auth.sessions"
}

func (m *SessionModel) ToRepositoryRecord() *port.SessionRecord {
	return &port.SessionRecord{
		Token:     m.Token,
		UserID:    m.UserID,
		CreatedAt: m.CreatedAt,
		ExpiresAt: m.ExpiresAt,
	}
}

// OutboxModel is the GORM model mapping to auth.outbox table.
type OutboxModel struct {
	ID           string       `gorm:"primaryKey;column:id"`
	EventID      string       `gorm:"column:event_id;uniqueIndex;not null"`
	EventType    string       `gorm:"column:event_type;not null"`
	Payload      []byte       `gorm:"column:payload;not null;type:jsonb"`
	PartitionKey string       `gorm:"column:partition_key;not null"`
	CreatedAt    time.Time    `gorm:"column:created_at"`
	Published    bool         `gorm:"column:published;default:false"`
	PublishedAt  sql.NullTime `gorm:"column:published_at"`
}

func (OutboxModel) TableName() string {
	return "auth.outbox"
}

func (m *OutboxModel) ToRepositoryRecord() *port.OutboxRecord {
	return &port.OutboxRecord{
		ID:           m.ID,
		EventID:      m.EventID,
		EventType:    m.EventType,
		Payload:      m.Payload,
		PartitionKey: m.PartitionKey,
	}
}
