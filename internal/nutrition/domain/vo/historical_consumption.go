package vo

import "time"

// HistoricalConsumption lưu vết bữa ăn đã tiêu thụ kèm danh sách nguyên liệu cấu thành để phục vụ tính toán suy giảm lặp lại.
type HistoricalConsumption struct {
	MealName    string
	Ingredients []string
	ConsumedAt  time.Time
}

// NewHistoricalConsumption tạo một bản ghi lịch sử tiêu thụ mới.
func NewHistoricalConsumption(
	mealName string,
	ingredients []string,
	consumedAt time.Time,
) HistoricalConsumption {
	return HistoricalConsumption{
		MealName:    mealName,
		Ingredients: ingredients,
		ConsumedAt:  consumedAt,
	}
}
