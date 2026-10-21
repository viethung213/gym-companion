package port

import "context"

// OutboxLogRepository quản lý việc ghi nhận các incoming events để đảm bảo tính Idempotency
type OutboxLogRepository interface {
	Save(ctx context.Context, eventID, eventType string, payload []byte, partitionKey, status, errMsg string) error
	IsProcessed(ctx context.Context, eventID string) (bool, error)
}
