//go:build unit

package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

func TestLogoutHandler(t *testing.T) {
	ctx := context.Background()
	sessRepo := &mockSessionRepo{sessions: make(map[string]*port.SessionRecord)}

	userID := "9e0dc099-0df4-436f-b258-004ea10a6234"
	token := "active-session-token"
	_ = sessRepo.Save(ctx, token, userID, time.Now().Add(1*time.Hour))

	handler := NewLogoutHandler(sessRepo)
	err := handler.Handle(ctx, LogoutCommand{
		RefreshToken: token,
		UserID:       userID,
	})
	if err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	// Session must be deleted
	_, err = sessRepo.FindByToken(ctx, token)
	if !errors.Is(err, apperror.ErrSessionNotFound) {
		t.Error("expected session to be deleted after logout")
	}
}

func TestLogoutHandler_BOLA_Failure(t *testing.T) {
	ctx := context.Background()
	sessRepo := &mockSessionRepo{sessions: make(map[string]*port.SessionRecord)}

	userID := "9e0dc099-0df4-436f-b258-004ea10a6234"
	token := "active-session-token"
	_ = sessRepo.Save(ctx, token, userID, time.Now().Add(1*time.Hour))

	handler := NewLogoutHandler(sessRepo)
	
	// Try logout with mismatched user id (BOLA)
	err := handler.Handle(ctx, LogoutCommand{
		RefreshToken: token,
		UserID:       "wrong-user-id",
	})
	if !errors.Is(err, apperror.ErrUnauthorized) {
		t.Errorf("got error %v, expected %v", err, apperror.ErrUnauthorized)
	}

	// Session must NOT be deleted
	_, err = sessRepo.FindByToken(ctx, token)
	if err != nil {
		t.Errorf("expected session to still exist, got err: %v", err)
	}
}
