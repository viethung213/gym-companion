package port

import "context"

type EventRecorder interface {
	RecordEvents(ctx context.Context, events []any) error
}
