package aggregate_test

import (
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
)

func TestNewUserStreak(t *testing.T) {
	_, err := aggregate.NewUserStreak("")
	if !errors.Is(err, aggregate.ErrEmptyUserID) {
		t.Errorf("NewUserStreak('') err = %v, want %v", err, aggregate.ErrEmptyUserID)
	}

	streak, err := aggregate.NewUserStreak("user-1")
	if err != nil {
		t.Fatalf("NewUserStreak unexpected error: %v", err)
	}
	if streak.CurrentStreak() != 0 || streak.LongestStreak() != 0 || streak.LastActivityDate() != "" {
		t.Errorf("NewUserStreak unexpected state: %+v", streak)
	}
}

func TestUserStreak_RecordActivity_SequentialDays(t *testing.T) {
	streak, _ := aggregate.NewUserStreak("user-1")
	now := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)

	// Day 1
	events, err := streak.RecordActivity("2026-10-10", "WORKOUT", now)
	if err != nil {
		t.Fatalf("Day 1 RecordActivity error: %v", err)
	}
	if got := len(events); got != 1 {
		t.Fatalf("Day 1 emitted %d events, want 1", got)
	}
	advEv, ok := events[0].(*event.StreakAdvanced)
	if !ok || advEv.NewStreak != 1 || advEv.LongestStreak != 1 {
		t.Errorf("Day 1 unexpected event: %+v", events[0])
	}
	if streak.CurrentStreak() != 1 || streak.LongestStreak() != 1 {
		t.Errorf("Day 1 state = (%d, %d), want (1, 1)", streak.CurrentStreak(), streak.LongestStreak())
	}

	// Multiple activities on same day (AC-ST-02: Idempotent)
	events, err = streak.RecordActivity("2026-10-10", "NUTRITION", now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("Same day RecordActivity error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("Same day second activity should emit 0 events, got %d", len(events))
	}
	if streak.CurrentStreak() != 1 {
		t.Errorf("CurrentStreak should still be 1, got %d", streak.CurrentStreak())
	}

	// Day 2 (Consecutive)
	events, err = streak.RecordActivity("2026-10-11", "WORKOUT", now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Day 2 error: %v", err)
	}
	if len(events) != 1 || streak.CurrentStreak() != 2 || streak.LongestStreak() != 2 {
		t.Errorf("Day 2 state = (%d, %d), want (2, 2)", streak.CurrentStreak(), streak.LongestStreak())
	}

	// Day 3 (Consecutive)
	events, err = streak.RecordActivity("2026-10-12", "NUTRITION", now.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("Day 3 error: %v", err)
	}
	if streak.CurrentStreak() != 3 || streak.LongestStreak() != 3 {
		t.Errorf("Day 3 state = (%d, %d), want (3, 3)", streak.CurrentStreak(), streak.LongestStreak())
	}
}

func TestUserStreak_RecordActivity_BrokenStreak(t *testing.T) {
	streak, _ := aggregate.NewUserStreak("user-1")
	now := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)

	// Day 1
	_, _ = streak.RecordActivity("2026-10-10", "WORKOUT", now)
	// Day 2
	_, _ = streak.RecordActivity("2026-10-11", "WORKOUT", now.Add(24*time.Hour))
	// Day 3
	_, _ = streak.RecordActivity("2026-10-12", "WORKOUT", now.Add(48*time.Hour))
	if streak.CurrentStreak() != 3 || streak.LongestStreak() != 3 {
		t.Fatalf("Pre-condition failed: streak = %d", streak.CurrentStreak())
	}

	// Skip 2026-10-13, come back on 2026-10-14 (Gap = 2 days)
	events, err := streak.RecordActivity("2026-10-14", "WORKOUT", now.Add(96*time.Hour))
	if err != nil {
		t.Fatalf("Broken streak error: %v", err)
	}
	if got := len(events); got != 2 {
		t.Fatalf("Broken streak should emit 2 events (StreakReset + StreakAdvanced), got %d", got)
	}

	resetEv, ok := events[0].(*event.StreakReset)
	if !ok || resetEv.PreviousStreak != 3 || resetEv.ResetDate != "2026-10-14" {
		t.Errorf("StreakReset unexpected values: %+v", resetEv)
	}

	advEv, ok := events[1].(*event.StreakAdvanced)
	if !ok || advEv.NewStreak != 1 || advEv.LongestStreak != 3 {
		t.Errorf("StreakAdvanced unexpected values: %+v", advEv)
	}

	if streak.CurrentStreak() != 1 {
		t.Errorf("CurrentStreak should be reset to 1, got %d", streak.CurrentStreak())
	}
	// Longest streak must be preserved!
	if streak.LongestStreak() != 3 {
		t.Errorf("LongestStreak must remain 3, got %d", streak.LongestStreak())
	}
}

func TestUserStreak_RecordActivity_MilestoneTrigger(t *testing.T) {
	now := time.Now().UTC()
	// Reconstitute streak at 6 days
	streak := aggregate.ReconstituteUserStreak("user-1", 6, 6, "2026-10-12", now)

	// Advance to day 7 -> Triggers 7-day milestone
	events, err := streak.RecordActivity("2026-10-13", "WORKOUT", now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("RecordActivity day 7 error: %v", err)
	}
	if got := len(events); got != 2 {
		t.Fatalf("Day 7 should emit 2 events (StreakAdvanced + StreakMilestoneReached), got %d", got)
	}

	msEv, ok := events[1].(*event.StreakMilestoneReached)
	if !ok || msEv.MilestoneDays != 7 {
		t.Errorf("StreakMilestoneReached unexpected values: %+v", msEv)
	}
}

