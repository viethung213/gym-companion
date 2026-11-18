package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	domainEvent "github.com/viethung213/gym-companion/internal/nutrition/domain/event"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// Compile-time check.
var _ *aggregate.NutritionPlan = nil

// GenerateDailyPlanCommand yêu cầu sinh thực đơn ngày mới cho người dùng.
type GenerateDailyPlanCommand struct {
	UserID               string
	PlanDate             time.Time
	BiologicalMetrics    service.BiologicalMetrics
	NutritionGoal        vo.NutritionGoal // mục tiêu dài hạn ảnh hưởng đến macro ratio
	UserRestrictions     []string
	PreferredIngredients []string // Ưu tiên chọn các nguyên liệu này nếu có trong catalog
	WeeklyCookingStyle   string   // Phong cách nấu ăn thường xuyên từ insight tuần trước
}

type GenerateDailyPlanHandler struct {
	planRepo       repository.NutritionPlanRepository
	historyRepo    repository.MealHistoryRepository
	insightRepo    repository.WeeklyInsightRepository
	eventRecorder  port.EventRecorder
	tdeeCalculator *service.TDEECalculator
	menuGenerator  *service.MenuGenerator
	txManager      port.TransactionManager
}

func NewGenerateDailyPlanHandler(
	planRepo repository.NutritionPlanRepository,
	historyRepo repository.MealHistoryRepository,
	eventRecorder port.EventRecorder,
	tdeeCalculator *service.TDEECalculator,
	menuGenerator *service.MenuGenerator,
	txManager ...port.TransactionManager,
) *GenerateDailyPlanHandler {
	var tm port.TransactionManager
	if len(txManager) > 0 {
		tm = txManager[0]
	}
	return &GenerateDailyPlanHandler{
		planRepo:       planRepo,
		historyRepo:    historyRepo,
		eventRecorder:  eventRecorder,
		tdeeCalculator: tdeeCalculator,
		menuGenerator:  menuGenerator,
		txManager:      tm,
	}
}

// WithWeeklyInsightRepo gắn WeeklyInsightRepository để lấy dữ liệu phong cách và nguyên liệu ưa thích của tuần trước.
func (h *GenerateDailyPlanHandler) WithWeeklyInsightRepo(insightRepo repository.WeeklyInsightRepository) *GenerateDailyPlanHandler {
	h.insightRepo = insightRepo
	return h
}

