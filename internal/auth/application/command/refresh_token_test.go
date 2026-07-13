//go:build unit

package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
)

func TestRefreshTokenHandler_Handle(t *testing.T) {
	ctx := context.Background()

	// Setup user
	userID := "e3fa878c-02cf-4b72-9118-8f8319fbc74b"
	user, err := aggregate.RegisterUser(userID, "test@example.com", "Test User", "user")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	t.Run("Success", func(t *testing.T) {
		userRepo := &mockUserRepo{users: map[string]*aggregate.User{userID: user}}
		keyRepo := &mockKeyRepo{
			keys: []*port.JWKRecord{
				{ID: "key-1", Status: port.KeyStatusActive, ExpiresAt: time.Now().Add(24 * time.Hour)},
			},
		}
		sessRepo := &mockSessionRepo{
			sessions: map[string]*port.SessionRecord{
				"valid-token": {
					Token:     "valid-token",
					UserID:    userID,
					ExpiresAt: time.Now().Add(24 * time.Hour),
				},
			},
		}
		tokenServ := mockTokenService{}

		handler := NewRefreshTokenHandler(userRepo, keyRepo, sessRepo, tokenServ)
		res, err := handler.Handle(ctx, RefreshTokenCommand{RefreshToken: "valid-token"})
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}

		if res.AccessToken != "mock-access-token" {
			t.Errorf("got access token %s, want %s", res.AccessToken, "mock-access-token")
		}
		if res.RefreshToken != "valid-token" {
			t.Errorf("got refresh token %s, want %s", res.RefreshToken, "valid-token")
		}
	})

	t.Run("InvalidToken_SessionNotFound", func(t *testing.T) {
		userRepo := &mockUserRepo{users: map[string]*aggregate.User{userID: user}}
		keyRepo := &mockKeyRepo{
			keys: []*port.JWKRecord{
				{ID: "key-1", Status: port.KeyStatusActive, ExpiresAt: time.Now().Add(24 * time.Hour)},
			},
		}
		sessRepo := &mockSessionRepo{sessions: map[string]*port.SessionRecord{}}
		tokenServ := mockTokenService{}

		handler := NewRefreshTokenHandler(userRepo, keyRepo, sessRepo, tokenServ)
		_, err := handler.Handle(ctx, RefreshTokenCommand{RefreshToken: "invalid-token"})
		if !errors.Is(err, apperror.ErrUnauthorized) {
			t.Errorf("got error %v, want %v", err, apperror.ErrUnauthorized)
		}
	})

	t.Run("ExpiredToken_ShouldDeleteSessionAndFail", func(t *testing.T) {
		userRepo := &mockUserRepo{users: map[string]*aggregate.User{userID: user}}
		keyRepo := &mockKeyRepo{
			keys: []*port.JWKRecord{
				{ID: "key-1", Status: port.KeyStatusActive, ExpiresAt: time.Now().Add(24 * time.Hour)},
			},
		}
		sessRepo := &mockSessionRepo{
			sessions: map[string]*port.SessionRecord{
				"expired-token": {
					Token:     "expired-token",
					UserID:    userID,
					ExpiresAt: time.Now().Add(-1 * time.Hour), // expired
				},
			},
		}
		tokenServ := mockTokenService{}

		handler := NewRefreshTokenHandler(userRepo, keyRepo, sessRepo, tokenServ)
		_, err := handler.Handle(ctx, RefreshTokenCommand{RefreshToken: "expired-token"})
		if !errors.Is(err, apperror.ErrUnauthorized) {
			t.Errorf("got error %v, want %v", err, apperror.ErrUnauthorized)
		}

		// Verify session was deleted from repository
		_, err = sessRepo.FindByToken(ctx, "expired-token")
		if !errors.Is(err, apperror.ErrSessionNotFound) {
			t.Errorf("expected session to be deleted, find returned: %v", err)
		}
	})

	t.Run("UserNotFound", func(t *testing.T) {
		userRepo := &mockUserRepo{users: map[string]*aggregate.User{}} // empty
		keyRepo := &mockKeyRepo{
			keys: []*port.JWKRecord{
				{ID: "key-1", Status: port.KeyStatusActive, ExpiresAt: time.Now().Add(24 * time.Hour)},
			},
		}
		sessRepo := &mockSessionRepo{
			sessions: map[string]*port.SessionRecord{
				"valid-token": {
					Token:     "valid-token",
					UserID:    userID,
					ExpiresAt: time.Now().Add(24 * time.Hour),
				},
			},
		}
		tokenServ := mockTokenService{}

		handler := NewRefreshTokenHandler(userRepo, keyRepo, sessRepo, tokenServ)
		_, err := handler.Handle(ctx, RefreshTokenCommand{RefreshToken: "valid-token"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, derror.ErrUserNotFound) {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("ActiveKeyNotFound", func(t *testing.T) {
		userRepo := &mockUserRepo{users: map[string]*aggregate.User{userID: user}}
		keyRepo := &mockKeyRepo{keys: []*port.JWKRecord{}} // empty
		sessRepo := &mockSessionRepo{
			sessions: map[string]*port.SessionRecord{
				"valid-token": {
					Token:     "valid-token",
					UserID:    userID,
					ExpiresAt: time.Now().Add(24 * time.Hour),
				},
			},
		}
		tokenServ := mockTokenService{}

		handler := NewRefreshTokenHandler(userRepo, keyRepo, sessRepo, tokenServ)
		_, err := handler.Handle(ctx, RefreshTokenCommand{RefreshToken: "valid-token"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
