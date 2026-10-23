package persistence_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	"gorm.io/gorm"
)

func TestGormLeaderboardRepository_Constructor(t *testing.T) {
	dummyDB := &gorm.DB{Config: &gorm.Config{}, Statement: &gorm.Statement{}}
	repo := persistence.NewGormLeaderboardRepository(dummyDB)
	if repo == nil {
		t.Fatal("NewGormLeaderboardRepository() = nil, want non-nil")
	}
}

func TestUserSnapshotEntity_Constructor(t *testing.T) {
	now := time.Now().UTC()
	snap := entity.NewUserSnapshot("user-1", "Champion Gymer", "https://cdn.example.com/avatar.jpg", now)
	if snap.UserID != "user-1" || snap.DisplayName != "Champion Gymer" || snap.AvatarURL != "https://cdn.example.com/avatar.jpg" {
		t.Errorf("unexpected snapshot entity: %+v", snap)
	}
}
