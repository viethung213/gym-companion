package postgres

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

// RoleModel is the GORM model mapping to auth.roles table.
type RoleModel struct {
	ID          string    `gorm:"primaryKey;column:id"`
	Name        string    `gorm:"column:name;not null"`
	Description string    `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (RoleModel) TableName() string {
	return "auth.roles"
}

// UserModel is the GORM model mapping to auth.users table.
type UserModel struct {
	ID        string             `gorm:"primaryKey;column:id"`
	FullName  string             `gorm:"column:full_name"`
	RoleID    string             `gorm:"column:role_id;not null;index:idx_users_role_id"`
	Status    string             `gorm:"column:status;not null;default:active"`
	CreatedAt time.Time          `gorm:"column:created_at"`
	UpdatedAt time.Time          `gorm:"column:updated_at"`
	Identity  *UserIdentityModel `gorm:"foreignKey:UserID;references:ID"`
}

func (UserModel) TableName() string {
	return "auth.users"
}

// UserIdentityModel is the GORM model mapping to auth.user_identities table.
type UserIdentityModel struct {
	ID             string    `gorm:"primaryKey;column:id"`
	UserID         string    `gorm:"column:user_id;not null;uniqueIndex:idx_identities_user_id"`
	IdentityType   string    `gorm:"column:identity_type;not null"`
	Identifier     string    `gorm:"column:identifier;not null"`
	CredentialData *string   `gorm:"column:credential_data"`
	Metadata       []byte    `gorm:"column:metadata;type:jsonb"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (UserIdentityModel) TableName() string {
	return "auth.user_identities"
}

func toUserModel(u *aggregate.User) *UserModel {
	var identModel *UserIdentityModel
	if u.Identity().ID() != "" {
		m := toUserIdentityModel(u.ID(), u.Identity())
		identModel = &m
	}

	return &UserModel{
		ID:        u.ID(),
		FullName:  u.FullName(),
		RoleID:    u.Role(),
		Status:    u.Status(),
		CreatedAt: u.CreatedAt(),
		UpdatedAt: u.UpdatedAt(),
		Identity:  identModel,
	}
}

func toUserIdentityModel(userID string, i aggregate.Identity) UserIdentityModel {
	var cred *string
	if i.CredentialData() != "" {
		c := i.CredentialData()
		cred = &c
	}
	return UserIdentityModel{
		ID:             i.ID(),
		UserID:         userID,
		IdentityType:   i.IdentityType(),
		Identifier:     i.Identifier(),
		CredentialData: cred,
		Metadata:       i.Metadata(),
		CreatedAt:      i.CreatedAt(),
		UpdatedAt:      i.UpdatedAt(),
	}
}

func (m *UserModel) ToDomain() (*aggregate.User, error) {
	role, err := vo.NewRole(m.RoleID)
	if err != nil {
		return nil, fmt.Errorf("invalid role in database: %w", err)
	}

	var identity aggregate.Identity
	if m.Identity != nil && m.Identity.ID != "" {
		var cred string
		if m.Identity.CredentialData != nil {
			cred = *m.Identity.CredentialData
		}
		identity = aggregate.NewIdentity(
			m.Identity.ID,
			m.Identity.IdentityType,
			m.Identity.Identifier,
			cred,
			m.Identity.Metadata,
			m.Identity.CreatedAt,
			m.Identity.UpdatedAt,
		)
	}

	return aggregate.NewUser(
		m.ID,
		m.FullName,
		role,
		m.Status,
		identity,
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
	UserID    string    `gorm:"column:user_id;not null;index:idx_sessions_user_id"`
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
	CreatedAt    time.Time    `gorm:"column:created_at;index:idx_outbox_published_created,priority:2"`
	Published    bool         `gorm:"column:published;default:false;index:idx_outbox_published_created,priority:1"`
	PublishedAt  sql.NullTime `gorm:"column:published_at"`
	Status       string       `gorm:"column:status;default:PENDING"`
	LockedUntil  sql.NullTime `gorm:"column:locked_until"`
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

// OTPModel is the GORM model mapping to auth.otps table.
type OTPModel struct {
	ID                string    `gorm:"primaryKey;column:id"` // otp_token
	Identifier        string    `gorm:"column:identifier;not null"`
	OTPHash           string    `gorm:"column:otp_hash;not null"`
	Purpose           string    `gorm:"column:purpose;not null"`
	Attempts          int       `gorm:"column:attempts;not null;default:0"`
	MaxAttempts       int       `gorm:"column:max_attempts;not null;default:5"`
	ExpiresAt         time.Time `gorm:"column:expires_at;not null"`
	ResendAvailableAt time.Time `gorm:"column:resend_available_at;not null"`
	IsUsed            bool      `gorm:"column:is_used;not null;default:false"`
	CreatedAt         time.Time `gorm:"column:created_at;not null"`
}

func (OTPModel) TableName() string {
	return "auth.otps"
}

// PasswordResetTokenModel is the GORM model mapping to auth.password_reset_tokens table.
type PasswordResetTokenModel struct {
	Token     string    `gorm:"primaryKey;column:token"`
	UserID    string    `gorm:"column:user_id;not null;index:idx_password_reset_tokens_user_id"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null"`
	IsUsed    bool      `gorm:"column:is_used;not null;default:false"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (PasswordResetTokenModel) TableName() string {
	return "auth.password_reset_tokens"
}
