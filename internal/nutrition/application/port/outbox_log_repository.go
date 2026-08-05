package port

import "context"

type OutboxLogRecord struct {
	ID           string
	EventID      string
	EventType    string
	Payload      []byte
	PartitionKey string
	Status       string
	ErrorMessage string
}

type OutboxLogRepository interface {
	IsProcessed(ctx context.Context, eventID string) (bool, error)
	SaveLog(ctx context.Context, logRecord *OutboxLogRecord) error
}
