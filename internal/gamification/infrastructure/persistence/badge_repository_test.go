package persistence_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	"gorm.io/gorm"
)

func TestGormBadgeRepository_Constructor(t *testing.T) {
	dummyDB := &gorm.DB{Config: &gorm.Config{}, Statement: &gorm.Statement{}}
	repo := persistence.NewGormBadgeRepository(dummyDB)
	if repo == nil {
		t.Fatal("NewGormBadgeRepository() = nil, want non-nil")
	}
}

func TestGormBadgeRepository_Mapping(t *testing.T) {
	now := time.Now().UTC()
	badge := aggregate.ReconstituteUserBadge(
		"badge-1",
		"user-1",
		"STREAK_30_DAYS",
		"30-Day Master",
		"STREAK",
		now,
		map[string]string{"streak_days": "30"},
	)
	if badge.BadgeCode() != "STREAK_30_DAYS" || badge.UserID() != "user-1" || badge.Metadata()["streak_days"] != "30" {
		t.Errorf("unexpected badge mapping: %+v", badge)
	}
}
