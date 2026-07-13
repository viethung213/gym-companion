//go:build unit

package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/event"
)

func TestOAuthLoginHandler_RegisterNew(t *testing.T) {
	ctx := context.Background()

	userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
	key := &port.JWKRecord{
		ID:            "active-kid",
		PrivateKeyPEM: "priv-pem",
		PublicKeyPEM:  "pub-pem",
		Algorithm:     "RS256",
		Status:        port.KeyStatusActive,
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}
	keyRepo := &mockKeyRepo{keys: []*port.JWKRecord{key}}
	sessRepo := &mockSessionRepo{sessions: make(map[string]*port.SessionRecord)}
	publisher := &mockEventPublisher{}

	handler := NewOAuthLoginHandler(
		userRepo,
		keyRepo,
		sessRepo,
		mockTokenService{},
		mockOAuthService{},
		publisher,
		&mockTxManager{},
	)

	accessToken, refreshToken, userID, err := handler.Handle(ctx, OAuthLoginCommand{
		Provider: "google",
		Code:     "valid_code",
	})
	if err != nil {
		t.Fatalf("unexpected login failure: %v", err)
	}

	if accessToken != "mock-access-token" || refreshToken != "mock-refresh-token" {
		t.Errorf("unexpected tokens: access=%s, refresh=%s", accessToken, refreshToken)
	}

	// Verify user was created
	createdUser, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("user not found: %v", err)
	}
	if got, want := createdUser.Email(), "oauth_user@example.com"; got != want {
		t.Errorf("got email %s, want %s", got, want)
	}

	// Verify session was saved
	sess, err := sessRepo.FindByToken(ctx, refreshToken)
	if err != nil {
		t.Fatalf("session not found: %v", err)
	}
	if sess.UserID != userID {
		t.Errorf("got session user %s, want %s", sess.UserID, userID)
	}

	// Verify domain event was published
	if got, want := len(publisher.events), 1; got != want {
		t.Fatalf("got %d events, want %d", got, want)
	}
	regEvent, ok := publisher.events[0].(event.UserRegisteredEvent)
	if !ok {
		t.Fatalf("expected UserRegisteredEvent, got %T", publisher.events[0])
	}
	if regEvent.UserID != userID || regEvent.Email != "oauth_user@example.com" {
		t.Errorf("unexpected event data: %+v", regEvent)
	}
}

func TestOAuthLoginHandler_UnsupportedProvider(t *testing.T) {
	ctx := context.Background()
	handler := NewOAuthLoginHandler(nil, nil, nil, nil, nil, nil, nil)

	_, _, _, err := handler.Handle(ctx, OAuthLoginCommand{
		Provider: "github",
		Code:     "code",
	})
	if err == nil {
		t.Error("expected error for unsupported oauth provider")
	}
}

type mockFailingCommitTxManager struct{}

func (m *mockFailingCommitTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := fn(ctx); err != nil {
		return err
	}
	return errors.New("database commit failed")
}

func TestOAuthLoginHandler_RegisterNew_CommitFailure(t *testing.T) {
	ctx := context.Background()

	userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
	key := &port.JWKRecord{
		ID:            "active-kid",
		PrivateKeyPEM: "priv-pem",
		PublicKeyPEM:  "pub-pem",
		Algorithm:     "RS256",
		Status:        port.KeyStatusActive,
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}
	keyRepo := &mockKeyRepo{keys: []*port.JWKRecord{key}}
	sessRepo := &mockSessionRepo{sessions: make(map[string]*port.SessionRecord)}
	publisher := &mockEventPublisher{}

	handler := NewOAuthLoginHandler(
		userRepo,
		keyRepo,
		sessRepo,
		mockTokenService{},
		mockOAuthService{},
		publisher,
		&mockFailingCommitTxManager{},
	)

	_, _, _, err := handler.Handle(ctx, OAuthLoginCommand{
		Provider: "google",
		Code:     "valid_code",
	})
	if err == nil {
		t.Fatal("expected error due to commit failure, got nil")
	}

	// Verify that user was created in the mock repo during the transaction block,
	// but the domain events were NOT cleared because the commit failed.
	var createdUser *aggregate.User
	for _, u := range userRepo.users {
		createdUser = u
		break
	}
	if createdUser == nil {
		t.Fatal("expected user to be created in mock repo during transaction run")
	}

	if got, want := len(createdUser.DomainEvents()), 1; got != want {
		t.Errorf("got %d domain events, want %d (events should not be cleared on commit failure)", got, want)
	}
}

func TestOAuthLoginHandler_StateValidationFailure(t *testing.T) {
	ctx := context.Background()

	handler := NewOAuthLoginHandler(
		&mockUserRepo{},
		&mockKeyRepo{},
		&mockSessionRepo{},
		mockTokenService{},
		mockOAuthService{},
		&mockEventPublisher{},
		&mockTxManager{},
	)

	_, _, _, err := handler.Handle(ctx, OAuthLoginCommand{
		Provider: "google",
		Code:     "valid_code",
		State:    "invalid-state-value",
	})
	if err == nil {
		t.Fatal("expected error due to state validation failure, got nil")
	}

	if !strings.Contains(err.Error(), "oauth state validation failed") {
		t.Errorf("got error %v, expected it to contain 'oauth state validation failed'", err)
	}
}
