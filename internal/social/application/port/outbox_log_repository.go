package port

import "context"

type OutboxLogRepository interface {
	Save(ctx context.Context, eventID, eventType string, payload []byte, partitionKey, status string, errMsg string) error
	IsProcessed(ctx context.Context, eventID string) (bool, error)
}
