package aggregate_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
)

func TestAggregates_ConcurrentCreationAndEvaluation(t *testing.T) {
	concurrency := 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	now := time.Now().UTC()

	for i := 0; i < concurrency; i++ {
		workerID := i
		go func() {
			defer wg.Done()

			userID := fmt.Sprintf("user-%03d", workerID)

			// 1. Concurrent UserXp creation & progression
			userXp, err := aggregate.NewUserXp(userID)
			if err != nil {
				t.Errorf("NewUserXp error: %v", err)
				return
			}
			events, err := userXp.EarnXp(100, "WORKOUT", now)
			if err != nil {
				t.Errorf("EarnXp error: %v", err)
				return
			}
			if len(events) == 0 {
				t.Errorf("expected events from EarnXp, got 0")
			}

			// 2. Concurrent UserStreak creation & activity recording
			streak, err := aggregate.NewUserStreak(userID)
			if err != nil {
				t.Errorf("NewUserStreak error: %v", err)
				return
			}
			streakEvents, err := streak.RecordActivity("2026-10-13", "WORKOUT", now)
			if err != nil {
				t.Errorf("RecordActivity error: %v", err)
				return
			}
			if len(streakEvents) != 1 {
				t.Errorf("expected 1 streak event, got %d", len(streakEvents))
			}

			// 3. Concurrent Badge unlocking
			badge, badgeEv, err := aggregate.UnlockBadge(
				fmt.Sprintf("b-%03d", workerID),
				userID,
				"FIRST_WORKOUT",
				"First Workout",
				"MILESTONE",
				nil,
				now,
			)
			if err != nil || badge == nil || badgeEv == nil {
				t.Errorf("UnlockBadge error: %v", err)
			}
		}()
	}

	wg.Wait()
}
