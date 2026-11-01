package command

import "time"

type ProcessNutritionActivityCommand struct {
	UserID       string
	ActivityDate string
	RecordedAt   time.Time
}
