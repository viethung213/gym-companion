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
	// maxConcurrentInsightCalls giới hạn số lượng AI calls đồng thời khi sinh insight tuần.
	maxConcurrentInsightCalls = 5
	// defaultSundayTargetHour là giờ mặc định kích hoạt vào cuối ngày Chủ Nhật (23:00).
	defaultSundayTargetHour = 23
	// defaultSundayTargetMinute là phút mặc định kích hoạt (00).
	defaultSundayTargetMinute = 0
)

// WeeklyInsightCronWorker kích hoạt đánh giá dinh dưỡng tuần vào cuối ngày Chủ Nhật mỗi tuần.
type WeeklyInsightCronWorker struct {
	stopChan               chan struct{}
	generateInsightHandler *command.GenerateWeeklyInsightHandler
	planRepo               repository.NutritionPlanRepository
	coordinator            repository.CronTaskCoordinator
	targetHour             int
	targetMinute           int
}

// NewWeeklyInsightCronWorker khởi tạo WeeklyInsightCronWorker.
// Nếu targetHour < 0, mặc định là 23:00.
func NewWeeklyInsightCronWorker(
	generateInsightHandler *command.GenerateWeeklyInsightHandler,
	planRepo repository.NutritionPlanRepository,
	coordinator repository.CronTaskCoordinator,
	targetHour, targetMinute int,
) *WeeklyInsightCronWorker {
	if targetHour < 0 || targetHour > 23 {
		targetHour = defaultSundayTargetHour
	}
	if targetMinute < 0 || targetMinute > 59 {
		targetMinute = defaultSundayTargetMinute
	}

	return &WeeklyInsightCronWorker{
		stopChan:               make(chan struct{}),
		generateInsightHandler: generateInsightHandler,
		planRepo:               planRepo,
		coordinator:            coordinator,
		targetHour:             targetHour,
		targetMinute:           targetMinute,
	}
}

// NextSundayTarget tính thời điểm cuối ngày Chủ Nhật tiếp theo tính từ mốc now.
func NextSundayTarget(now time.Time, targetHour, targetMinute int) time.Time {
	weekday := now.Weekday()
	daysUntilSunday := (int(time.Sunday) - int(weekday) + 7) % 7

	targetToday := time.Date(now.Year(), now.Month(), now.Day(), targetHour, targetMinute, 0, 0, now.Location())

	if daysUntilSunday == 0 {
		if now.Before(targetToday) {
			return targetToday
		}
		// Đã qua giờ chạy tối Chủ Nhật hôm nay, lên lịch cho Chủ Nhật tuần sau
		daysUntilSunday = 7
	}

	targetDate := now.AddDate(0, 0, daysUntilSunday)
	return time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), targetHour, targetMinute, 0, 0, now.Location())
}

// DurationUntilNextRun trả về thời gian chờ tới lần kích hoạt Chủ Nhật tiếp theo.
func (w *WeeklyInsightCronWorker) DurationUntilNextRun(now time.Time) time.Duration {
	nextRun := NextSundayTarget(now, w.targetHour, w.targetMinute)
	dur := nextRun.Sub(now)
	if dur <= 0 {
		return 7 * 24 * time.Hour
	}
	return dur
}

// Start khởi chạy vòng lặp kích hoạt worker vào cuối ngày Chủ Nhật hàng tuần.
func (w *WeeklyInsightCronWorker) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "WeeklyInsightCronWorker started — scheduled for every Sunday night",
		slog.Int("target_hour", w.targetHour),
		slog.Int("target_minute", w.targetMinute),
	)

	go func() {
		for {
			duration := w.DurationUntilNextRun(time.Now())
			slog.InfoContext(ctx, "WeeklyInsightCronWorker waiting for next run",
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

// RunOnce thực thi quét người dùng hoạt động và kích hoạt sinh insight tuần qua application layer.
func (w *WeeklyInsightCronWorker) RunOnce(ctx context.Context) (successCount, failureCount int, err error) {
	slog.InfoContext(ctx, "WeeklyInsightCronWorker triggered: evaluating active users for weekly insight")

	if w.planRepo == nil || w.generateInsightHandler == nil {
		slog.WarnContext(ctx, "WeeklyInsightCronWorker skipped: dependencies not configured")
		return 0, 0, errors.New("weekly insight cron dependencies not configured")
	}

	if w.coordinator == nil {
		slog.ErrorContext(ctx, "WeeklyInsightCronWorker aborted: CronTaskCoordinator is required but nil")
		return 0, 0, errors.New("weekly insight cron coordinator is nil")
	}

	targetDate := time.Now().Format("2006-01-02")
	userIDs, err := w.coordinator.EnsureTaskSet(ctx, "weekly_insight", targetDate, func() ([]string, error) {
		return w.planRepo.FindActiveUserIDs(ctx, 7)
	})
	if err != nil {
		slog.ErrorContext(ctx, "Failed to load active users for weekly insight", slog.String("error", err.Error()))
		return 0, 0, err
	}

	if len(userIDs) == 0 {
		slog.InfoContext(ctx, "No active users found for weekly insight evaluation, skipping")
		return 0, 0, nil
	}

	slog.InfoContext(ctx, "Starting weekly insight generation",
		slog.Int("total_users", len(userIDs)),
		slog.Int("max_concurrent", maxConcurrentInsightCalls),
	)

	semaphore := make(chan struct{}, maxConcurrentInsightCalls)
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount, failCount, skippedCount := 0, 0, 0

	for _, userID := range userIDs {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(uid string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			claimed, claimErr := w.coordinator.TryClaimUser(ctx, "weekly_insight", targetDate, uid, 5*time.Minute)
			if claimErr != nil {
				slog.ErrorContext(ctx, "Failed to claim weekly insight task for user",
					slog.String("user_id", uid),
					slog.String("error", claimErr.Error()),
				)
				return
			}
			if !claimed {
				// Another node is working on this user or user has already completed
				return
			}

			insight, err := w.generateInsightHandler.Handle(ctx, command.GenerateWeeklyInsightCommand{
				UserID:   uid,
				GoalType: "MAINTENANCE",
			})

			mu.Lock()
			switch {
			case err != nil:
				failCount++
				slog.ErrorContext(ctx, "Failed to generate weekly insight for user",
					slog.String("user_id", uid),
					slog.String("error", err.Error()),
				)
				_ = w.coordinator.ReleaseUserLock(ctx, "weekly_insight", targetDate, uid)
			case insight == nil:
				skippedCount++
				slog.InfoContext(ctx, "Skipped weekly insight (no data logged in past 7 days)",
					slog.String("user_id", uid),
				)
				_ = w.coordinator.CompleteUser(ctx, "weekly_insight", targetDate, uid)
			default:
				successCount++
				_ = w.coordinator.CompleteUser(ctx, "weekly_insight", targetDate, uid)
			}
			mu.Unlock()
		}(userID)
	}

	wg.Wait()

	slog.InfoContext(ctx, "WeeklyInsightCronWorker evaluation batch completed",
		slog.Int("success", successCount),
		slog.Int("failed", failCount),
		slog.Int("skipped", skippedCount),
		slog.Int("total", len(userIDs)),
	)

	return successCount, failCount, nil
}

// Stop dừng worker an toàn.
func (w *WeeklyInsightCronWorker) Stop() {
	close(w.stopChan)
}
