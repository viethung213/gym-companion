package event

import (
	"time"

	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type ProfileCompletedEvent struct {
	userID                string
	biologicalMetrics     vo.BiologicalMetrics
	goals                 []string
	injuries              []*entity.Injury
	preferredWorkoutTimes []string
	completedAt           time.Time
}

func NewProfileCompletedEvent(
	userID string,
	bio vo.BiologicalMetrics,
	goals []string,
	injuries []*entity.Injury,
	preferredWorkoutTimes []string,
	completedAt time.Time,
) *ProfileCompletedEvent {
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	return &ProfileCompletedEvent{
		userID:                userID,
		biologicalMetrics:     bio,
		goals:                 goals,
		injuries:              injuries,
		preferredWorkoutTimes: preferredWorkoutTimes,
		completedAt:           completedAt,
	}
}

func (e *ProfileCompletedEvent) UserID() string                          { return e.userID }
func (e *ProfileCompletedEvent) BiologicalMetrics() vo.BiologicalMetrics { return e.biologicalMetrics }
func (e *ProfileCompletedEvent) Goals() []string                         { return e.goals }
func (e *ProfileCompletedEvent) Injuries() []*entity.Injury              { return e.injuries }
func (e *ProfileCompletedEvent) PreferredWorkoutTimes() []string         { return e.preferredWorkoutTimes }
func (e *ProfileCompletedEvent) CompletedAt() time.Time                  { return e.completedAt }
