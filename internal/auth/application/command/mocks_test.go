//go:build unit

package command

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
	"github.com/viethung213/gym-companion/internal/auth/domain/event"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
)

// ---------------------------------------------------------------------------
// mockUserRepo
// ---------------------------------------------------------------------------

type mockUserRepo struct {
	users             map[string]*aggregate.User
	findByEmailErr    error
	findByGoogleIDErr error
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

func (m *mockUserRepo) FindByIdentity(ctx context.Context, identityType string, identifier string) (*aggregate.User, error) {
	if identityType == "google" && m.findByGoogleIDErr != nil {
		return nil, m.findByGoogleIDErr
	}
	if identityType == "email" && m.findByEmailErr != nil {
		return nil, m.findByEmailErr
	}
	for _, u := range m.users {
		if _, ok := u.FindIdentity(identityType, identifier); ok {
			return u, nil
		}
	}
	return nil, derror.ErrUserNotFound
}

func (m *mockUserRepo) List(ctx context.Context, filter repository.ListUsersFilter) ([]*aggregate.User, int, error) {
	var filtered []*aggregate.User
	for _, u := range m.users {
		if filter.Role != "" && !strings.EqualFold(u.Role(), filter.Role) {
			continue
		}
		if filter.Status != "" && !strings.EqualFold(u.Status(), filter.Status) {
			continue
		}
		if filter.Search != "" {
			term := strings.ToLower(filter.Search)
			nameMatch := strings.Contains(strings.ToLower(u.FullName()), term)
			identMatch := strings.Contains(strings.ToLower(u.Identity().Identifier()), term)
			if !nameMatch && !identMatch {
				continue
			}
		}
		filtered = append(filtered, u)
	}
	total := len(filtered)
	start := filter.Offset
	if start > total {
		start = total
	}
	end := start + filter.Limit
	if end > total {
		end = total
	}
	return filtered[start:end], total, nil
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

func (m *mockKeyRepo) DeactivateAllActiveKeys(ctx context.Context) error {
	for _, k := range m.keys {
		if k.Status == port.KeyStatusActive {
			k.Status = port.KeyStatusInactive
		}
	}
	return nil
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
		ID:            "9e0dc099-0df4-436f-b258-004ea10a6234",
		Email:         "oauth_user@example.com",
		FullName:      "OAuth User",
		EmailVerified: true,
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

// ---------------------------------------------------------------------------
// mockHasher
// ---------------------------------------------------------------------------

type mockHasher struct {
	hashPrefix string
}

func (h *mockHasher) Hash(raw string) (string, error) {
	return "hashed_" + raw, nil
}

func (h *mockHasher) Compare(hashed, raw string) error {
	if hashed == "hashed_"+raw || hashed == raw {
		return nil
	}
	return errors.New("hash mismatch")
}

// ---------------------------------------------------------------------------
// mockOTPRepo
// ---------------------------------------------------------------------------

type mockOTPRepo struct {
	otps map[string]*entity.OTP
}

func newMockOTPRepo() *mockOTPRepo {
	return &mockOTPRepo{otps: make(map[string]*entity.OTP)}
}

func (m *mockOTPRepo) Save(ctx context.Context, o *entity.OTP) error {
	m.otps[o.ID()] = o
	return nil
}

func (m *mockOTPRepo) FindByID(ctx context.Context, id string) (*entity.OTP, error) {
	o, ok := m.otps[id]
	if !ok {
		return nil, derror.ErrNotFound
	}
	return o, nil
}

func (m *mockOTPRepo) FindLatestActive(ctx context.Context, identifier string, purpose string) (*entity.OTP, error) {
	var latest *entity.OTP
	for _, o := range m.otps {
		if o.Identifier() == identifier && o.Purpose() == purpose && !o.IsUsed() {
			if latest == nil || o.CreatedAt().After(latest.CreatedAt()) {
				latest = o
			}
		}
	}
	return latest, nil
}

func (m *mockOTPRepo) Update(ctx context.Context, o *entity.OTP) error {
	m.otps[o.ID()] = o
	return nil
}

// ---------------------------------------------------------------------------
// mockPasswordResetTokenRepo
// ---------------------------------------------------------------------------

type mockPasswordResetTokenRepo struct {
	tokens map[string]*entity.PasswordResetToken
}

func newMockPasswordResetTokenRepo() *mockPasswordResetTokenRepo {
	return &mockPasswordResetTokenRepo{tokens: make(map[string]*entity.PasswordResetToken)}
}

func (m *mockPasswordResetTokenRepo) Save(ctx context.Context, token *entity.PasswordResetToken) error {
	m.tokens[token.Token()] = token
	return nil
}

func (m *mockPasswordResetTokenRepo) FindByToken(ctx context.Context, token string) (*entity.PasswordResetToken, error) {
	t, ok := m.tokens[token]
	if !ok {
		return nil, derror.ErrNotFound
	}
	return t, nil
}

func (m *mockPasswordResetTokenRepo) Update(ctx context.Context, token *entity.PasswordResetToken) error {
	m.tokens[token.Token()] = token
	return nil
}
