package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

// GenerateWeeklyInsightCommand kích hoạt việc phân tích và lưu đánh giá tuần vào DB.
type GenerateWeeklyInsightCommand struct {
	UserID   string
	GoalType string
}

// GenerateWeeklyInsightHandler xử lý nghiệp vụ gom dữ liệu 7 ngày, gọi AI và lưu kết quả vào DB.
type GenerateWeeklyInsightHandler struct {
	planRepo    repository.NutritionPlanRepository
	historyRepo repository.MealHistoryRepository
	insightRepo repository.WeeklyInsightRepository
	aiService   repository.AIService
}

// NewGenerateWeeklyInsightHandler khởi tạo handler.
func NewGenerateWeeklyInsightHandler(
	planRepo repository.NutritionPlanRepository,
	historyRepo repository.MealHistoryRepository,
	insightRepo repository.WeeklyInsightRepository,
	aiService repository.AIService,
) *GenerateWeeklyInsightHandler {
	return &GenerateWeeklyInsightHandler{
		planRepo:    planRepo,
		historyRepo: historyRepo,
		insightRepo: insightRepo,
		aiService:   aiService,
	}
}

// Handle tổng hợp số liệu 7 ngày, gọi AI và lưu vào DB.
func (h *GenerateWeeklyInsightHandler) Handle(
	ctx context.Context,
	cmd GenerateWeeklyInsightCommand,
) (*entity.WeeklyInsight, error) {
	if cmd.UserID == "" {
		return nil, errors.New("generate weekly insight: user_id is required")
	}

	now := time.Now()
	const fixedDays = 7

	startDate := now.AddDate(0, 0, -(fixedDays - 1))
	endDate := now

	// 1. Chống trùng lặp đa Node (Idempotency Check): Nếu tuần này đã có insight rồi thì trả về luôn, không gọi LLM lại
	existing, err := h.insightRepo.GetByUserIDAndStartDate(ctx, cmd.UserID, startDate)
	if err == nil && existing != nil {
		return existing, nil
	}

	// 2. Chỉ query meal logs trong 7 ngày gần nhất thay vì toàn bộ lịch sử (tránh query dư thừa)
	history, err := h.historyRepo.FindByUserIDAndDateRange(ctx, cmd.UserID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("generate weekly insight: meal history query failed: %w", err)
	}

	// 3. Nghiệp vụ: Nếu liên tục 7 ngày không có dữ liệu bữa ăn nào thì bỏ qua, không gọi LLM và không lưu lại
	if history == nil || len(history.MealLogs()) == 0 {
		return nil, nil
	}

	var totalConsumedCal, totalTargetCal float64
	var totalConsumedProt, totalTargetProt float64
	var totalConsumedCarb, totalTargetCarb float64
	var totalConsumedFat, totalTargetFat float64
	totalLoggedCount := 0

	weeklySlots := make([]repository.DailyMealSlotRecord, 0, fixedDays)
	logs := history.MealLogs()

	for i := fixedDays - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		dateStr := day.Format("2006-01-02")
		consumed, protein, carbs, fat := history.CalculateConsumedToday(day)

		totalConsumedCal += consumed
		totalConsumedProt += protein
		totalConsumedCarb += carbs
		totalConsumedFat += fat

		plan, planErr := h.planRepo.FindByUserIDAndDate(ctx, cmd.UserID, day)
		if planErr == nil && plan != nil {
			alloc := plan.CalorieAllocation()
			totalTargetCal += alloc.TargetCalories()
			totalTargetProt += alloc.ProteinGrams()
			totalTargetCarb += alloc.CarbGrams()
			totalTargetFat += alloc.FatGrams()
		}

		slotRecord := repository.DailyMealSlotRecord{Date: dateStr}
		dy, dm, dd := day.Date()

		var bFastNames, lunchNames, dinnerNames, snackNames []string
		for idx := range logs {
			ly, lm, ld := logs[idx].LoggedAt().Date()
			if ly == dy && lm == dm && ld == dd {
				totalLoggedCount++
				mType := strings.ToUpper(strings.TrimSpace(logs[idx].MealType()))
				mName := strings.TrimSpace(logs[idx].MealName())
				if mName == "" {
					continue
				}
				switch mType {
				case "BREAKFAST":
					bFastNames = append(bFastNames, mName)
				case "LUNCH":
					lunchNames = append(lunchNames, mName)
				case "DINNER":
					dinnerNames = append(dinnerNames, mName)
				case "SNACK":
					snackNames = append(snackNames, mName)
				}
			}
		}

		slotRecord.Breakfast = strings.Join(bFastNames, ", ")
		slotRecord.Lunch = strings.Join(lunchNames, ", ")
		slotRecord.Dinner = strings.Join(dinnerNames, ", ")
		slotRecord.Snack = strings.Join(snackNames, ", ")

		weeklySlots = append(weeklySlots, slotRecord)
	}

	// Nghiệp vụ: Nếu sau khi gom 7 ngày mà số bữa ăn được ghi nhận là 0, bỏ qua không gọi LLM và không lưu lại
	if totalLoggedCount == 0 {
		return nil, nil
	}

	goal := strings.TrimSpace(cmd.GoalType)
	if goal == "" {
		goal = "MAINTENANCE"
	}

	insightCtx := repository.InsightPromptContext{
		UserID:   cmd.UserID,
		GoalType: goal,
		WeeklyMacroSummary: repository.WeeklyMacroSummary{
			TotalConsumedCalories: totalConsumedCal,
			TotalTargetCalories:   totalTargetCal,
			TotalConsumedProtein:  totalConsumedProt,
			TotalTargetProtein:    totalTargetProt,
			TotalConsumedCarbs:    totalConsumedCarb,
			TotalTargetCarbs:      totalTargetCarb,
			TotalConsumedFat:      totalConsumedFat,
			TotalTargetFat:        totalTargetFat,
			LoggedMealsCount:      totalLoggedCount,
			TotalExpectedMeals:    fixedDays * 4,
		},
		WeeklyMealSlots: weeklySlots,
	}

	aiResult, err := h.aiService.GenerateNutritionInsight(ctx, insightCtx)
	if err != nil {
		return nil, fmt.Errorf("generate weekly insight: ai analysis failed: %w", err)
	}

	areas := make([]entity.WeeklyImprovementArea, 0, len(aiResult.ImprovementAreas))
	for _, a := range aiResult.ImprovementAreas {
		areas = append(areas, entity.WeeklyImprovementArea{
			Nutrient:    a.Nutrient,
			Action:      a.Action,
			DeltaAmount: a.DeltaAmount,
			Suggestion:  a.Suggestion,
		})
	}

	summary := entity.WeeklyMacroSummary{
		ConsumedCalories: totalConsumedCal,
		TargetCalories:   totalTargetCal,
		ConsumedProtein:  totalConsumedProt,
		TargetProtein:    totalTargetProt,
		ConsumedCarbs:    totalConsumedCarb,
		TargetCarbs:      totalTargetCarb,
		ConsumedFat:      totalConsumedFat,
		TargetFat:        totalTargetFat,
		LoggedMealsCount: totalLoggedCount,
		TotalMealsCount:  fixedDays * 4,
	}

	weeklyInsight := entity.NewWeeklyInsight(
		uuid.New().String(),
		cmd.UserID,
		startDate,
		endDate,
		aiResult.OverallScore,
		aiResult.Summary,
		aiResult.PredictedCookingStyle,
		aiResult.PreferredIngredients,
		areas,
		summary,
		now,
		now,
	)

	if err := h.insightRepo.Save(ctx, weeklyInsight); err != nil {
		return nil, fmt.Errorf("generate weekly insight: save to db: %w", err)
	}

	return weeklyInsight, nil
}
