package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

const (
	// maxConcurrentAICalls giới hạn số lượng AI calls đồng thời lúc 5AM để tránh bottleneck.
	maxConcurrentAICalls = 10
	// activeUserWithinDays xác định user "hoạt động" là đã log bữa ăn trong 7 ngày qua.
	activeUserWithinDays = 7
	// defaultDailyTargetHour là giờ mặc định kích hoạt hằng ngày (05:00 sáng).
	defaultDailyTargetHour = 5
	// defaultDailyTargetMinute là phút mặc định kích hoạt (00).
	defaultDailyTargetMinute = 0
)

// DailyMenuCronWorker sinh thực đơn hằng ngày cho các user hoạt động.
type DailyMenuCronWorker struct {
	stopChan            chan struct{}
	generatePlanHandler *command.GenerateDailyPlanHandler
	planRepo            repository.NutritionPlanRepository
	profileClient       repository.ProfileClient
	allergyRepo         repository.UserAllergyRepository
	coordinator         repository.CronTaskCoordinator
	targetHour          int
	targetMinute        int
}

func (w *DailyMenuCronWorker) WithUserAllergyRepository(repo repository.UserAllergyRepository) *DailyMenuCronWorker {
	w.allergyRepo = repo
	return w
}

// NewDailyMenuCronWorker khởi tạo DailyMenuCronWorker.
// profileClient có thể là nil — khi đó worker dùng fallback metrics.
func NewDailyMenuCronWorker(
	generatePlanHandler *command.GenerateDailyPlanHandler,
	planRepo repository.NutritionPlanRepository,
	profileClient repository.ProfileClient,
	coordinator repository.CronTaskCoordinator,
	targetHour, targetMinute int,
) *DailyMenuCronWorker {
	if targetHour < 0 || targetHour > 23 {
		targetHour = defaultDailyTargetHour
	}
	if targetMinute < 0 || targetMinute > 59 {
		targetMinute = defaultDailyTargetMinute
	}

	return &DailyMenuCronWorker{
		stopChan:            make(chan struct{}),
		generatePlanHandler: generatePlanHandler,
		planRepo:            planRepo,
		profileClient:       profileClient,
		coordinator:         coordinator,
		targetHour:          targetHour,
		targetMinute:        targetMinute,
	}
}

// NextDailyTarget tính thời điểm targetHour:targetMinute tiếp theo tính từ mốc now.
func NextDailyTarget(now time.Time, targetHour, targetMinute int) time.Time {
	targetToday := time.Date(now.Year(), now.Month(), now.Day(), targetHour, targetMinute, 0, 0, now.Location())
	if now.Before(targetToday) {
		return targetToday
	}
	tomorrow := now.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), targetHour, targetMinute, 0, 0, now.Location())
}

// DurationUntilNextRun trả về thời gian chờ tới lần kích hoạt tiếp theo.
func (w *DailyMenuCronWorker) DurationUntilNextRun(now time.Time) time.Duration {
	nextRun := NextDailyTarget(now, w.targetHour, w.targetMinute)
	dur := nextRun.Sub(now)
	if dur <= 0 {
		return 24 * time.Hour
	}
	return dur
}

