package entity_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

func TestUserSnapshot(t *testing.T) {
	now := time.Now().UTC()
	u := entity.NewUserSnapshot("u-1", "Alice", "https://avatar.png", "user", now)
	if u.ID() != "u-1" || u.FullName() != "Alice" || u.AvatarURL() != "https://avatar.png" || u.Role() != "user" || u.UpdatedAt() != now {
		t.Errorf("unexpected snapshot properties")
	}

	u.UpdateRole("brand")
	if u.Role() != "brand" {
		t.Errorf("expected role 'brand', got %s", u.Role())
	}
}

func TestUserSnapshot_DefaultAvatar(t *testing.T) {
	now := time.Now().UTC()
	u := entity.NewUserSnapshot("u-2", "Bob", "", "user", now)
	if u.AvatarURL() != entity.DefaultAvatarURL {
		t.Errorf("expected default avatar %s, got %s", entity.DefaultAvatarURL, u.AvatarURL())
	}
}
