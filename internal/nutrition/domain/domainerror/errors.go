package domainerror

import "errors"

var (
	ErrInvalidCalories         = errors.New("target calories must be at least 1200 kcal")
	ErrMealOptionNotFound      = errors.New("meal option not found in nutrition plan")
	ErrPlanAlreadyLogged       = errors.New("meal option has already been logged")
	ErrInvalidStatusTransition = errors.New("invalid food item status transition")
	ErrItemLocked              = errors.New("ingredient or source is currently locked under lockout rules")
	ErrEmptyCatalog            = errors.New("active food catalog is empty")
)
