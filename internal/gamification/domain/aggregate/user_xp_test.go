package aggregate_test

import (
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
)

func TestNewUserXp(t *testing.T) {
	_, err := aggregate.NewUserXp("")
	if !errors.Is(err, aggregate.ErrEmptyUserID) {
		t.Errorf("NewUserXp('') err = %v, want %v", err, aggregate.ErrEmptyUserID)
	}

	userXp, err := aggregate.NewUserXp("user-1")
	if err != nil {
		t.Fatalf("NewUserXp('user-1') unexpected error: %v", err)
	}
	if got := userXp.UserID(); got != "user-1" {
		t.Errorf("UserID() = %q, want 'user-1'", got)
	}
	if got := userXp.Xp(); got != 0 {
		t.Errorf("Xp() = %d, want 0", got)
	}
	if got := userXp.Level(); got != 1 {
		t.Errorf("Level() = %d, want 1", got)
	}
	if got := userXp.LastNutritionRewardDate(); got != nil {
		t.Errorf("LastNutritionRewardDate() = %v, want nil", got)
	}
}

func TestUserXp_EarnXp(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	userXp, _ := aggregate.NewUserXp("user-1")

	// 1. Invalid delta <= 0
	_, err := userXp.EarnXp(0, "WORKOUT", now)
	if !errors.Is(err, aggregate.ErrInvalidXpDelta) {
		t.Errorf("EarnXp(0) err = %v, want %v", err, aggregate.ErrInvalidXpDelta)
	}

	// 2. Earn 30 XP (not enough to level up to 2)
	events, err := userXp.EarnXp(30, "WORKOUT", now)
	if err != nil {
		t.Fatalf("EarnXp(30) unexpected error: %v", err)
	}
	if got := len(events); got != 1 {
		t.Fatalf("EarnXp(30) emitted %d events, want 1", got)
	}
	earnedEv, ok := events[0].(*event.XpEarned)
	if !ok {
		t.Fatalf("event[0] is not *event.XpEarned")
	}
	if earnedEv.DeltaXp != 30 || earnedEv.NewXp != 30 || earnedEv.NewLevel != 1 {
		t.Errorf("XpEarned unexpected values: %+v", earnedEv)
	}
	if userXp.Xp() != 30 || userXp.Level() != 1 {
		t.Errorf("userXp state = (%d, %d), want (30, 1)", userXp.Xp(), userXp.Level())
	}

	// 3. Earn another 30 XP (total 60 XP -> reaches Level 2)
	events, err = userXp.EarnXp(30, "WORKOUT", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("EarnXp(30) second time error: %v", err)
	}
	if got := len(events); got != 2 {
		t.Fatalf("EarnXp(30) second time emitted %d events, want 2 (XpEarned + UserLeveledUp)", got)
	}
	leveledUpEv, ok := events[1].(*event.UserLeveledUp)
	if !ok {
		t.Fatalf("event[1] is not *event.UserLeveledUp")
	}
	if leveledUpEv.OldLevel != 1 || leveledUpEv.NewLevel != 2 || leveledUpEv.TotalXp != 60 {
		t.Errorf("UserLeveledUp unexpected values: %+v", leveledUpEv)
	}
	if userXp.Level() != 2 || userXp.Xp() != 60 {
		t.Errorf("userXp state = (%d, %d), want (60, 2)", userXp.Xp(), userXp.Level())
	}
}

func TestUserXp_EarnNutritionXp(t *testing.T) {
	now := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	userXp, _ := aggregate.NewUserXp("user-1")

	// First claim for 2026-10-13 -> +30 XP
	events, err := userXp.EarnNutritionXp("2026-10-13", now)
	if err != nil {
		t.Fatalf("EarnNutritionXp first time error: %v", err)
	}
	if len(events) != 1 || userXp.Xp() != 30 {
		t.Errorf("unexpected state after nutrition reward: xp = %d, events = %d", userXp.Xp(), len(events))
	}
	if userXp.LastNutritionRewardDate() == nil || *userXp.LastNutritionRewardDate() != "2026-10-13" {
		t.Errorf("LastNutritionRewardDate() = %v, want '2026-10-13'", userXp.LastNutritionRewardDate())
	}

	// Second claim on SAME date -> rejected
	_, err = userXp.EarnNutritionXp("2026-10-13", now.Add(4*time.Hour))
	if !errors.Is(err, aggregate.ErrNutritionRewardAlreadyClaimedToday) {
		t.Errorf("EarnNutritionXp duplicate date err = %v, want %v", err, aggregate.ErrNutritionRewardAlreadyClaimedToday)
	}

	// Next date 2026-10-14 -> succeeds (+30 XP -> 60 XP -> Level 2!)
	events, err = userXp.EarnNutritionXp("2026-10-14", now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("EarnNutritionXp next date error: %v", err)
	}
	if userXp.Xp() != 60 || userXp.Level() != 2 {
		t.Errorf("state after second day nutrition reward: xp = %d, level = %d", userXp.Xp(), userXp.Level())
	}
}

func TestReconstituteUserXp(t *testing.T) {
	now := time.Now().UTC()
	date := "2026-10-12"
	userXp := aggregate.ReconstituteUserXp("user-99", 5000, 11, &date, now)
	if userXp.UserID() != "user-99" || userXp.Xp() != 5000 || userXp.Level() != 11 {
		t.Errorf("ReconstituteUserXp unexpected state: %+v", userXp)
	}
	if userXp.LastNutritionRewardDate() == nil || *userXp.LastNutritionRewardDate() != "2026-10-12" {
		t.Errorf("LastNutritionRewardDate() = %v, want '2026-10-12'", userXp.LastNutritionRewardDate())
	}
}
