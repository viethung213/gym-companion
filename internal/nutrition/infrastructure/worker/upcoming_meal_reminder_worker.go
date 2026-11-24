package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
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
	planRepo      repository.NutritionPlanRepository
	eventRecorder port.EventRecorder
	interval      time.Duration
	stopChan      chan struct{}
	mu            sync.Mutex
	sentReminders map[string]time.Time

	profileClient repository.ProfileClient
	zones         map[string]cachedZone // userID -> zone, refreshed every zoneCacheTTL
}

// cachedZone is a user's time zone as last read from the Profile context.
type cachedZone struct {
	loc      *time.Location
	loadedAt time.Time
}

// zoneCacheTTL bounds how stale a cached zone may be; zones rarely change.
const zoneCacheTTL = time.Hour

// NewUpcomingMealReminderWorker constructs UpcomingMealReminderWorker.
func NewUpcomingMealReminderWorker(
	planRepo repository.NutritionPlanRepository,
	eventRecorder port.EventRecorder,
	interval time.Duration,
) *UpcomingMealReminderWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &UpcomingMealReminderWorker{
		planRepo:      planRepo,
		eventRecorder: eventRecorder,
		interval:      interval,
		stopChan:      make(chan struct{}),
		sentReminders: make(map[string]time.Time),
		zones:         make(map[string]cachedZone),
	}
}

// WithProfileClient makes meal times resolve in each user's time zone; without
// it they are read as UTC wall clock.
func (w *UpcomingMealReminderWorker) WithProfileClient(client repository.ProfileClient) *UpcomingMealReminderWorker {
	w.profileClient = client
	return w
}

// Start launches the background ticker.
func (w *UpcomingMealReminderWorker) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "UpcomingMealReminderWorker started, checking upcoming meals", slog.Duration("interval", w.interval))
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

// runCheck enqueues reminders for upcoming meals.
func (w *UpcomingMealReminderWorker) runCheck(ctx context.Context) {
	w.runCheckAt(ctx, time.Now())
}

// runCheckAt looks at the plans dated yesterday, today and tomorrow (UTC): a
// user's "today" is one of them in every zone. Each meal time is wall clock in
// the user's zone.
func (w *UpcomingMealReminderWorker) runCheckAt(ctx context.Context, now time.Time) {
	w.cleanupOldReminders(now)

	var plans []*aggregate.NutritionPlan
	for _, d := range []int{-1, 0, 1} {
		dayPlans, err := w.planRepo.FindPlansForDate(ctx, now.UTC().AddDate(0, 0, d))
		if err != nil {
			slog.ErrorContext(ctx, "Error fetching plans for date in UpcomingMealReminderWorker", slog.String("error", err.Error()))
			return
		}
		plans = append(plans, dayPlans...)
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
			mealTime := parseMealTime(plan.PlanDate(), scheduledTimeStr, w.zone(ctx, plan.UserID(), now))
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

				if err := w.eventRecorder.RecordEvents(ctx, []any{reminderEvent}); err != nil {
					// Release the slot so the next tick in the window retries.
					w.mu.Lock()
					delete(w.sentReminders, reminderKey)
					w.mu.Unlock()
					slog.ErrorContext(ctx, "Error recording reminder for meal",
						slog.String("meal_type", meal.MealType()),
						slog.String("user_id", plan.UserID()),
						slog.String("error", err.Error()),
					)
				} else {
					slog.InfoContext(ctx, "Recorded meal reminder for user",
						slog.String("user_id", plan.UserID()),
						slog.String("meal_type", meal.MealType()),
						slog.String("scheduled_time", scheduledTimeStr),
					)
				}
			}
		}
	}
}

// parseMealTime combines plan_date's calendar day with an "HH:MM" wall clock in loc.
func parseMealTime(planDate time.Time, scheduledTimeStr string, loc *time.Location) time.Time {
	year, month, day := planDate.Date()
	hour, minute := 12, 0 // Default 12:00 if invalid format
	if scheduledTimeStr != "" {
		_, _ = fmt.Sscanf(scheduledTimeStr, "%d:%d", &hour, &minute)
	}
	return time.Date(year, month, day, hour, minute, 0, 0, loc)
}

func getMealName(meal aggregate.DailyMeal) string {
	opts := meal.Options()
	if len(opts) > 0 && opts[0].MealName() != "" {
		return opts[0].MealName()
	}
	switch strings.ToUpper(strings.TrimSpace(meal.MealType())) {
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

// zone returns the user's time zone, cached for zoneCacheTTL. Without a profile
// client, or when the profile is unreachable, meal times are read as UTC.
func (w *UpcomingMealReminderWorker) zone(ctx context.Context, userID string, now time.Time) *time.Location {
	if w.profileClient == nil {
		return time.UTC
	}
	w.mu.Lock()
	cached, ok := w.zones[userID]
	w.mu.Unlock()
	if ok && now.Sub(cached.loadedAt) < zoneCacheTTL {
		return cached.loc
	}

	loc := command.ResolveAthleteProfile(ctx, w.profileClient, userID).Location
	w.mu.Lock()
	w.zones[userID] = cachedZone{loc: loc, loadedAt: now}
	w.mu.Unlock()
	return loc
}
