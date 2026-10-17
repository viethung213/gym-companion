package persistence_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	"gorm.io/gorm"
)

func TestGormUserXpRepository_Constructor(t *testing.T) {
	dummyDB := &gorm.DB{Config: &gorm.Config{}, Statement: &gorm.Statement{}}
	repo := persistence.NewGormUserXpRepository(dummyDB)
	if repo == nil {
		t.Fatal("NewGormUserXpRepository() = nil, want non-nil")
	}
}

func TestGormUserXpRepository_Mapping(t *testing.T) {
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	date := "2026-10-13"

	// Reconstitute aggregate
	userXp := aggregate.ReconstituteUserXp("user-1", 1250, 6, &date, now)
	if userXp.UserID() != "user-1" || userXp.Xp() != 1250 || userXp.Level() != 6 {
		t.Fatalf("unexpected aggregate state: %+v", userXp)
	}
	if userXp.LastNutritionRewardDate() == nil || *userXp.LastNutritionRewardDate() != "2026-10-13" {
		t.Errorf("LastNutritionRewardDate() = %v, want '2026-10-13'", userXp.LastNutritionRewardDate())
	}
}
