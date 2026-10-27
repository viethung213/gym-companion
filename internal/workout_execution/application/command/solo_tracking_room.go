package command

import (
	"context"
	"log/slog"
	"time"

	domainEvent "github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
)

// checkAndEmitRoomReady checks if both participants are present in the room and emits SoloMatchRoomReady if ready.
func (s *SoloTrackingService) checkAndEmitRoomReady(ctx context.Context, matchID string) (bool, error) {
	if s.liveSync == nil {
		return false, nil
	}
	ready, err := s.liveSync.IsMatchReady(ctx, matchID)
	if err != nil {
		return false, err
	}
	if ready {
		s.emitRoomReadyEvent(ctx, matchID)
	}
	return ready, nil
}

// CheckMatchReady checks if both participants are present in the room and emits SoloMatchRoomReady if ready.
func (s *SoloTrackingService) CheckMatchReady(ctx context.Context, matchID string) (bool, error) {
	return s.checkAndEmitRoomReady(ctx, matchID)
}

// emitRoomReadyEvent emits a SoloMatchRoomReady event to the outbox once per matchID.
func (s *SoloTrackingService) emitRoomReadyEvent(ctx context.Context, matchID string) {
	if _, loaded := s.readyEmitted.LoadOrStore(matchID, true); loaded {
		return
	}
	if s.outbox == nil {
		return
	}
	readyEvt := &domainEvent.SoloMatchRoomReady{
		MatchID:   matchID,
		StartedAt: time.Now().UTC(),
	}
	if err := s.outbox.WriteEvents(ctx, "SoloMatch", matchID, []interface{}{readyEvt}); err != nil {
		slog.WarnContext(ctx, "failed to write SoloMatchRoomReady event to outbox",
			slog.String("match_id", matchID),
			slog.Any("err", err),
		)
	} else {
		slog.InfoContext(ctx, "SoloMatchRoomReady event emitted to outbox",
			slog.String("match_id", matchID),
		)
	}
}
