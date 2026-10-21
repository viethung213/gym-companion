package entity_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
)

func TestUserSnapshot(t *testing.T) {
	now := time.Now().UTC()

	t.Run("successfully creates user snapshot with valid params", func(t *testing.T) {
		u, err := entity.NewUserSnapshot("user_123", "John Doe", "https://avatar.png", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.UserID() != "user_123" {
			t.Errorf("UserID got %v, want user_123", u.UserID())
		}
		if u.DisplayName() != "John Doe" {
			t.Errorf("DisplayName got %v, want John Doe", u.DisplayName())
		}
		if u.AvatarURL() != "https://avatar.png" {
			t.Errorf("AvatarURL got %v, want https://avatar.png", u.AvatarURL())
		}
		if !u.SyncedAt().Equal(now) {
			t.Errorf("SyncedAt got %v, want %v", u.SyncedAt(), now)
		}
	})

	t.Run("fails when user ID is empty", func(t *testing.T) {
		_, err := entity.NewUserSnapshot("", "John Doe", "", now)
		if err == nil {
			t.Errorf("expected error for empty user ID, got nil")
		}
	})

	t.Run("falls back to default Gym Athlete when display name is empty", func(t *testing.T) {
		u, err := entity.NewUserSnapshot("user_456", "", "", time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.DisplayName() != "Gym Athlete" {
			t.Errorf("DisplayName got %v, want Gym Athlete", u.DisplayName())
		}
	})

	t.Run("successfully updates snapshot profile and sync timestamp", func(t *testing.T) {
		u, _ := entity.NewUserSnapshot("user_123", "John Doe", "https://avatar.png", now)
		later := now.Add(1 * time.Hour)
		u.Update("John Updated", "https://new_avatar.png", later)

		if u.DisplayName() != "John Updated" {
			t.Errorf("Updated DisplayName got %v, want John Updated", u.DisplayName())
		}
		if u.AvatarURL() != "https://new_avatar.png" {
			t.Errorf("Updated AvatarURL got %v, want https://new_avatar.png", u.AvatarURL())
		}
		if !u.SyncedAt().Equal(later) {
			t.Errorf("Updated SyncedAt got %v, want %v", u.SyncedAt(), later)
		}
	})
}
