package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/event"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

// ReactionDebounceWorker processes matured reaction flush tasks and synchronizes them to DB.
type ReactionDebounceWorker struct {
	buffer           port.ReactionDebounceBuffer
	interactionRepo  repository.InteractionRepository
	feedItemRepo     repository.FeedItemRepository
	userSnapshotRepo repository.UserSnapshotRepository
	eventRecorder    port.EventRecorder
	txManager        port.TransactionManager
	interval         time.Duration
}

// NewReactionDebounceWorker creates a new ReactionDebounceWorker instance.
func NewReactionDebounceWorker(
	buffer port.ReactionDebounceBuffer,
	interactionRepo repository.InteractionRepository,
	feedItemRepo repository.FeedItemRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
	eventRecorder port.EventRecorder,
	txManager port.TransactionManager,
	interval time.Duration,
) *ReactionDebounceWorker {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	return &ReactionDebounceWorker{
		buffer:           buffer,
		interactionRepo:  interactionRepo,
		feedItemRepo:     feedItemRepo,
		userSnapshotRepo: userSnapshotRepo,
		eventRecorder:    eventRecorder,
		txManager:        txManager,
		interval:         interval,
	}
}

// Start begins periodic processing of matured reaction tasks until ctx is cancelled.
func (w *ReactionDebounceWorker) Start(ctx context.Context) {
	log.Printf("Starting Social Reaction Debounce worker (interval: %v)...", w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Social Reaction Debounce worker due to context cancellation.")
			return
		case <-ticker.C:
			if err := w.ProcessBatch(ctx); err != nil {
				log.Printf("Social reaction debounce worker batch processing error: %v", err)
			}
		}
	}
}

// ProcessBatch polls matured tasks and applies state changes transactionally.
func (w *ReactionDebounceWorker) ProcessBatch(ctx context.Context) error {
	tasks, err := w.buffer.PopMaturedReactions(ctx, time.Now().UTC(), 50)
	if err != nil {
		return fmt.Errorf("pop matured reactions: %w", err)
	}

	for _, task := range tasks {
		tErr := w.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			existing, getErr := w.interactionRepo.GetReaction(txCtx, task.UserID, task.FeedItemID)
			if getErr != nil {
				return fmt.Errorf("get existing reaction: %w", getErr)
			}

			var currentReaction string
			if existing != nil {
				currentReaction = string(existing.ReactionType())
			}

			// If current DB state already matches desired state, no-op and ack.
			if currentReaction == task.ReactionType {
				return w.buffer.AckReaction(txCtx, task.UserID, task.FeedItemID)
			}

			feedItem, feedErr := w.feedItemRepo.GetByID(txCtx, task.FeedItemID)
			if feedErr != nil {
				return fmt.Errorf("get target feed item: %w", feedErr)
			}
			if feedItem == nil {
				// Target feed item no longer exists, discard task
				return w.buffer.AckReaction(txCtx, task.UserID, task.FeedItemID)
			}

			if task.ReactionType == "" {
				// Toggle OFF: delete existing reaction and decrement count
				if existing != nil {
					if delErr := w.interactionRepo.DeleteReaction(txCtx, task.UserID, task.FeedItemID); delErr != nil {
						return fmt.Errorf("delete reaction: %w", delErr)
					}
					if decrErr := w.feedItemRepo.UpdateReactionCount(txCtx, task.FeedItemID, -1); decrErr != nil {
						return fmt.Errorf("decrement counter: %w", decrErr)
					}
				}
			} else {
				rt, rtErr := vo.NewReactionType(task.ReactionType)
				if rtErr != nil {
					return w.buffer.AckReaction(txCtx, task.UserID, task.FeedItemID)
				}

				if existing != nil {
					existing.ChangeReactionType(rt)
					if saveErr := w.interactionRepo.SaveReaction(txCtx, existing); saveErr != nil {
						return fmt.Errorf("update reaction type: %w", saveErr)
					}
				} else {
					newReaction, newErr := entity.NewReaction(
						uuid.New().String(),
						task.UserID,
						task.FeedItemID,
						rt,
						time.Now().UTC(),
					)
					if newErr != nil {
						return fmt.Errorf("create reaction entity: %w", newErr)
					}
					if saveErr := w.interactionRepo.SaveReaction(txCtx, newReaction); saveErr != nil {
						return fmt.Errorf("save reaction: %w", saveErr)
					}
					if incrErr := w.feedItemRepo.UpdateReactionCount(txCtx, task.FeedItemID, 1); incrErr != nil {
						return fmt.Errorf("increment counter: %w", incrErr)
					}

					if w.eventRecorder != nil {
						var actorName, actorAvatarURL string
						if w.userSnapshotRepo != nil {
							if u, uErr := w.userSnapshotRepo.GetByID(txCtx, task.UserID); uErr == nil && u != nil {
								actorName = u.FullName()
								actorAvatarURL = u.AvatarURL()
							}
						}

						ev := &event.PostReactedEvent{
							FeedItemID:    task.FeedItemID,
							PostOwnerID:   feedItem.UserID(),
							UserID:        task.UserID,
							UserName:      actorName,
							UserAvatarURL: actorAvatarURL,
							ReactionType:  rt,
							ReactedAt:     time.Now().UTC(),
						}
						if recErr := w.eventRecorder.RecordEvents(txCtx, []any{ev}); recErr != nil {
							return fmt.Errorf("record post reacted event: %w", recErr)
						}
					}
				}
			}

			return w.buffer.AckReaction(txCtx, task.UserID, task.FeedItemID)
		})
		if tErr != nil {
			log.Printf("ReactionDebounceWorker task error (user: %s, item: %s): %v", task.UserID, task.FeedItemID, tErr)
		}
	}

	return nil
}
