package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/event"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
)

type RecordParticipantAbortCommand struct {
	MatchID string
	UserID  string
	Reason  string
}

type RecordParticipantAbortHandler struct {
	matchRepo repository.SoloMatchRepository
	eventPub  port.EventPublisher
	botRepo   repository.BotRepository
	txManager port.TransactionManager
}

func NewRecordParticipantAbortHandler(
	matchRepo repository.SoloMatchRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
	botRepo repository.BotRepository,
) *RecordParticipantAbortHandler {
	return &RecordParticipantAbortHandler{
		matchRepo: matchRepo,
		eventPub:  eventPub,
		botRepo:   botRepo,
		txManager: txManager,
	}
}

func (h *RecordParticipantAbortHandler) Handle(ctx context.Context, cmd RecordParticipantAbortCommand) error {
	match, err := h.matchRepo.FindByID(ctx, cmd.MatchID)
	if err != nil {
		return fmt.Errorf("find match %s: %w", cmd.MatchID, err)
	}
	if match == nil {
		return fmt.Errorf("match %s not found", cmd.MatchID)
	}

	if err := match.ForfeitParticipant(cmd.UserID, cmd.Reason); err != nil {
		return fmt.Errorf("forfeit participant: %w", err)
	}

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.matchRepo.Save(txCtx, match); err != nil {
			return fmt.Errorf("save match: %w", err)
		}

		p1 := match.Participants()[0]
		p2 := match.Participants()[1]

		var opponentID string
		if p1.UserID() == cmd.UserID {
			opponentID = p2.UserID()
		} else {
			opponentID = p1.UserID()
		}

		var botUserID string
		if h.botRepo != nil && opponentID != "" {
			isBot, bErr := h.botRepo.IsBot(txCtx, opponentID)
			if bErr != nil {
				return fmt.Errorf("check opponent is bot: %w", bErr)
			}
			if isBot {
				botUserID = opponentID
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

		forfeitedEv := &event.SoloMatchForfeited{
			MatchID:       match.ID(),
			ForfeitedByID: cmd.UserID,
			WinnerID:      winnerID,
			BotUserID:     botUserID,
			Reason:        cmd.Reason,
			ForfeitedAt:   endedAt,
		}

		completedEv := &event.SoloMatchCompleted{
			MatchID:       match.ID(),
			ExerciseID:    match.ExerciseID(),
			ChallengeType: match.ChallengeType().String(),
			WinnerID:      winnerID,
			IsDraw:        false,
			Player1ID:     p1.UserID(),
			Player1Score:  p1.Score().Value(),
			Player2ID:     p2.UserID(),
			Player2Score:  p2.Score().Value(),
			BotUserID:     botUserID,
			CompletedAt:   endedAt,
		}

		if err := h.eventPub.PublishEvents(txCtx, []any{forfeitedEv, completedEv}); err != nil {
			return fmt.Errorf("publish outbox events: %w", err)
		}

		return nil
	})
}
