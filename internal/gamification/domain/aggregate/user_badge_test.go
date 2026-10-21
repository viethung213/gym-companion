package aggregate_test

import (
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
)

func TestUnlockBadge(t *testing.T) {
	now := time.Now().UTC()

	// Empty user ID
	_, _, err := aggregate.UnlockBadge("id-1", "", "FIRST_WORKOUT", "First Workout", "MILESTONE", nil, now)
	if !errors.Is(err, aggregate.ErrEmptyUserID) {
		t.Errorf("UnlockBadge('') err = %v, want %v", err, aggregate.ErrEmptyUserID)
	}

	// Empty badge code
	_, _, err = aggregate.UnlockBadge("id-1", "user-1", "", "First Workout", "MILESTONE", nil, now)
	if !errors.Is(err, aggregate.ErrEmptyBadgeCode) {
		t.Errorf("UnlockBadge(code='') err = %v, want %v", err, aggregate.ErrEmptyBadgeCode)
	}

	// Successful unlock
	badge, ev, err := aggregate.UnlockBadge(
		"badge-1",
		"user-1",
		"STREAK_7_DAYS",
		"7-Day Warrior",
		"STREAK",
		map[string]string{"streak_days": "7"},
		now,
	)
	if err != nil {
		t.Fatalf("UnlockBadge error: %v", err)
	}

	if badge.BadgeCode() != "STREAK_7_DAYS" || badge.UserID() != "user-1" {
		t.Errorf("badge state mismatch: %+v", badge)
	}

	badgeEv, ok := ev.(*event.BadgeUnlocked)
	if !ok {
		t.Fatalf("emitted event is not *BadgeUnlocked: %T", ev)
	}
	if badgeEv.BadgeCode != "STREAK_7_DAYS" || badgeEv.UserID != "user-1" {
		t.Errorf("event state mismatch: %+v", badgeEv)
	}
}

func TestReconstituteUserBadge(t *testing.T) {
	now := time.Now().UTC()
	badge := aggregate.ReconstituteUserBadge(
		"b-1",
		"u-1",
		"LEVEL_10",
		"Level 10 Master",
		"LEVEL",
		now,
		map[string]string{"level": "10"},
	)

	if badge.ID() != "b-1" || badge.BadgeCode() != "LEVEL_10" || badge.Metadata()["level"] != "10" {
		t.Errorf("ReconstituteUserBadge mismatch: %+v", badge)
	}
}
