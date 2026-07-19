package persistence

import (
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
	"gorm.io/gorm"
)

func insertOutbox(tx *gorm.DB, event *domain.Event) error {
	if event == nil {
		return nil
	}

	record := outboxRecord{
		ID:           event.ID,
		EventID:      event.ID,
		EventType:    event.Type,
		Payload:      event.Payload,
		PartitionKey: event.PartitionKey,
		CreatedAt:    event.CreatedAt,
	}
	if err := tx.Create(&record).Error; err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}

	return nil
}
