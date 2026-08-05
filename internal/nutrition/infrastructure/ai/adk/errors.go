package adk

import "errors"

// ErrNutritionPlanFailed được trả về khi tất cả các lượt thử đều thất bại và Degraded Mode không khả thi.
var ErrNutritionPlanFailed = errors.New("nutrition plan generation failed")
