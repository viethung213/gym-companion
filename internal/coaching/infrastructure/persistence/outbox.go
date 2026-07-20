package persistence

import (
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
	"github.com/viethung213/gym-companion/internal/shared/cloudevent"
	"gorm.io/gorm"
)

const coachingEventSource = "services/coaching-service"

func insertOutbox(tx *gorm.DB, event *domain.Event) error {
	if event == nil {
		return nil
	}

	payload, err := cloudevent.Encode(
		event.ID,
		coachingEventSource,
		event.Type,
		event.CreatedAt,
		event.Payload,
	)
	if err != nil {
		return fmt.Errorf("encode outbox CloudEvent: %w", err)
	}

	record := outboxRecord{
		ID:           event.ID,
		EventID:      event.ID,
		EventType:    event.Type,
		Payload:      payload,
		PartitionKey: event.PartitionKey,
		CreatedAt:    event.CreatedAt,
	}
	if err := tx.Create(&record).Error; err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}

	return nil
}
