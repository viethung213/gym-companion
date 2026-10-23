package consumer

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	authv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
)

type UserRegisteredConsumer struct {
	reader        *kafka.Reader
	initHandler   *command.InitUserGamificationHandler
	outboxLogRepo port.OutboxLogRepository
}

func NewUserRegisteredConsumer(
	reader *kafka.Reader,
	initHandler *command.InitUserGamificationHandler,
	outboxLogRepo port.OutboxLogRepository,
) *UserRegisteredConsumer {
	return &UserRegisteredConsumer{
		reader:        reader,
		initHandler:   initHandler,
		outboxLogRepo: outboxLogRepo,
	}
}

func (c *UserRegisteredConsumer) Start(ctx context.Context) {
	runConsumerLoop(ctx, "UserRegistered", c.reader, c.ProcessMessage)
}

func (c *UserRegisteredConsumer) ProcessMessage(ctx context.Context, msg *kafka.Message) error {
	return processIdempotentCloudEvent(
		ctx,
		msg,
		c.outboxLogRepo,
		func(eventType string) bool {
			return eventType == "contracts.generic.auth.v1.userRegistered"
		},
		func(ctx context.Context, ce *CloudEvent, _ *kafka.Message) error {
			var protoEv authv1event.UserRegistered
			if err := protojson.Unmarshal(ce.Data, &protoEv); err != nil {
				return fmt.Errorf("unmarshal proto UserRegistered: %w", err)
			}

			return c.initHandler.Handle(ctx, command.InitUserGamificationCommand{
				UserID:      protoEv.GetUserId(),
				DisplayName: protoEv.GetFullName(),
				AvatarURL:   protoEv.GetAvatarUrl(),
			})
		},
	)
}
