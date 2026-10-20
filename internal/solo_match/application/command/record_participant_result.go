package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/event"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/service"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

type RecordParticipantResultCommand struct {
	MatchID     string
	UserID      string
	ScoreValue  float64
	CompletedAt time.Time
}

type RecordParticipantResultHandler struct {
	matchRepo repository.SoloMatchRepository
	eventPub  port.EventPublisher
	botRepo   repository.BotRepository
	txManager port.TransactionManager
	resolver  *service.MatchResolver
}

func NewRecordParticipantResultHandler(
	matchRepo repository.SoloMatchRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
	resolver *service.MatchResolver,
	botRepo repository.BotRepository,
) *RecordParticipantResultHandler {
	if resolver == nil {
		resolver = service.NewMatchResolver()
	}
	return &RecordParticipantResultHandler{
		matchRepo: matchRepo,
		eventPub:  eventPub,
		botRepo:   botRepo,
		txManager: txManager,
		resolver:  resolver,
	}
}

func (h *RecordParticipantResultHandler) Handle(ctx context.Context, cmd RecordParticipantResultCommand) error {
	match, err := h.matchRepo.FindByID(ctx, cmd.MatchID)
	if err != nil {
		return fmt.Errorf("find match %s: %w", cmd.MatchID, err)
	}
	if match == nil {
		return fmt.Errorf("match %s not found", cmd.MatchID)
	}

	completedAt := cmd.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}

	if err := match.RecordParticipantResult(cmd.UserID, cmd.ScoreValue, completedAt, h.resolver); err != nil {
		return fmt.Errorf("record participant result: %w", err)
	}

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.matchRepo.Save(txCtx, match); err != nil {
			return fmt.Errorf("save match: %w", err)
		}

		if match.Status() == valueobject.MatchStatusCompleted {
			p1 := match.Participants()[0]
			p2 := match.Participants()[1]
			var botUserID string
			if h.botRepo != nil {
				if isBot, _ := h.botRepo.IsBot(txCtx, p1.UserID()); isBot {
					botUserID = p1.UserID()
				} else if isBot, _ := h.botRepo.IsBot(txCtx, p2.UserID()); isBot {
					botUserID = p2.UserID()
				}
			}

			winnerID := ""
			if match.WinnerID() != nil {
				winnerID = *match.WinnerID()
			}

			endedAt := time.Now().UTC()
			if match.EndedAt() != nil {
				endedAt = *match.EndedAt()
			}

			ev := &event.SoloMatchCompleted{
				MatchID:       match.ID(),
				ExerciseID:    match.ExerciseID(),
				ChallengeType: match.ChallengeType().String(),
				WinnerID:      winnerID,
				IsDraw:        match.IsDraw(),
				Player1ID:     p1.UserID(),
				Player1Score:  p1.Score().Value(),
				Player2ID:     p2.UserID(),
				Player2Score:  p2.Score().Value(),
				BotUserID:     botUserID,
				CompletedAt:   endedAt,
			}
			if err := h.eventPub.PublishEvents(txCtx, []any{ev}); err != nil {
				return fmt.Errorf("publish outbox event: %w", err)
			}
		}

		return nil
	})
}
