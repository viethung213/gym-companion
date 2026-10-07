//go:build unit

package aggregate_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/event"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

func TestRegisterUser_Success(t *testing.T) {
	userID := "9e0dc099-0df4-436f-b258-004ea10a6234"
	emailStr := "test@example.com"
	fullName := "John Doe"
	now := time.Now()
	ident := aggregate.NewIdentity("ident-1", aggregate.IdentityTypeEmail, emailStr, "hash", nil, now, now)

	user := aggregate.RegisterUser(userID, fullName, ident, "https://example.com/avatar.jpg")

	if got, want := user.ID(), userID; got != want {
		t.Errorf("got ID %s, want %s", got, want)
	}
	if got, want := user.FullName(), fullName; got != want {
		t.Errorf("got full name %s, want %s", got, want)
	}
	if got, want := user.Role(), "user"; got != want {
		t.Errorf("got role %s, want %s", got, want)
	}
	if got, want := user.Status(), aggregate.UserStatusActive; got != want {
		t.Errorf("got status %s, want %s", got, want)
	}
	if got, want := user.Identity().Identifier(), emailStr; got != want {
		t.Errorf("got identity identifier %s, want %s", got, want)
	}

	if user.CreatedAt().IsZero() || user.UpdatedAt().IsZero() {
		t.Error("expected timestamps to be initialized")
	}

	events := user.DomainEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 domain event, got %d", len(events))
	}

	regEvent, ok := events[0].(event.UserRegisteredEvent)
	if !ok {
		t.Fatalf("expected UserRegisteredEvent, got %T", events[0])
	}

	if regEvent.UserID != userID || regEvent.IdentityType != aggregate.IdentityTypeEmail || regEvent.Identifier != emailStr || regEvent.FullName != fullName || regEvent.Role != "user" {
		t.Errorf("unexpected event payload: %+v", regEvent)
	}

	user.ClearDomainEvents()
	if len(user.DomainEvents()) != 0 {
		t.Error("expected domain events to be cleared")
	}
}

func TestRegisterUser_DefaultRole(t *testing.T) {
	ident := aggregate.NewIdentity("id-1", aggregate.IdentityTypeEmail, "test@example.com", "", nil, time.Now(), time.Now())
	user := aggregate.RegisterUser("id-123", "John", ident, "")
	if got, want := user.Role(), vo.RoleUser; got != want {
		t.Errorf("got role %s, want %s", got, want)
	}
	if got, want := user.RoleVO().Value(), vo.RoleUser; got != want {
		t.Errorf("got roleVO %s, want %s", got, want)
	}
}

func TestUser_IdentityManagement(t *testing.T) {
	now := time.Now()
	id1 := aggregate.NewIdentity("id-1", aggregate.IdentityTypeGoogle, "google-sub-123", "", nil, now, now)
	user := aggregate.RegisterUser("id-123", "John Doe", id1, "")

	// FindIdentity
	found, ok := user.FindIdentity(aggregate.IdentityTypeGoogle, "google-sub-123")
	if !ok {
		t.Fatalf("expected identity to be found")
	}
	if got, want := found.Identifier(), "google-sub-123"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	// Find nonexistent identity
	_, notFoundOk := user.FindIdentity(aggregate.IdentityTypeEmail, "missing@example.com")
	if notFoundOk {
		t.Errorf("expected identity to not be found")
	}

	// Update identity and check PrimaryEmail
	emailId := aggregate.NewIdentity("id-2", aggregate.IdentityTypeEmail, "test@example.com", "hash", nil, now, now)
	user.SetIdentity(emailId)
	if got, want := user.PrimaryEmail(), "test@example.com"; got != want {
		t.Errorf("got primary email %s, want %s", got, want)
	}
}

func TestUser_StateTransitions(t *testing.T) {
	ident := aggregate.NewIdentity("id-1", aggregate.IdentityTypeEmail, "test@example.com", "", nil, time.Now(), time.Now())
	user := aggregate.RegisterUser("id-123", "John Doe", ident, "")

	if err := user.Lock(); err != nil {
		t.Fatalf("unexpected error locking user: %v", err)
	}
	if got, want := user.Status(), aggregate.UserStatusLocked; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if err := user.Lock(); err != derror.ErrUserAlreadyLocked {
		t.Errorf("expected ErrUserAlreadyLocked, got %v", err)
	}

	user.Suspend()
	if got, want := user.Status(), aggregate.UserStatusSuspended; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	if err := user.Activate(); err != nil {
		t.Fatalf("unexpected error activating user: %v", err)
	}
	if got, want := user.Status(), aggregate.UserStatusActive; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if err := user.Activate(); err != derror.ErrUserAlreadyActive {
		t.Errorf("expected ErrUserAlreadyActive, got %v", err)
	}

	newRole, _ := vo.NewRole("brand")
	user.ChangeRole(newRole)
	if got, want := user.Role(), "brand"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	user.UpdateFullName("Jane Doe")
	if got, want := user.FullName(), "Jane Doe"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestIdentityTypes_Validation(t *testing.T) {
	tests := []struct {
		name      string
		give      string
		wantOAuth bool
	}{
		{
			name:      "email identity",
			give:      aggregate.IdentityTypeEmail,
			wantOAuth: false,
		},
		{
			name:      "phone identity",
			give:      aggregate.IdentityTypePhone,
			wantOAuth: false,
		},
		{
			name:      "google identity",
			give:      aggregate.IdentityTypeGoogle,
			wantOAuth: true,
		},
		{
			name:      "facebook identity",
			give:      aggregate.IdentityTypeFacebook,
			wantOAuth: true,
		},
		{
			name:      "unsupported identity",
			give:      "twitter",
			wantOAuth: false,
		},
		{
			name:      "empty identity",
			give:      "",
			wantOAuth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := aggregate.IsSupportedOAuthProvider(tt.give), tt.wantOAuth; got != want {
				t.Errorf("IsSupportedOAuthProvider(%q) = %v, want %v", tt.give, got, want)
			}
		})
	}
}
