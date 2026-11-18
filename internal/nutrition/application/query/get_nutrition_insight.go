package query

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

// GetNutritionInsightQuery truy vấn đánh giá tuần đã lưu trong DB.
type GetNutritionInsightQuery struct {
	UserID        string
	WeekStartDate string // Tùy chọn (YYYY-MM-DD), nếu để trống sẽ lấy tuần gần nhất
}

// GetNutritionInsightHandler xử lý GetNutritionInsightQuery mà KHÔNG gọi LLM.
type GetNutritionInsightHandler struct {
	insightRepo repository.WeeklyInsightRepository
}

// NewGetNutritionInsightHandler khởi tạo GetNutritionInsightHandler.
func NewGetNutritionInsightHandler(
	insightRepo repository.WeeklyInsightRepository,
) *GetNutritionInsightHandler {
	return &GetNutritionInsightHandler{
		insightRepo: insightRepo,
	}
}

// Handle truy vấn DB và trả về kết quả đánh giá tuần nhanh chóng.
func (h *GetNutritionInsightHandler) Handle(
	ctx context.Context,
	q GetNutritionInsightQuery,
) (*entity.WeeklyInsight, error) {
	if q.UserID == "" {
		return nil, errors.New("get nutrition insight: user_id is required")
	}

	if q.WeekStartDate != "" {
		startDate, err := time.Parse("2006-01-02", q.WeekStartDate)
		if err == nil {
			insight, err := h.insightRepo.GetByUserIDAndStartDate(ctx, q.UserID, startDate)
			if err != nil {
				return nil, fmt.Errorf("get nutrition insight by start date: %w", err)
			}
			return insight, nil
		}
	}

	insight, err := h.insightRepo.GetLatestByUserID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("get latest nutrition insight: %w", err)
	}
	return insight, nil
}
