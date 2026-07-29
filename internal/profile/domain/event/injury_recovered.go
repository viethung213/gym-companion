package event

import (
	"time"

	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
)

type InjuryRecoveredEvent struct {
	userID      string
	injury      *entity.Injury
	recoveredAt time.Time
}

func NewInjuryRecoveredEvent(userID string, injury *entity.Injury, recoveredAt time.Time) *InjuryRecoveredEvent {
	if recoveredAt.IsZero() {
		recoveredAt = time.Now()
	}
	return &InjuryRecoveredEvent{
		userID:      userID,
		injury:      injury,
		recoveredAt: recoveredAt,
	}
}

func (e *InjuryRecoveredEvent) UserID() string {
	return e.userID
}

func (e *InjuryRecoveredEvent) Injury() *entity.Injury {
	return e.injury
}

func (e *InjuryRecoveredEvent) RecoveredAt() time.Time {
	return e.recoveredAt
}
