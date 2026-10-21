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
	if u.FollowerCount() != 0 || u.FollowingCount() != 0 {
		t.Errorf("expected zero follow counts by default, got follower=%d following=%d", u.FollowerCount(), u.FollowingCount())
	}
}

func TestUserSnapshot_WithCounts(t *testing.T) {
	now := time.Now().UTC()
	u := entity.NewUserSnapshotWithCounts("u-3", "Charlie", "https://avatar.png", "brand", 15, 25, now)
	if u.FollowerCount() != 15 || u.FollowingCount() != 25 {
		t.Errorf("expected counts 15 and 25, got follower=%d following=%d", u.FollowerCount(), u.FollowingCount())
	}

	// Negative values normalized to 0
	uNegative := entity.NewUserSnapshotWithCounts("u-4", "Dave", "", "user", -5, -10, now)
	if uNegative.FollowerCount() != 0 || uNegative.FollowingCount() != 0 {
		t.Errorf("expected negative counts normalized to 0, got follower=%d following=%d", uNegative.FollowerCount(), uNegative.FollowingCount())
	}

	u.UpdateCounts(20, 30)
	if u.FollowerCount() != 20 || u.FollowingCount() != 30 {
		t.Errorf("expected updated counts 20 and 30, got follower=%d following=%d", u.FollowerCount(), u.FollowingCount())
	}

	u.UpdateCounts(-1, -2)
	if u.FollowerCount() != 0 || u.FollowingCount() != 0 {
		t.Errorf("expected updated negative counts normalized to 0, got follower=%d following=%d", u.FollowerCount(), u.FollowingCount())
	}
}