//nolint:gocritic // cmd is passed by value per command pattern
func (h *GenerateDailyPlanHandler) Handle(ctx context.Context, cmd GenerateDailyPlanCommand) (*aggregate.NutritionPlan, error) {
	existing, err := h.planRepo.FindByUserIDAndDate(ctx, cmd.UserID, cmd.PlanDate)
	if err == nil && existing != nil {
		return existing, nil
	}

	goal := cmd.NutritionGoal
	if goal == "" {
		goal = vo.GoalMaintenance
	}

	allocation, calcErr := h.tdeeCalculator.CalculateBaseTDEE(cmd.BiologicalMetrics, goal)
	if calcErr != nil {
		return nil, fmt.Errorf("generate daily plan: %w", calcErr)
	}

	var recentConsumptions []vo.HistoricalConsumption

	// 1. Lấy history trong 7 ngày gần nhất để xác định các bữa ăn và ngày user đã thực sự cập nhật
	sevenDaysAgo := cmd.PlanDate.AddDate(0, 0, -7)
	history, histErr := h.historyRepo.FindByUserIDAndDateRange(ctx, cmd.UserID, sevenDaysAgo, cmd.PlanDate)
	if histErr != nil || history == nil {
		history, _ = h.historyRepo.FindByUserID(ctx, cmd.UserID)
	}

	if history != nil && len(history.MealLogs()) > 0 {
		mealLogs := history.MealLogs()

		// Thu thập các ngày duy nhất mà user đã log bữa ăn trong 7 ngày qua
		dateMap := make(map[string]time.Time)
		for _, logItem := range mealLogs {
			d := logItem.LoggedAt()
			if d.After(sevenDaysAgo) || d.Equal(sevenDaysAgo) {
				dateStr := d.Format("2006-01-02")
				dateMap[dateStr] = d
			}
		}

		// 2. Chỉ truy vấn các plan trong các ngày có trong history (tránh query dư thừa toàn bộ 7 ngày)
		plansByDate := make(map[string]*aggregate.NutritionPlan)
		if len(dateMap) > 0 {
			dates := make([]time.Time, 0, len(dateMap))
			for _, d := range dateMap {
				dates = append(dates, d)
			}
			plans, planErr := h.planRepo.FindByUserIDAndDates(ctx, cmd.UserID, dates)
			if planErr == nil {
				for _, p := range plans {
					plansByDate[p.PlanDate().Format("2006-01-02")] = p
				}
			}
		}

		// 3. Với từng bữa ăn user đã log, đi thẳng vào đúng bữa (mealType) trong plan ngày đó để lấy nguyên liệu
		for _, logItem := range mealLogs {
			logDate := logItem.LoggedAt()
			if logDate.Before(sevenDaysAgo) {
				continue
			}
			dateKey := logDate.Format("2006-01-02")
			plan := plansByDate[dateKey]

			var ingredients []string
			if plan != nil {
				targetMealType := strings.ToUpper(strings.TrimSpace(logItem.MealType()))
				logNameUpper := strings.ToUpper(strings.TrimSpace(logItem.MealName()))

				// Đi thẳng vào bữa tương ứng mà user đã log
				for _, m := range plan.DailyMeals() {
					if strings.ToUpper(strings.TrimSpace(m.MealType())) == targetMealType {
						for _, opt := range m.Options() {
							optNameUpper := strings.ToUpper(strings.TrimSpace(opt.MealName()))
							if opt.IsLogged() || optNameUpper == logNameUpper ||
								strings.Contains(optNameUpper, logNameUpper) || strings.Contains(logNameUpper, optNameUpper) {
								for _, ing := range opt.Ingredients() {
									ingredients = append(ingredients, ing.IngredientName())
								}
								break
							}
						}
						break
					}
				}
			}

			// Ghi nhận 1 bữa ăn = 1 HistoricalConsumption chứa danh sách nguyên liệu
			recentConsumptions = append(recentConsumptions, vo.HistoricalConsumption{
				MealName:    logItem.MealName(),
				Ingredients: ingredients,
				ConsumedAt:  logDate,
			})
		}
	}

	var personalization service.DailyPlanPersonalization
	personalization.RecentConsumptions = recentConsumptions
	if h.insightRepo != nil {
		latestInsight, insightErr := h.insightRepo.GetLatestByUserID(ctx, cmd.UserID)
		if insightErr == nil && latestInsight != nil {
			// Đảm bảo CHỈ LẤY ĐÚNG TUẦN VỪA RỒI:
			// Khoảng cách giữa ngày lập kế hoạch (PlanDate) và ngày kết thúc tuần đánh giá (EndDate)
			// phải nằm trong khoảng tối đa 8 ngày (7 ngày của tuần hiện tại + 1 ngày dung sai mốc chạy sáng Chủ Nhật).
			// Nếu người dùng đã nghỉ 2 tuần hoặc 1 tháng, dữ liệu cũ không còn phản ánh thói quen tuần vừa rồi -> Bỏ qua.
			timeSinceLastInsight := cmd.PlanDate.Sub(latestInsight.EndDate())
			if timeSinceLastInsight >= 0 && timeSinceLastInsight <= 8*24*time.Hour {
				personalization.PreferredIngredients = latestInsight.PreferredIngredients()
				personalization.WeeklyCookingStyle = latestInsight.PredictedCookingStyle()
			}
		}
	}
	if len(cmd.PreferredIngredients) > 0 {
		personalization.PreferredIngredients = cmd.PreferredIngredients
	}
	if cmd.WeeklyCookingStyle != "" {
		personalization.WeeklyCookingStyle = cmd.WeeklyCookingStyle
	}

	// External AI Generation is intentionally executed outside the DB transaction to prevent connection bottleneck
	plan, genErr := h.menuGenerator.GenerateDailyPlan(
		ctx,
		cmd.UserID,
		cmd.PlanDate,
		allocation,
		cmd.UserRestrictions,
		goal,
		personalization,
	)
	if genErr != nil {
		return nil, fmt.Errorf("generate daily plan: %w", genErr)
	}

	savedSchedules, _ := h.planRepo.GetUserMealSchedules(ctx, cmd.UserID)
	if len(savedSchedules) > 0 {
		plan.UpdateMealSchedule(savedSchedules)
	}

	// Wrap local DB persistence and outbox event recording in an atomic transaction
	saveFunc := func(txCtx context.Context) error {
		if saveErr := h.planRepo.Save(txCtx, plan); saveErr != nil {
			return fmt.Errorf("generate daily plan save: %w", saveErr)
		}

		if h.eventRecorder != nil {
			ev := domainEvent.NewNutritionPlanGeneratedEvent(
				plan.ID(),
				plan.UserID(),
				plan.PlanDate(),
				time.Now(),
			)
			if err := h.eventRecorder.RecordEvents(txCtx, []any{ev}); err != nil {
				return fmt.Errorf("record nutrition plan generated event: %w", err)
			}
		}
		return nil
	}

	if h.txManager != nil {
		if err := h.txManager.WithTransaction(ctx, saveFunc); err != nil {
			return nil, err
		}
	} else {
		if err := saveFunc(ctx); err != nil {
			return nil, err
		}
	}

	return plan, nil
}
