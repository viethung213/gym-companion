package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
	workoutexecutionv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
)

// PersonalRecordConsumer listens to contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved
// and awards +25 PR Bonus XP and unlocks PR_BREAKER badge.
type PersonalRecordConsumer struct {
	reader        *kafka.Reader
	userXpRepo    repository.UserXpRepository
	badgeRepo     repository.BadgeRepository
	txManager     port.TransactionManager
	outboxWriter  port.OutboxWriter
	outboxLogRepo port.OutboxLogRepository
}

// NewPersonalRecordConsumer constructs PersonalRecordConsumer.
func NewPersonalRecordConsumer(
	reader *kafka.Reader,
	userXpRepo repository.UserXpRepository,
	badgeRepo repository.BadgeRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
	outboxLogRepo port.OutboxLogRepository,
) *PersonalRecordConsumer {
	return &PersonalRecordConsumer{
		reader:        reader,
		userXpRepo:    userXpRepo,
		badgeRepo:     badgeRepo,
		txManager:     txManager,
		outboxWriter:  outboxWriter,
		outboxLogRepo: outboxLogRepo,
	}
}

// Start polls Kafka for PR events until context cancellation.
func (c *PersonalRecordConsumer) Start(ctx context.Context) {
	if c.reader == nil {
		return
	}
	log.Println("[Gamification] Starting PersonalRecord Kafka Consumer...")
	for {
		select {
		case <-ctx.Done():
			log.Println("[Gamification] Stopping PersonalRecord Kafka Consumer due to context cancellation.")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[Gamification] PersonalRecord consumer fetch error: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			var processErr error
			for attempt := 0; attempt < 3; attempt++ {
				processErr = c.ProcessMessage(ctx, &msg)
				if processErr == nil {
					break
				}
				if ctx.Err() != nil {
					return
				}
				time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)
			}

			if processErr != nil {
				log.Printf("[Gamification] PersonalRecord consumer process error after retries: %v", processErr)
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[Gamification] PersonalRecord consumer commit error: %v", err)
			}
		}
	}
}

// ProcessMessage handles the PR achieved event.
func (c *PersonalRecordConsumer) ProcessMessage(ctx context.Context, msg *kafka.Message) error {
	if msg == nil {
		return errors.New("kafka message is nil")
	}
	var ce CloudEvent
	if err := json.Unmarshal(msg.Value, &ce); err != nil {
		return fmt.Errorf("unmarshal cloudevent envelope: %w", err)
	}

	if !strings.Contains(ce.Type, "newPersonalRecordAchieved") &&
		!strings.Contains(ce.Type, "NewPersonalRecordAchieved") {
		return nil
	}

	// Idempotency check via outbox_log
	if c.outboxLogRepo != nil && ce.ID != "" {
		processed, pErr := c.outboxLogRepo.IsProcessed(ctx, ce.ID)
		if pErr == nil && processed {
			return nil
		}
	}

	var protoEv workoutexecutionv1event.NewPersonalRecordAchieved
	if err := protojson.Unmarshal(ce.Data, &protoEv); err != nil {
		return fmt.Errorf("unmarshal proto NewPersonalRecordAchieved: %w", err)
	}

	userID := protoEv.GetUserId()
	achievedAt := time.Now().UTC()
	if protoEv.AchievedAt != nil {
		achievedAt = protoEv.AchievedAt.AsTime()
	}
	activityDate := ResolveLocalDate(achievedAt)

	// Execute inside local transaction: +25 XP & Unlock PR_BREAKER Badge
	err := c.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		userXp, findErr := c.userXpRepo.FindByIDForUpdate(txCtx, userID)
		if findErr != nil {
			return fmt.Errorf("find user_xp for update: %w", findErr)
		}
		if userXp == nil {
			var createErr error
			userXp, createErr = aggregate.NewUserXp(userID)
			if createErr != nil {
				return fmt.Errorf("create user_xp: %w", createErr)
			}
		}

		// Award +25 XP for PR
		xpEvents, earnErr := userXp.EarnXp(25, "PERSONAL_RECORD", achievedAt)
		if earnErr != nil {
			return fmt.Errorf("earn pr xp: %w", earnErr)
		}

		if saveXpErr := c.userXpRepo.Save(txCtx, userXp); saveXpErr != nil {
			return fmt.Errorf("save user_xp: %w", saveXpErr)
		}

		history := entity.NewXpHistoryEntry(
			uuid.NewString(),
			userID,
			25,
			"PERSONAL_RECORD",
			nil,
			activityDate,
			achievedAt,
		)
		if saveHistErr := c.userXpRepo.SaveHistory(txCtx, history); saveHistErr != nil {
			return fmt.Errorf("save pr xp_history: %w", saveHistErr)
		}

		outboxEvents := make([]any, len(xpEvents))
		for i, ev := range xpEvents {
			outboxEvents[i] = ev
		}

		if err := c.outboxWriter.WriteEvents(txCtx, "user_xp", userID, outboxEvents); err != nil {
			return err
		}

		// Check and unlock PR_BREAKER badge
		has, hasBadgeErr := c.badgeRepo.HasBadge(txCtx, userID, "PR_BREAKER")
		if hasBadgeErr == nil && !has {
			def, _ := vo.GetBadgeDefinition("PR_BREAKER")
			meta := map[string]string{
				"exerciseId": protoEv.GetExerciseId(),
				"weight":     fmt.Sprintf("%.1f", protoEv.GetWeight()),
				"reps":       strconv.Itoa(int(protoEv.GetReps())),
			}
			badge, badgeEv, bErr := aggregate.UnlockBadge(
				uuid.NewString(),
				userID,
				"PR_BREAKER",
				def.Name,
				def.Category,
				meta,
				achievedAt,
			)
			if bErr == nil {
				if saveBadgeErr := c.badgeRepo.Save(txCtx, badge); saveBadgeErr == nil {
					if err := c.outboxWriter.WriteEvents(txCtx, "user_badges", userID, []any{badgeEv}); err != nil {
						return err
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		if c.outboxLogRepo != nil && ce.ID != "" {
			_ = c.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
				EventID:      ce.ID,
				EventType:    ce.Type,
				Payload:      msg.Value,
				PartitionKey: string(msg.Key),
				Status:       "FAILED",
				ErrorMessage: err.Error(),
			})
		}
		return err
	}

	if c.outboxLogRepo != nil && ce.ID != "" {
		_ = c.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
			EventID:      ce.ID,
			EventType:    ce.Type,
			Payload:      msg.Value,
			PartitionKey: string(msg.Key),
			Status:       "PROCESSED",
		})
	}

	return nil
}
