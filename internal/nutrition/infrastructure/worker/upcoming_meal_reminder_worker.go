package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/event"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

const (
	defaultMealReminderWindow    = 20 * time.Minute
	defaultMealReminderWindowMax = 40 * time.Minute
	remindBeforeMealMinutes      = 30
)

// UpcomingMealReminderWorker scans nutrition plans for today and enqueues UpcomingMealReminder events ~30m before meal time.
type UpcomingMealReminderWorker struct {
	planRepo       repository.NutritionPlanRepository
	eventPublisher port.EventPublisher
	interval       time.Duration
	stopChan       chan struct{}
	mu             sync.Mutex
	sentReminders  map[string]time.Time
}

// NewUpcomingMealReminderWorker constructs UpcomingMealReminderWorker.
func NewUpcomingMealReminderWorker(
	planRepo repository.NutritionPlanRepository,
	eventPublisher port.EventPublisher,
	interval time.Duration,
) *UpcomingMealReminderWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &UpcomingMealReminderWorker{
		planRepo:       planRepo,
		eventPublisher: eventPublisher,
		interval:       interval,
		stopChan:       make(chan struct{}),
		sentReminders:  make(map[string]time.Time),
	}
}

// Start launches the background ticker.
func (w *UpcomingMealReminderWorker) Start(ctx context.Context) error {
	log.Printf("[UpcomingMealReminderWorker] Started — checking upcoming meals every %v", w.interval)
	ticker := time.NewTicker(w.interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				w.runCheck(ctx)
			case <-w.stopChan:
				ticker.Stop()
				return
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	return nil
}

// runCheck fetches today's plans and enqueues reminders for upcoming meals.
func (w *UpcomingMealReminderWorker) runCheck(ctx context.Context) {
	now := time.Now()
	w.cleanupOldReminders(now)

	plans, err := w.planRepo.FindPlansForDate(ctx, now)
	if err != nil {
		log.Printf("[UpcomingMealReminderWorker] Error fetching plans for date: %v", err)
		return
	}

	for _, plan := range plans {
		if plan == nil {
			continue
		}

		for _, meal := range plan.DailyMeals() {
			// Check if any meal option is already logged/consumed
			isConsumed := false
			for _, opt := range meal.Options() {
				if opt.IsLogged() {
					isConsumed = true
					break
				}
			}
			if isConsumed {
				continue
			}

			scheduledTimeStr := meal.ScheduledTime()
			mealTime := parseMealTime(plan.PlanDate(), scheduledTimeStr)
			timeDiff := mealTime.Sub(now)

			if timeDiff >= defaultMealReminderWindow && timeDiff <= defaultMealReminderWindowMax {
				reminderKey := fmt.Sprintf("%s:%s:%s:%s", plan.UserID(), plan.ID(), meal.MealType(), mealTime.Format("2006-01-02_15:04"))

				w.mu.Lock()
				_, alreadySent := w.sentReminders[reminderKey]
				if !alreadySent {
					w.sentReminders[reminderKey] = now
				}
				w.mu.Unlock()

				if alreadySent {
					continue
				}

				mealName := getMealName(meal)
				reminderEvent := event.NewUpcomingMealReminderEvent(
					plan.UserID(),
					mealName,
					meal.MealType(),
					scheduledTimeStr,
					remindBeforeMealMinutes,
				)

				if err := w.eventPublisher.PublishEvents(ctx, []any{reminderEvent}); err != nil {
					log.Printf("[UpcomingMealReminderWorker] Error publishing reminder for meal %s (user %s): %v", meal.MealType(), plan.UserID(), err)
				} else {
					log.Printf("[UpcomingMealReminderWorker] Published meal reminder for user %s, meal %s at %s", plan.UserID(), meal.MealType(), scheduledTimeStr)
				}
			}
		}
	}
}

func parseMealTime(planDate time.Time, scheduledTimeStr string) time.Time {
	year, month, day := planDate.Date()
	hour, minute := 12, 0 // Default 12:00 if invalid format
	if scheduledTimeStr != "" {
		_, _ = fmt.Sscanf(scheduledTimeStr, "%d:%d", &hour, &minute)
	}
	return time.Date(year, month, day, hour, minute, 0, 0, planDate.Location())
}

func getMealName(meal aggregate.DailyMeal) string {
	opts := meal.Options()
	if len(opts) > 0 && opts[0].MealName() != "" {
		return opts[0].MealName()
	}
	switch meal.MealType() {
	case "BREAKFAST":
		return "Bữa Sáng"
	case "LUNCH":
		return "Bữa Trưa"
	case "DINNER":
		return "Bữa Tối"
	case "SNACK":
		return "Bữa Phụ"
	default:
		return "Bữa Ăn Theo Lịch"
	}
}

func (w *UpcomingMealReminderWorker) cleanupOldReminders(now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for key, sentAt := range w.sentReminders {
		if now.Sub(sentAt) > 24*time.Hour {
			delete(w.sentReminders, key)
		}
	}
}

// Stop terminates the worker ticker loop.
func (w *UpcomingMealReminderWorker) Stop() {
	close(w.stopChan)
}
