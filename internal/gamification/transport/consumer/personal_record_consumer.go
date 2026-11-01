package consumer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	workoutexecutionv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
)

type PersonalRecordHandler interface {
	Handle(ctx context.Context, cmd command.ProcessPersonalRecordCommand) error
}

type PersonalRecordConsumer struct {
	reader        *kafka.Reader
	handler       PersonalRecordHandler
	outboxLogRepo port.OutboxLogRepository
}

func NewPersonalRecordConsumer(
	reader *kafka.Reader,
	handler PersonalRecordHandler,
	outboxLogRepo port.OutboxLogRepository,
) *PersonalRecordConsumer {
	return &PersonalRecordConsumer{
		reader:        reader,
		handler:       handler,
		outboxLogRepo: outboxLogRepo,
	}
}

func (c *PersonalRecordConsumer) Start(ctx context.Context) {
	runConsumerLoop(ctx, "PersonalRecord", c.reader, c.ProcessMessage)
}

func (c *PersonalRecordConsumer) ProcessMessage(ctx context.Context, msg *kafka.Message) error {
	return processIdempotentCloudEvent(
		ctx,
		msg,
		c.outboxLogRepo,
		func(eventType string) bool {
			return strings.Contains(eventType, "newPersonalRecordAchieved") ||
				strings.Contains(eventType, "NewPersonalRecordAchieved")
		},
		func(ctx context.Context, ce *CloudEvent, _ *kafka.Message) error {
			var protoEv workoutexecutionv1event.NewPersonalRecordAchieved
			if err := protojson.Unmarshal(ce.Data, &protoEv); err != nil {
				return fmt.Errorf("unmarshal proto NewPersonalRecordAchieved: %w", err)
			}

			achievedAt := time.Now().UTC()
			if protoEv.AchievedAt != nil {
				achievedAt = protoEv.AchievedAt.AsTime()
			}

			if c.handler == nil {
				return nil
			}
			return c.handler.Handle(ctx, command.ProcessPersonalRecordCommand{
				UserID:       protoEv.GetUserId(),
				ExerciseID:   protoEv.GetExerciseId(),
				Weight:       float64(protoEv.GetWeight()),
				Reps:         protoEv.GetReps(),
				ActivityDate: achievedAt.UTC().Format("2006-01-02"),
				AchievedAt:   achievedAt,
			})
		},
	)
}
