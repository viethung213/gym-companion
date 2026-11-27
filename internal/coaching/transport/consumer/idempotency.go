package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

// handleOnce runs handle for an event id until it succeeds once (D9):
//
//   - the event is claimed in coaching.outbox_log before handle runs;
//   - on success the claim becomes "processed", and later deliveries of the
//     same id are skipped as duplicates;
//   - on failure the claim is released, so the consumer's retries are not
//     mistaken for duplicates;
//   - a claim left by a process that died mid-handling is taken over by the
//     redelivery (the consumer commits offsets only after handling).
//
// Once the consumer loop (sharedKafka.RunConsumerLoop) has exhausted its
// retries it publishes the event to the dead-letter topic and commits the
// offset; the released claim lets a replay from that topic run it again.
func handleOnce(
	ctx context.Context,
	outbox port.OutboxRepository,
	env *cloudEventEnvelope,
	rawValue []byte,
	handle func() error,
) error {
	claimed, err := outbox.ClaimEvent(ctx, env.ID, env.Type, rawValue)
	if err != nil {
		return fmt.Errorf("claim event: %w", err)
	}
	if !claimed {
		slog.DebugContext(ctx, "Duplicate event skipped",
			slog.String("event_id", env.ID), slog.String("event_type", env.Type))
		return nil
	}

	// Settle the claim even when the caller's context is already done, so the
	// outcome is recorded rather than left to the redelivery.
	settleCtx := context.WithoutCancel(ctx)

	handleErr := handle()
	if handleErr == nil {
		if doneErr := outbox.CompleteEvent(settleCtx, env.ID); doneErr != nil {
			// The work is done; at worst a redelivery runs it again, which
			// every handler tolerates.
			slog.WarnContext(ctx, "Could not mark event processed",
				slog.String("event_id", env.ID), slog.Any("error", doneErr))
		}
		return nil
	}

	if releaseErr := outbox.ReleaseEvent(settleCtx, env.ID); releaseErr != nil {
		return errors.Join(handleErr, fmt.Errorf("release event %s for retry: %w", env.ID, releaseErr))
	}
	return handleErr
}

// occurredAt is the event's CloudEvents time, zero when absent or malformed.
func (e *cloudEventEnvelope) occurredAt() time.Time {
	t, err := time.Parse(time.RFC3339Nano, e.Time)
	if err != nil {
		return time.Time{}
	}
	return t
}
