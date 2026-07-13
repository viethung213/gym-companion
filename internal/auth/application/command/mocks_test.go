//go:build unit

package command

import (
	"context"
	"errors"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/event"
)

// ---------------------------------------------------------------------------
// mockUserRepo
// ---------------------------------------------------------------------------

type mockUserRepo struct {
	users map[string]*aggregate.User
}

func (m *mockUserRepo) Create(ctx context.Context, u *aggregate.User) error {
	m.users[u.ID()] = u
	return nil
}

func (m *mockUserRepo) Update(ctx context.Context, u *aggregate.User) error {
	if _, ok := m.users[u.ID()]; !ok {
		return derror.ErrUserNotFound
	}
	m.users[u.ID()] = u
	return nil
}

func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*aggregate.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, derror.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*aggregate.User, error) {
	for _, u := range m.users {
		if u.Email() == email {
			return u, nil
		}
	}
	return nil, derror.ErrUserNotFound
}

func (m *mockUserRepo) FindByGoogleID(ctx context.Context, googleID string) (*aggregate.User, error) {
	for _, u := range m.users {
		if u.GoogleID() == googleID {
			return u, nil
		}
	}
	return nil, derror.ErrUserNotFound
}

func (m *mockUserRepo) FindByFacebookID(ctx context.Context, facebookID string) (*aggregate.User, error) {
	for _, u := range m.users {
		if u.FacebookID() == facebookID {
			return u, nil
		}
	}
	return nil, derror.ErrUserNotFound
}

// ---------------------------------------------------------------------------
// mockKeyRepo
// ---------------------------------------------------------------------------

type mockKeyRepo struct {
	keys []*port.JWKRecord
}

func (m *mockKeyRepo) Save(ctx context.Context, key *port.JWKRecord) error {
	m.keys = append(m.keys, key)
	return nil
}

func (m *mockKeyRepo) GetActiveKey(ctx context.Context) (*port.JWKRecord, error) {
	for _, k := range m.keys {
		if k.Status == port.KeyStatusActive {
			return k, nil
		}
	}
	return nil, apperror.ErrKeyNotFound
}

func (m *mockKeyRepo) GetAllActiveAndInactiveKeys(ctx context.Context) ([]*port.JWKRecord, error) {
	return m.keys, nil
}

func (m *mockKeyRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	for _, k := range m.keys {
		if k.ID == id {
			k.Status = status
			return nil
		}
	}
	return apperror.ErrKeyNotFound
}

func (m *mockKeyRepo) DeleteExpiredKeys(ctx context.Context) error {
	return nil
}

// ---------------------------------------------------------------------------
// mockSessionRepo
// ---------------------------------------------------------------------------

type mockSessionRepo struct {
	sessions map[string]*port.SessionRecord
}

func (m *mockSessionRepo) Save(ctx context.Context, token string, userID string, expiresAt time.Time) error {
	m.sessions[token] = &port.SessionRecord{
		Token:     token,
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}
	return nil
}

func (m *mockSessionRepo) FindByToken(ctx context.Context, token string) (*port.SessionRecord, error) {
	s, ok := m.sessions[token]
	if !ok {
		return nil, apperror.ErrSessionNotFound
	}
	return s, nil
}

func (m *mockSessionRepo) Delete(ctx context.Context, token string) error {
	delete(m.sessions, token)
	return nil
}

func (m *mockSessionRepo) DeleteAllByUserID(ctx context.Context, userID string) error {
	for k, v := range m.sessions {
		if v.UserID == userID {
			delete(m.sessions, k)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// mockTokenService
// ---------------------------------------------------------------------------

type mockTokenService struct{}

func (mockTokenService) GenerateAccessToken(ctx context.Context, user *aggregate.User, kid string) (string, time.Time, error) {
	return "mock-access-token", time.Now().Add(15 * time.Minute), nil
}

func (mockTokenService) GenerateRefreshToken(ctx context.Context, user *aggregate.User) (string, time.Time, error) {
	return "mock-refresh-token", time.Now().Add(24 * time.Hour), nil
}

// ---------------------------------------------------------------------------
// mockOAuthService
// ---------------------------------------------------------------------------

type mockOAuthService struct{}

func (mockOAuthService) GetOAuthLoginURL(ctx context.Context, provider string, state string, redirectURI string) (string, error) {
	return "https://mock-oauth.com/auth", nil
}

func (mockOAuthService) ExchangeCodeForProfile(ctx context.Context, provider string, code string, redirectURI string) (*port.OAuthUserProfile, error) {
	return &port.OAuthUserProfile{
		ID:       "9e0dc099-0df4-436f-b258-004ea10a6234",
		Email:    "oauth_user@example.com",
		FullName: "OAuth User",
	}, nil
}

func (mockOAuthService) GenerateState(ctx context.Context) (string, error) {
	return "mock-state-value", nil
}

func (mockOAuthService) ValidateState(ctx context.Context, state string) error {
	if state != "mock-state-value" && state != "valid_state" && state != "" {
		return errors.New("invalid mock state")
	}
	return nil
}

// ---------------------------------------------------------------------------
// mockKeyGenerator
// ---------------------------------------------------------------------------

type mockKeyGenerator struct {
	privPEM string
	pubPEM  string
}

func (m *mockKeyGenerator) Generate(ctx context.Context) (string, string, error) {
	return m.privPEM, m.pubPEM, nil
}

// ---------------------------------------------------------------------------
// mockEventPublisher
// ---------------------------------------------------------------------------

type mockEventPublisher struct {
	events []event.DomainEvent
}

func (m *mockEventPublisher) Write(ctx context.Context, ev event.DomainEvent) error {
	m.events = append(m.events, ev)
	return nil
}

// ---------------------------------------------------------------------------
// mockTxManager
// ---------------------------------------------------------------------------

type mockTxManager struct{}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
