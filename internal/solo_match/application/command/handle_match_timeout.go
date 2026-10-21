package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/event"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

type HandleMatchTimeoutCommand struct {
	Now   time.Time
	Limit int
}

type HandleMatchTimeoutHandler struct {
	matchRepo repository.SoloMatchRepository
	eventPub  port.EventPublisher
	botRepo   repository.BotRepository
	txManager port.TransactionManager
}

func NewHandleMatchTimeoutHandler(
	matchRepo repository.SoloMatchRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
	botRepo repository.BotRepository,
) *HandleMatchTimeoutHandler {
	return &HandleMatchTimeoutHandler{
		matchRepo: matchRepo,
		eventPub:  eventPub,
		botRepo:   botRepo,
		txManager: txManager,
	}
}

func (h *HandleMatchTimeoutHandler) Handle(ctx context.Context, cmd HandleMatchTimeoutCommand) (int, error) {
	now := cmd.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	limit := cmd.Limit
	if limit <= 0 {
		limit = 50
	}

	expiredMatches, err := h.matchRepo.FindExpiredMatches(ctx, now, limit)
	if err != nil {
		return 0, fmt.Errorf("find expired matches: %w", err)
	}

	processedCount := 0
	for _, match := range expiredMatches {
		if tErr := match.Timeout(now); tErr != nil {
			continue
		}

		txErr := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			if sErr := h.matchRepo.Save(txCtx, match); sErr != nil {
				return sErr
			}

			if match.Status() == valueobject.MatchStatusForfeited || match.Status() == valueobject.MatchStatusCompleted {
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
					IsDraw:        false,
					Player1ID:     p1.UserID(),
					Player1Score:  p1.Score().Value(),
					Player2ID:     p2.UserID(),
					Player2Score:  p2.Score().Value(),
					BotUserID:     botUserID,
					CompletedAt:   endedAt,
				}

				if pErr := h.eventPub.PublishEvents(txCtx, []any{ev}); pErr != nil {
					return pErr
				}
			}

			return nil
		})
		if txErr == nil {
			processedCount++
		}
	}

	return processedCount, nil
}
