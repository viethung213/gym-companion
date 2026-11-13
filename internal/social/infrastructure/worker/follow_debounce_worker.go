package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/event"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

// FollowDebounceWorker processes matured follow/unfollow tasks and synchronizes them to DB.
type FollowDebounceWorker struct {
	buffer           port.FollowDebounceBuffer
	followRepo       repository.FollowRepository
	userSnapshotRepo repository.UserSnapshotRepository
	eventRecorder    port.EventRecorder
	txManager        port.TransactionManager
	interval         time.Duration
}

// NewFollowDebounceWorker creates a new FollowDebounceWorker instance.
func NewFollowDebounceWorker(
	buffer port.FollowDebounceBuffer,
	followRepo repository.FollowRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
	eventRecorder port.EventRecorder,
	txManager port.TransactionManager,
	interval time.Duration,
) *FollowDebounceWorker {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	return &FollowDebounceWorker{
		buffer:           buffer,
		followRepo:       followRepo,
		userSnapshotRepo: userSnapshotRepo,
		eventRecorder:    eventRecorder,
		txManager:        txManager,
		interval:         interval,
	}
}

// Start begins periodic processing of matured follow/unfollow tasks until ctx is cancelled.
func (w *FollowDebounceWorker) Start(ctx context.Context) {
	log.Printf("Starting Social Follow Debounce worker (interval: %v)...", w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Social Follow Debounce worker due to context cancellation.")
			return
		case <-ticker.C:
			if err := w.ProcessBatch(ctx); err != nil {
				log.Printf("Social follow debounce worker batch processing error: %v", err)
			}
		}
	}
}

// ProcessBatch polls matured tasks and applies follow state changes transactionally.
func (w *FollowDebounceWorker) ProcessBatch(ctx context.Context) error {
	tasks, err := w.buffer.PopMaturedFollows(ctx, time.Now().UTC(), 50)
	if err != nil {
		return fmt.Errorf("pop matured follows: %w", err)
	}

	for _, task := range tasks {
		tErr := w.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			isFollowing, checkErr := w.followRepo.IsFollowing(txCtx, task.FollowerID, task.FollowingID)
			if checkErr != nil {
				return fmt.Errorf("check is following: %w", checkErr)
			}

			// If current DB state already matches desired state, no-op and ack.
			if isFollowing == task.IsFollowing {
				return w.buffer.AckFollow(txCtx, task.FollowerID, task.FollowingID)
			}

			if task.IsFollowing {
				// User wants to follow
				follow, createErr := aggregate.NewFollow(
					uuid.New().String(),
					task.FollowerID,
					task.FollowingID,
					time.Now().UTC(),
				)
				if createErr != nil {
					return fmt.Errorf("create follow aggregate: %w", createErr)
				}

				if err := w.followRepo.Follow(txCtx, follow); err != nil {
					return fmt.Errorf("persist follow: %w", err)
				}

				if w.userSnapshotRepo != nil {
					if err := w.userSnapshotRepo.UpdateFollowCounts(txCtx, task.FollowerID, task.FollowingID, 1); err != nil {
						return fmt.Errorf("update follow counts: %w", err)
					}
				}

				if w.eventRecorder != nil {
					var followerName, followerAvatarURL string
					if w.userSnapshotRepo != nil {
						if u, uErr := w.userSnapshotRepo.GetByID(txCtx, task.FollowerID); uErr == nil && u != nil {
							followerName = u.FullName()
							followerAvatarURL = u.AvatarURL()
						}
					}

					ev := &event.UserFollowedEvent{
						FollowerID:        task.FollowerID,
						FollowingID:       task.FollowingID,
						FollowedAt:        follow.CreatedAt(),
						FollowerName:      followerName,
						FollowerAvatarURL: followerAvatarURL,
					}
					if recErr := w.eventRecorder.RecordEvents(txCtx, []any{ev}); recErr != nil {
						return fmt.Errorf("record user followed event: %w", recErr)
					}
				}
			} else {
				// User wants to unfollow
				if err := w.followRepo.Unfollow(txCtx, task.FollowerID, task.FollowingID); err != nil {
					return fmt.Errorf("unfollow: %w", err)
				}

				if w.userSnapshotRepo != nil {
					if err := w.userSnapshotRepo.UpdateFollowCounts(txCtx, task.FollowerID, task.FollowingID, -1); err != nil {
						return fmt.Errorf("update follow counts: %w", err)
					}
				}
			}

			return w.buffer.AckFollow(txCtx, task.FollowerID, task.FollowingID)
		})
		if tErr != nil {
			log.Printf("FollowDebounceWorker task error (%s -> %s): %v", task.FollowerID, task.FollowingID, tErr)
		}
	}

	return nil
}
