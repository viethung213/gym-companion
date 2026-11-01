package consumer

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	profilev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
)

type UserIdentityUpdatedConsumer struct {
	reader        *kafka.Reader
	syncHandler   *command.SyncUserSnapshotHandler
	outboxLogRepo port.OutboxLogRepository
}

func NewUserIdentityUpdatedConsumer(
	reader *kafka.Reader,
	syncHandler *command.SyncUserSnapshotHandler,
	outboxLogRepo port.OutboxLogRepository,
) *UserIdentityUpdatedConsumer {
	return &UserIdentityUpdatedConsumer{
		reader:        reader,
		syncHandler:   syncHandler,
		outboxLogRepo: outboxLogRepo,
	}
}

func (c *UserIdentityUpdatedConsumer) Start(ctx context.Context) {
	runConsumerLoop(ctx, "UserIdentityUpdated", c.reader, c.ProcessMessage)
}

func (c *UserIdentityUpdatedConsumer) ProcessMessage(ctx context.Context, msg *kafka.Message) error {
	return processIdempotentCloudEvent(
		ctx,
		msg,
		c.outboxLogRepo,
		func(eventType string) bool {
			return eventType == "contracts.supporting.profile.v1.event.UserIdentityUpdated"
		},
		func(ctx context.Context, ce *CloudEvent, _ *kafka.Message) error {
			var protoEv profilev1event.UserIdentityUpdated
			if err := protojson.Unmarshal(ce.Data, &protoEv); err != nil {
				return fmt.Errorf("unmarshal proto UserIdentityUpdated: %w", err)
			}

			return c.syncHandler.Handle(ctx, command.SyncUserSnapshotCommand{
				UserID:      protoEv.GetUserId(),
				DisplayName: protoEv.GetFullName(),
				AvatarURL:   protoEv.GetAvatarUrl(),
			})
		},
	)
}
