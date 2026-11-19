package command

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	domainEvent "github.com/viethung213/gym-companion/internal/nutrition/domain/event"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

// RebalanceWorkoutNutritionCommand chứa dữ liệu sự kiện tập luyện để tái cân bằng năng lượng.
type RebalanceWorkoutNutritionCommand struct {
	UserID      string
	PlanDate    time.Time
	SessionID   string
	TotalVolume float64
}

// RebalanceWorkoutNutritionHandler xử lý bù calo và điều chỉnh khẩu phần các bữa chưa ăn (BR-NU-04).
type RebalanceWorkoutNutritionHandler struct {
	planRepo      repository.NutritionPlanRepository
	eventRecorder port.EventRecorder
	txManager     port.TransactionManager
}

func NewRebalanceWorkoutNutritionHandler(
	planRepo repository.NutritionPlanRepository,
	eventRecorder port.EventRecorder,
	txManager ...port.TransactionManager,
) *RebalanceWorkoutNutritionHandler {
	var tm port.TransactionManager
	if len(txManager) > 0 {
		tm = txManager[0]
	}
	return &RebalanceWorkoutNutritionHandler{
		planRepo:      planRepo,
		eventRecorder: eventRecorder,
		txManager:     tm,
	}
}

// CalculateSurplusCalories ước lượng lượng calo tiêu hao cần bù sau buổi tập từ khối lượng tạ (volume).
func CalculateSurplusCalories(totalVolume float64) float64 {
	if totalVolume <= 0 {
		return 250.0 // Mức calo tiêu hao trung bình cho 1 buổi tập cơ bản
	}
	surplus := 150.0 + (totalVolume * 0.05)
	if surplus > 600.0 {
		surplus = 600.0
	}
	if surplus < 200.0 {
		surplus = 200.0
	}
	return math.Round(surplus)
}

func (h *RebalanceWorkoutNutritionHandler) Handle(ctx context.Context, cmd *RebalanceWorkoutNutritionCommand) (*aggregate.NutritionPlan, error) {
	if cmd == nil {
		return nil, errors.New("rebalance workout nutrition: command cannot be nil")
	}
	if cmd.UserID == "" {
		return nil, errors.New("rebalance workout nutrition: user_id is required")
	}

	plan, err := h.planRepo.FindByUserIDAndDate(ctx, cmd.UserID, cmd.PlanDate)
	if err != nil || plan == nil {
		return nil, fmt.Errorf("rebalance workout nutrition: plan not found for date %s: %w", cmd.PlanDate.Format("2006-01-02"), err)
	}

	surplusCalories := CalculateSurplusCalories(cmd.TotalVolume)
	if err := plan.AdjustWorkoutSurplus(surplusCalories); err != nil {
		return nil, fmt.Errorf("rebalance workout nutrition adjust: %w", err)
	}

	saveFunc := func(txCtx context.Context) error {
		if updateErr := h.planRepo.Update(txCtx, plan); updateErr != nil {
			return fmt.Errorf("rebalance workout nutrition update: %w", updateErr)
		}

		if h.eventRecorder != nil {
			ev := domainEvent.NewNutritionPlanRecalibratedEvent(
				plan.ID(),
				plan.UserID(),
				fmt.Sprintf("Post-workout surplus rebalance (+%.0f kcal for session %s)", surplusCalories, cmd.SessionID),
				time.Now(),
			)
			if err := h.eventRecorder.RecordEvents(txCtx, []any{ev}); err != nil {
				return fmt.Errorf("record nutrition plan recalibrated event: %w", err)
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
