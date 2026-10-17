package persistence_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	"gorm.io/gorm"
)

func TestGormUserStreakRepository_Constructor(t *testing.T) {
	dummyDB := &gorm.DB{Config: &gorm.Config{}, Statement: &gorm.Statement{}}
	repo := persistence.NewGormUserStreakRepository(dummyDB)
	if repo == nil {
		t.Fatal("NewGormUserStreakRepository() = nil, want non-nil")
	}
}

func TestGormUserStreakRepository_Mapping(t *testing.T) {
	now := time.Now().UTC()
	streak := aggregate.ReconstituteUserStreak("user-1", 7, 14, "2026-10-13", now)
	if streak.UserID() != "user-1" || streak.CurrentStreak() != 7 || streak.LongestStreak() != 14 || streak.LastActivityDate() != "2026-10-13" {
		t.Errorf("unexpected streak aggregate mapping: %+v", streak)
	}
}
