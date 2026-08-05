package worker

import (
	"context"
	"log"
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

// defaultBiologicalMetrics là fallback khi không lấy được profile thực của user.
var defaultBiologicalMetrics = service.BiologicalMetrics{
	WeightKg:      70.0,
	HeightCm:      170.0,
	Age:           25,
	Gender:        "MALE",
	ActivityLevel: "MODERATELY_ACTIVE",
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
	log.Printf("[DailyMenuCronWorker] Started — sẽ sinh thực đơn cho user hoạt động lúc 5:00 AM mỗi ngày")

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
	log.Printf("[DailyMenuCronWorker] 5:00 AM Cron Triggered: đang tải danh sách user hoạt động...")

	activeUserIDs, err := w.planRepo.FindActiveUserIDs(ctx, activeUserWithinDays)
	if err != nil {
		log.Printf("[DailyMenuCronWorker] Lỗi lấy active users: %v", err)
		return
	}

	if len(activeUserIDs) == 0 {
		log.Printf("[DailyMenuCronWorker] Không có user hoạt động trong %d ngày qua — bỏ qua.", activeUserWithinDays)
		return
	}

	log.Printf("[DailyMenuCronWorker] Bắt đầu sinh thực đơn cho %d user (max %d concurrent AI calls)...",
		len(activeUserIDs), maxConcurrentAICalls)

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
				log.Printf("[DailyMenuCronWorker] Lỗi sinh thực đơn user %s: %v", uid, genErr)
			} else {
				successCount++
			}
			mu.Unlock()
		}(userID)
	}

	wg.Wait()
	log.Printf("[DailyMenuCronWorker] Hoàn thành: %d thành công, %d thất bại / tổng %d user",
		successCount, failCount, len(activeUserIDs))
}

// fetchBiologicalMetrics lấy chỉ số sinh trắc học từ ProfileService.
// Nếu profile không tìm thấy hoặc lỗi, trả về defaultBiologicalMetrics.
func (w *DailyMenuCronWorker) fetchBiologicalMetrics(ctx context.Context, userID string) service.BiologicalMetrics {
	if w.profileClient == nil {
		log.Printf("[DailyMenuCronWorker] profileClient chưa được cấu hình, dùng fallback cho user %s", userID)
		return defaultBiologicalMetrics
	}

	profileMetrics, err := w.profileClient.GetBiologicalMetrics(ctx, userID)
	if err != nil {
		log.Printf("[DailyMenuCronWorker] Không lấy được profile user %s (err: %v), dùng fallback", userID, err)
		return defaultBiologicalMetrics
	}
	if profileMetrics == nil {
		log.Printf("[DailyMenuCronWorker] User %s chưa có profile, dùng fallback", userID)
		return defaultBiologicalMetrics
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
