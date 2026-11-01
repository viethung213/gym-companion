package consumer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	appService "github.com/viethung213/gym-companion/internal/gamification/application/service"
	workoutexecutionv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
)

type WorkoutEventConsumer struct {
	reader        *kafka.Reader
	coordinator   *appService.ActivityIngestionCoordinator
	outboxLogRepo port.OutboxLogRepository
}

func NewWorkoutEventConsumer(
	reader *kafka.Reader,
	coordinator *appService.ActivityIngestionCoordinator,
	outboxLogRepo port.OutboxLogRepository,
) *WorkoutEventConsumer {
	return &WorkoutEventConsumer{
		reader:        reader,
		coordinator:   coordinator,
		outboxLogRepo: outboxLogRepo,
	}
}

func (c *WorkoutEventConsumer) Start(ctx context.Context) {
	runConsumerLoop(ctx, "WorkoutEvent", c.reader, c.ProcessMessage)
}

func (c *WorkoutEventConsumer) ProcessMessage(ctx context.Context, msg *kafka.Message) error {
	return processIdempotentCloudEvent(
		ctx,
		msg,
		c.outboxLogRepo,
		func(eventType string) bool {
			return strings.Contains(eventType, "workoutSessionCompleted") ||
				strings.Contains(eventType, "WorkoutSessionCompleted")
		},
		func(ctx context.Context, ce *CloudEvent, _ *kafka.Message) error {
			var protoEv workoutexecutionv1event.WorkoutSessionCompleted
			if err := protojson.Unmarshal(ce.Data, &protoEv); err != nil {
				return fmt.Errorf("unmarshal proto WorkoutSessionCompleted: %w", err)
			}

			completedAt := time.Now().UTC()
			if protoEv.CompletedAt != nil {
				completedAt = protoEv.CompletedAt.AsTime()
			}

			return c.coordinator.IngestWorkoutSession(ctx, &command.ProcessWorkoutXpCommand{
				UserID:           protoEv.GetUserId(),
				SessionID:        protoEv.GetSessionId(),
				ActivityDate:     completedAt.UTC().Format("2006-01-02"),
				TotalSets:        protoEv.GetTotalSets(),
				TotalVolumeKg:    float64(protoEv.GetTotalVolume()),
				AverageFormScore: protoEv.AverageFormScore,
				HasNewPR:         false,
				CompletedAt:      completedAt,
			})
		},
	)
}
