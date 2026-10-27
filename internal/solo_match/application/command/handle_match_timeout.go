package command

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/event"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

type HandleMatchTimeoutCommand struct {
	Now                 time.Time
	Limit               int
	InProgressThreshold time.Duration
}

type HandleMatchTimeoutHandler struct {
	matchRepo repository.SoloMatchRepository
	eventPub  port.EventPublisher
	txManager port.TransactionManager
	botRepo   repository.BotRepository
}

func NewHandleMatchTimeoutHandler(
	matchRepo repository.SoloMatchRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
	botRepo ...repository.BotRepository,
) *HandleMatchTimeoutHandler {
	var br repository.BotRepository
	if len(botRepo) > 0 {
		br = botRepo[0]
	}
	return &HandleMatchTimeoutHandler{
		matchRepo: matchRepo,
		eventPub:  eventPub,
		txManager: txManager,
		botRepo:   br,
	}
}

func (h *HandleMatchTimeoutHandler) Handle(ctx context.Context, cmd HandleMatchTimeoutCommand) error {
	now := cmd.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	limit := cmd.Limit
	if limit <= 0 {
		limit = 50
	}

	// 1. Quét và hủy các trận PENDING_READY quá hạn ready_deadline (60s check-in window)
	unreadyMatches, uErr := h.matchRepo.FindUnreadyExpiredMatches(ctx, now, limit)
	if uErr != nil {
		return fmt.Errorf("find unready expired matches: %w", uErr)
	}

	for _, match := range unreadyMatches {
		if cErr := match.CancelOnNoShow(now); cErr != nil {
			continue
		}

		if txErr := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			if sErr := h.matchRepo.Save(txCtx, match); sErr != nil {
				return sErr
			}

			endedAt := now.UTC()
			if match.EndedAt() != nil {
				endedAt = *match.EndedAt()
			}

			cancelledEv := &event.SoloMatchCancelled{
				MatchID:     match.ID(),
				Reason:      "READY_TIMEOUT",
				CancelledAt: endedAt,
			}

			return h.eventPub.PublishEvents(txCtx, []any{cancelledEv})
		}); txErr != nil {
			log.Printf("[SoloMatch Timeout] Failed to cancel expired match %s: %v", match.ID(), txErr)
		}
	}

	// 2. Quét và xử lý các trận IN_PROGRESS quá hạn thời gian tối đa (mặc định 35 phút)
	inProgressThreshold := cmd.InProgressThreshold
	if inProgressThreshold <= 0 {
		inProgressThreshold = 35 * time.Minute
	}
	startedBefore := now.Add(-inProgressThreshold)

	expiredMatches, eErr := h.matchRepo.FindExpiredInProgressMatches(ctx, startedBefore, limit)
	if eErr != nil {
		log.Printf("[SoloMatch Timeout] Find expired in-progress matches error: %v", eErr)
	} else {
		for _, match := range expiredMatches {
			if tErr := match.TimeoutInProgress(now); tErr != nil {
				continue
			}

			if txErr := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
				if sErr := h.matchRepo.Save(txCtx, match); sErr != nil {
					return sErr
				}

				endedAt := now.UTC()
				if match.EndedAt() != nil {
					endedAt = *match.EndedAt()
				}

				var eventsToPublish []any
				if match.Status() == valueobject.MatchStatusForfeited {
					winnerID := ""
					if match.WinnerID() != nil {
						winnerID = *match.WinnerID()
					}

					var botUserID string
					if h.botRepo != nil && winnerID != "" {
						p1 := match.Participants()[0]
						p2 := match.Participants()[1]
						otherID := p1.UserID()
						if otherID == winnerID {
							otherID = p2.UserID()
						}
						isBot, _ := h.botRepo.IsBot(txCtx, otherID)
						if isBot {
							botUserID = otherID
						}
					}

					forfeitedByID := ""
					for _, p := range match.Participants() {
						if p.UserID() != winnerID {
							forfeitedByID = p.UserID()
							break
						}
					}

					forfeitedEv := &event.SoloMatchForfeited{
						MatchID:       match.ID(),
						ForfeitedByID: forfeitedByID,
						WinnerID:      winnerID,
						BotUserID:     botUserID,
						Reason:        "IN_PROGRESS_TIMEOUT",
						ForfeitedAt:   endedAt,
					}

					p1 := match.Participants()[0]
					p2 := match.Participants()[1]
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
					eventsToPublish = []any{forfeitedEv, completedEv}
				} else {
					cancelledEv := &event.SoloMatchCancelled{
						MatchID:     match.ID(),
						Reason:      "IN_PROGRESS_TIMEOUT",
						CancelledAt: endedAt,
					}
					eventsToPublish = []any{cancelledEv}
				}

				return h.eventPub.PublishEvents(txCtx, eventsToPublish)
			}); txErr != nil {
				log.Printf("[SoloMatch Timeout] Failed to timeout expired in-progress match %s: %v", match.ID(), txErr)
			}
		}
	}

	return nil
}
