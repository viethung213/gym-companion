package event

import (
	"time"

	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
)

type InjuryReportedEvent struct {
	userID     string
	injury     *entity.Injury
	occurredAt time.Time
}

func NewInjuryReportedEvent(userID string, injury *entity.Injury) *InjuryReportedEvent {
	return &InjuryReportedEvent{
		userID:     userID,
		injury:     injury,
		occurredAt: time.Now(),
	}
}

func (e *InjuryReportedEvent) UserID() string         { return e.userID }
func (e *InjuryReportedEvent) Injury() *entity.Injury { return e.injury }
func (e *InjuryReportedEvent) OccurredAt() time.Time  { return e.occurredAt }