func TestUserStreak_RecordActivity_InvalidDateFormat(t *testing.T) {
	streak, _ := aggregate.NewUserStreak("user-1")
	_, err := streak.RecordActivity("invalid-date", "WORKOUT", time.Now().UTC())
	if !errors.Is(err, aggregate.ErrInvalidDateFormat) {
		t.Errorf("RecordActivity('invalid-date') err = %v, want %v", err, aggregate.ErrInvalidDateFormat)
	}
}

func TestUserStreak_RepairStreakByAdmin(t *testing.T) {
	now := time.Now().UTC()

	t.Run("repairs streak and updates longest streak when exceeded", func(t *testing.T) {
		streak := aggregate.ReconstituteUserStreak("user-1", 5, 10, "2026-10-10", now)

		err := streak.RepairStreakByAdmin(15, "", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := streak.CurrentStreak(); got != 15 {
			t.Errorf("current streak = %d, want 15", got)
		}
		if got := streak.LongestStreak(); got != 15 {
			t.Errorf("longest streak = %d, want 15", got)
		}
	})

	t.Run("repairs streak without changing longest streak when below", func(t *testing.T) {
		streak := aggregate.ReconstituteUserStreak("user-1", 2, 20, "2026-10-10", now)

		err := streak.RepairStreakByAdmin(8, "", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := streak.CurrentStreak(); got != 8 {
			t.Errorf("current streak = %d, want 8", got)
		}
		if got := streak.LongestStreak(); got != 20 {
			t.Errorf("longest streak = %d, want 20", got)
		}
	})

	t.Run("resets streak to 0 and clears last activity date", func(t *testing.T) {
		streak := aggregate.ReconstituteUserStreak("user-1", 5, 10, "2026-10-10", now)

		err := streak.RepairStreakByAdmin(0, "", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := streak.CurrentStreak(); got != 0 {
			t.Errorf("current streak = %d, want 0", got)
		}
		if got := streak.LastActivityDate(); got != "" {
			t.Errorf("last activity date = %q, want empty", got)
		}
		if got := streak.LongestStreak(); got != 10 {
			t.Errorf("longest streak = %d, want 10 (preserved)", got)
		}
	})

	t.Run("default effective date sets yesterday allowing today workout to advance streak", func(t *testing.T) {
		streak, _ := aggregate.NewUserStreak("user-fresh")
		todayDate := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)

		// Admin repairs streak at 10:00 today without specifying effective date
		err := streak.RepairStreakByAdmin(7, "", todayDate)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if streak.LastActivityDate() != "2026-10-14" {
			t.Fatalf("last activity date = %q, want 2026-10-14 (yesterday)", streak.LastActivityDate())
		}

		// User works out at 18:00 today (2026-10-15)
		events, err := streak.RecordActivity("2026-10-15", "WORKOUT", todayDate.Add(8*time.Hour))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if streak.CurrentStreak() != 8 {
			t.Errorf("current streak = %d, want 8 (continued today)", streak.CurrentStreak())
		}
		if streak.LongestStreak() != 8 {
			t.Errorf("longest streak = %d, want 8", streak.LongestStreak())
		}
		if len(events) == 0 {
			t.Fatal("expected StreakAdvanced event, got none")
		}
	})

	t.Run("supports explicit valid effective date", func(t *testing.T) {
		streak, _ := aggregate.NewUserStreak("user-1")
		err := streak.RepairStreakByAdmin(10, "2026-10-10", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if streak.LastActivityDate() != "2026-10-10" {
			t.Errorf("last activity date = %q, want 2026-10-10", streak.LastActivityDate())
		}
	})

	t.Run("rejects future effective date", func(t *testing.T) {
		streak, _ := aggregate.NewUserStreak("user-1")
		futureDate := now.AddDate(0, 0, 2).Format("2006-01-02")
		err := streak.RepairStreakByAdmin(10, futureDate, now)
		if !errors.Is(err, aggregate.ErrFutureActivityDate) {
			t.Errorf("err = %v, want %v", err, aggregate.ErrFutureActivityDate)
		}
	})

	t.Run("rejects invalid date format", func(t *testing.T) {
		streak, _ := aggregate.NewUserStreak("user-1")
		err := streak.RepairStreakByAdmin(10, "invalid-date", now)
		if !errors.Is(err, aggregate.ErrInvalidDateFormat) {
			t.Errorf("err = %v, want %v", err, aggregate.ErrInvalidDateFormat)
		}
	})

	t.Run("rejects negative target streak", func(t *testing.T) {
		streak, _ := aggregate.NewUserStreak("user-1")
		err := streak.RepairStreakByAdmin(-1, "", now)
		if !errors.Is(err, aggregate.ErrNegativeStreak) {
			t.Errorf("err = %v, want %v", err, aggregate.ErrNegativeStreak)
		}
	})

	t.Run("rejects target streak exceeding 3650 days", func(t *testing.T) {
		streak, _ := aggregate.NewUserStreak("user-1")
		err := streak.RepairStreakByAdmin(5000, "", now)
		if !errors.Is(err, aggregate.ErrStreakExceedsLimit) {
			t.Errorf("err = %v, want %v", err, aggregate.ErrStreakExceedsLimit)
		}
	})
}
