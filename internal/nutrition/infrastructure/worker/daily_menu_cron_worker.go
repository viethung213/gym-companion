package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
)

const (
	// maxConcurrentAICalls giới hạn số lượng AI calls đồng thời lúc 5AM để tránh bottleneck.
	maxConcurrentAICalls = 10
	// activeUserWithinDays xác định user "hoạt động" là đã log bữa ăn trong 7 ngày qua.
	activeUserWithinDays = 7
)

// getDefaultBiologicalMetrics trả về chỉ số sinh trắc học mặc định khi không lấy được profile thực của user.
func getDefaultBiologicalMetrics() service.BiologicalMetrics {
	return service.BiologicalMetrics{
		WeightKg:      70.0,
		HeightCm:      170.0,
		Age:           25,
		Gender:        "MALE",
		ActivityLevel: "MODERATELY_ACTIVE",
	}
}

// DailyMenuCronWorker sinh thực đơn hằng ngày cho các user hoạt động.
type DailyMenuCronWorker struct {
	stopChan            chan struct{}
	generatePlanHandler *command.GenerateDailyPlanHandler
	planRepo            repository.NutritionPlanRepository
	profileClient       repository.ProfileClient
}

// NewDailyMenuCronWorker khởi tạo DailyMenuCronWorker.
// profileClient có thể là nil — khi đó worker dùng fallback metrics.
func NewDailyMenuCronWorker(
	generatePlanHandler *command.GenerateDailyPlanHandler,
	planRepo repository.NutritionPlanRepository,
	profileClient repository.ProfileClient,
) *DailyMenuCronWorker {
	return &DailyMenuCronWorker{
		stopChan:            make(chan struct{}),
		generatePlanHandler: generatePlanHandler,
		planRepo:            planRepo,
		profileClient:       profileClient,
	}
}

// Start bắt đầu vòng lặp cron mỗi 24 giờ.
func (w *DailyMenuCronWorker) Start(ctx context.Context) error {
	ticker := time.NewTicker(24 * time.Hour)
	slog.InfoContext(ctx, "DailyMenuCronWorker started — generates daily menus for active users")

	go func() {
		for {
			select {
			case <-ticker.C:
				w.runDailyGeneration(ctx)
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

// runDailyGeneration lấy danh sách user hoạt động và sinh thực đơn song song với semaphore pool.
func (w *DailyMenuCronWorker) runDailyGeneration(ctx context.Context) {
	slog.InfoContext(ctx, "DailyMenuCronWorker triggered, loading active users")

	activeUserIDs, err := w.planRepo.FindActiveUserIDs(ctx, activeUserWithinDays)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get active users in DailyMenuCronWorker", slog.String("error", err.Error()))
		return
	}

	if len(activeUserIDs) == 0 {
		slog.InfoContext(ctx, "No active users found in past days, skipping menu generation", slog.Int("days", activeUserWithinDays))
		return
	}

	slog.InfoContext(ctx, "Starting daily menu generation",
		slog.Int("total_users", len(activeUserIDs)),
		slog.Int("max_concurrent", maxConcurrentAICalls),
	)

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

			metrics := w.fetchBiologicalMetrics(ctx, uid)

			_, genErr := w.generatePlanHandler.Handle(ctx, command.GenerateDailyPlanCommand{
				UserID:            uid,
				PlanDate:          time.Now(),
				BiologicalMetrics: metrics,
			})

			mu.Lock()
			if genErr != nil {
				failCount++
				slog.ErrorContext(ctx, "Failed to generate daily plan for user", slog.String("user_id", uid), slog.String("error", genErr.Error()))
			} else {
				successCount++
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
}

// fetchBiologicalMetrics lấy chỉ số sinh trắc học từ ProfileService.
// Nếu profile không tìm thấy hoặc lỗi, trả về defaultBiologicalMetrics.
func (w *DailyMenuCronWorker) fetchBiologicalMetrics(ctx context.Context, userID string) service.BiologicalMetrics {
	if w.profileClient == nil {
		slog.WarnContext(ctx, "profileClient not configured, using fallback metrics", slog.String("user_id", userID))
		return getDefaultBiologicalMetrics()
	}

	profileMetrics, err := w.profileClient.GetBiologicalMetrics(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "Could not fetch profile metrics, using fallback", slog.String("user_id", userID), slog.String("error", err.Error()))
		return getDefaultBiologicalMetrics()
	}
	if profileMetrics == nil {
		slog.WarnContext(ctx, "User profile not found, using fallback", slog.String("user_id", userID))
		return getDefaultBiologicalMetrics()
	}

	return service.BiologicalMetrics{
		WeightKg:      profileMetrics.WeightKg,
		HeightCm:      profileMetrics.HeightCm,
		Age:           profileMetrics.Age,
		Gender:        profileMetrics.Gender,
		ActivityLevel: profileMetrics.ActivityLevel,
	}
}

// Stop dừng cron worker.
func (w *DailyMenuCronWorker) Stop() {
	close(w.stopChan)
}
