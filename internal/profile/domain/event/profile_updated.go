package event

import (
	"time"

	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type ProfileUpdatedEvent struct {
	userID            string
	biologicalMetrics vo.BiologicalMetrics
	goals             []string
	completionRate    float64
	updatedAt         time.Time
}

func NewProfileUpdatedEvent(
	userID string,
	bio vo.BiologicalMetrics,
	goals []string,
	completionRate float64,
	updatedAt time.Time,
) *ProfileUpdatedEvent {
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	return &ProfileUpdatedEvent{
		userID:            userID,
		biologicalMetrics: bio,
		goals:             goals,
		completionRate:    completionRate,
		updatedAt:         updatedAt,
	}
}

func (e *ProfileUpdatedEvent) UserID() string                          { return e.userID }
func (e *ProfileUpdatedEvent) BiologicalMetrics() vo.BiologicalMetrics { return e.biologicalMetrics }
func (e *ProfileUpdatedEvent) Goals() []string                         { return e.goals }
func (e *ProfileUpdatedEvent) CompletionRate() float64                 { return e.completionRate }
func (e *ProfileUpdatedEvent) UpdatedAt() time.Time                    { return e.updatedAt }
