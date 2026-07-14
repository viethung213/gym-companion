//go:build unit

package aggregate

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/event"
)

func TestRegisterUser_Success(t *testing.T) {
	userID := "9e0dc099-0df4-436f-b258-004ea10a6234"
	emailStr := "test@example.com"
	fullName := "John Doe"
	roleStr := "user"

	user, err := RegisterUser(userID, emailStr, fullName, roleStr)
	if err != nil {
		t.Fatalf("unexpected registration failure: %v", err)
	}

	// Verify fields
	if got, want := user.ID(), userID; got != want {
		t.Errorf("got ID %s, want %s", got, want)
	}
	if got, want := user.Email(), emailStr; got != want {
		t.Errorf("got email %s, want %s", got, want)
	}
	if got, want := user.FullName(), fullName; got != want {
		t.Errorf("got full name %s, want %s", got, want)
	}
	if got, want := user.Role(), roleStr; got != want {
		t.Errorf("got role %s, want %s", got, want)
	}

	// Verify timestamps
	if user.CreatedAt().IsZero() || user.UpdatedAt().IsZero() {
		t.Error("expected timestamps to be initialized")
	}

	// Verify Domain Events
	events := user.DomainEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 domain event, got %d", len(events))
	}

	regEvent, ok := events[0].(event.UserRegisteredEvent)
	if !ok {
		t.Fatalf("expected UserRegisteredEvent, got %T", events[0])
	}

	if regEvent.UserID != userID || regEvent.Email != emailStr || regEvent.FullName != fullName {
		t.Errorf("unexpected event payload: %+v", regEvent)
	}

	// Verify clear works
	user.ClearDomainEvents()
	if len(user.DomainEvents()) != 0 {
		t.Error("expected domain events to be cleared")
	}
}

func TestRegisterUser_InvalidID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"empty id", ""},
		{"random string", "invalid-id-format"},
		{"too short", "9e0dc099-0df4"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RegisterUser(tc.id, "test@example.com", "John", "user")
			if err == nil {
				t.Error("expected error due to invalid user id format")
			}
		})
	}
}

func TestRegisterUser_InvalidEmail(t *testing.T) {
	_, err := RegisterUser("9e0dc099-0df4-436f-b258-004ea10a6234", "invalid-email", "John", "user")
	if err == nil {
		t.Error("expected error due to invalid email format")
	}
}

func TestRegisterUser_InvalidRole(t *testing.T) {
	_, err := RegisterUser("9e0dc099-0df4-436f-b258-004ea10a6234", "test@example.com", "John", "invalid-role")
	if err == nil {
		t.Error("expected error due to invalid role")
	}
}

func TestUser_LinkGoogle(t *testing.T) {
	user, _ := RegisterUser("9e0dc099-0df4-436f-b258-004ea10a6234", "test@example.com", "John", "user")
	originalUpdatedAt := user.UpdatedAt()

	// Wait slightly to ensure time advances for UpdatedAt check
	time.Sleep(10 * time.Millisecond)

	googleID := "google-id-123"
	user.LinkGoogle(googleID)

	if got, want := user.GoogleID(), googleID; got != want {
		t.Errorf("got GoogleID %s, want %s", got, want)
	}
	if !user.UpdatedAt().After(originalUpdatedAt) {
		t.Error("expected UpdatedAt timestamp to be updated")
	}
}

func TestUser_LinkFacebook(t *testing.T) {
	user, _ := RegisterUser("9e0dc099-0df4-436f-b258-004ea10a6234", "test@example.com", "John", "user")
	originalUpdatedAt := user.UpdatedAt()

	// Wait slightly to ensure time advances for UpdatedAt check
	time.Sleep(10 * time.Millisecond)

	facebookID := "fb-id-123"
	user.LinkFacebook(facebookID)

	if got, want := user.FacebookID(), facebookID; got != want {
		t.Errorf("got FacebookID %s, want %s", got, want)
	}
	if !user.UpdatedAt().After(originalUpdatedAt) {
		t.Error("expected UpdatedAt timestamp to be updated")
	}
}
