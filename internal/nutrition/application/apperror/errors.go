package apperror

import "errors"

var (
	ErrInvalidInput          = errors.New("invalid request parameters")
	ErrNutritionPlanNotFound = errors.New("nutrition plan not found")
	ErrFoodItemNotFound      = errors.New("food item not found")
	ErrMealHistoryNotFound   = errors.New("meal history not found")
	ErrEventPublishFailed    = errors.New("failed to publish outbox event to message broker")
	ErrTransactionFailed     = errors.New("database transaction failed")
	ErrAIServiceUnavailable  = errors.New("external AI creative chef service unavailable")
	ErrInternal              = errors.New("internal server error")
)