// Start bắt đầu vòng lặp cron thức dậy đúng 1 lần vào targetHour:targetMinute mỗi ngày.
func (w *DailyMenuCronWorker) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "DailyMenuCronWorker started — generates daily menus for active users",
		slog.Int("target_hour", w.targetHour),
		slog.Int("target_minute", w.targetMinute),
	)

	go func() {
		for {
			duration := w.DurationUntilNextRun(time.Now())
			slog.InfoContext(ctx, "DailyMenuCronWorker waiting for next run",
				slog.Duration("time_until_run", duration),
			)

			timer := time.NewTimer(duration)
			select {
			case <-timer.C:
				_, _, _ = w.RunOnce(ctx)
			case <-w.stopChan:
				timer.Stop()
				return
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
	}()

	return nil
}

// RunOnce lấy danh sách user hoạt động và sinh thực đơn song song với semaphore pool thông qua coordinator.
func (w *DailyMenuCronWorker) RunOnce(ctx context.Context) (successCount, failureCount int, err error) {
	slog.InfoContext(ctx, "DailyMenuCronWorker triggered, loading active users")

	if w.planRepo == nil || w.generatePlanHandler == nil {
		slog.WarnContext(ctx, "DailyMenuCronWorker skipped: dependencies not configured")
		return 0, 0, errors.New("daily menu cron dependencies not configured")
	}

	if w.coordinator == nil {
		slog.ErrorContext(ctx, "DailyMenuCronWorker aborted: CronTaskCoordinator is required but nil")
		return 0, 0, errors.New("daily menu cron coordinator is nil")
	}

	targetDate := time.Now().Format("2006-01-02")
	activeUserIDs, err := w.coordinator.EnsureTaskSet(ctx, "daily_menu", targetDate, func() ([]string, error) {
		return w.planRepo.FindActiveUserIDs(ctx, activeUserWithinDays)
	})
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get active users in DailyMenuCronWorker", slog.String("error", err.Error()))
		return 0, 0, err
	}

	if len(activeUserIDs) == 0 {
		slog.InfoContext(ctx, "No active users found in past days, skipping menu generation", slog.Int("days", activeUserWithinDays))
		return 0, 0, nil
	}

	slog.InfoContext(ctx, "Starting daily menu generation",
		slog.Int("total_users", len(activeUserIDs)),
		slog.Int("max_concurrent", maxConcurrentAICalls),
	)

	var userAllergiesMap map[string][]string
	if w.allergyRepo != nil {
		allergies, aErr := w.allergyRepo.BatchFindAllergiesByUserIDs(ctx, activeUserIDs)
		if aErr != nil {
			slog.WarnContext(ctx, "Failed to batch query user allergies, continuing with empty", slog.Any("error", aErr))
		} else {
			userAllergiesMap = allergies
		}
	}

	semaphore := make(chan struct{}, maxConcurrentAICalls)
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount, failCount := 0, 0

	for _, userID := range activeUserIDs {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(uid string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			claimed, claimErr := w.coordinator.TryClaimUser(ctx, "daily_menu", targetDate, uid, 5*time.Minute)
			if claimErr != nil {
				slog.ErrorContext(ctx, "Failed to claim daily menu task for user",
					slog.String("user_id", uid),
					slog.String("error", claimErr.Error()),
				)
				return
			}
			if !claimed {
				// Another node is working on this user or user has already completed
				return
			}

			athlete := command.ResolveAthleteProfile(ctx, w.profileClient, uid)

			var restrictions []string
			if userAllergiesMap != nil {
				restrictions = userAllergiesMap[uid]
			}

			_, genErr := w.generatePlanHandler.Handle(ctx, command.GenerateDailyPlanCommand{
				UserID:            uid,
				PlanDate:          athlete.Today(time.Now()),
				BiologicalMetrics: athlete.Metrics,
				UserRestrictions:  restrictions,
			})

			mu.Lock()
			if genErr != nil {
				failCount++
				slog.ErrorContext(ctx, "Failed to generate daily plan for user", slog.String("user_id", uid), slog.String("error", genErr.Error()))
				_ = w.coordinator.ReleaseUserLock(ctx, "daily_menu", targetDate, uid)
			} else {
				successCount++
				_ = w.coordinator.CompleteUser(ctx, "daily_menu", targetDate, uid)
			}
			mu.Unlock()
		}(userID)
	}

	wg.Wait()
	slog.InfoContext(ctx, "DailyMenuCronWorker completed",
		slog.Int("success", successCount),
		slog.Int("failed", failCount),
		slog.Int("total", len(activeUserIDs)),
	)

	return successCount, failCount, nil
}

// Stop dừng cron worker.
func (w *DailyMenuCronWorker) Stop() {
	close(w.stopChan)
}
