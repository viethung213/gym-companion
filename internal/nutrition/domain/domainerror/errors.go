package domainerror

import "errors"

var (
	ErrInvalidCalories         = errors.New("target calories must be at least 1200 kcal")
	ErrMealOptionNotFound      = errors.New("meal option not found in nutrition plan")
	ErrPlanAlreadyLogged       = errors.New("meal option has already been logged")
	ErrInvalidStatusTransition = errors.New("invalid food item status transition")
	ErrItemLocked              = errors.New("ingredient or source is currently locked under lockout rules")
	ErrEmptyCatalog            = errors.New("active food catalog is empty")
	ErrAllergenConflict        = errors.New("ingredient conflicts with user allergen restrictions")
	ErrMealAlreadyLogged       = errors.New("cannot recalibrate a meal that has already been logged")
	ErrMealTypeNotFound        = errors.New("specified meal type does not exist in daily plan")
	ErrMealTypeRequired        = errors.New("meal type is required")
)
