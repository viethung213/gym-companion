package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
	gamificationv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/event"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"google.golang.org/protobuf/encoding/protojson"
)

var errXpEarnedMissingUserID = errors.New("xpEarned event is missing user_id")

// LeaderboardScoreRefresher re-projects a user's score from the source of truth.
type LeaderboardScoreRefresher interface {
	Handle(ctx context.Context, cmd command.RefreshLeaderboardScoreCommand) error
}

// LeaderboardXpEarnedConsumer uses the module's own XpEarned events as change signals for the
// leaderboard read model.
type LeaderboardXpEarnedConsumer struct {
	reader    *kafka.Reader
	refresher LeaderboardScoreRefresher
}

// NewLeaderboardXpEarnedConsumer constructs LeaderboardXpEarnedConsumer.
func NewLeaderboardXpEarnedConsumer(
	reader *kafka.Reader,
	refresher LeaderboardScoreRefresher,
) *LeaderboardXpEarnedConsumer {
	return &LeaderboardXpEarnedConsumer{reader: reader, refresher: refresher}
}

// Start consumes until ctx is cancelled.
func (c *LeaderboardXpEarnedConsumer) Start(ctx context.Context) {
	sharedKafka.RunConsumerLoop(ctx, c.reader, leaderboardLoopConfig(), c.ProcessMessage)
}

// leaderboardLoopConfig drops a signal whose refresh keeps failing instead of dead-lettering it:
// the event only says "this user changed", so replaying it later adds nothing, and the periodic
// reconcile re-derives every score from PostgreSQL. Dead-lettering would flood the system DLQ for
// the whole of a Redis outage; leaving it uncommitted would stall the consumer.
func leaderboardLoopConfig() sharedKafka.ConsumerConfig {
	return sharedKafka.ConsumerConfig{
		Name:        "LeaderboardXpEarned",
		MaxRetries:  consumerMaxRetries,
		BaseBackoff: consumerBaseBackoff,
		OnDLQ:       dropLeaderboardSignal,
	}
}

func dropLeaderboardSignal(ctx context.Context, msg *kafka.Message, err error) error {
	slog.WarnContext(ctx, "Leaderboard refresh signal dropped; the periodic reconcile will repair it",
		slog.String("module", "gamification"),
		slog.String("topic", msg.Topic),
		slog.Int64("offset", msg.Offset),
		slog.String("error", err.Error()))
	return nil
}

// ProcessMessage handles one CloudEvent. Refreshing is idempotent, so no outbox-log deduplication
// is needed.
func (c *LeaderboardXpEarnedConsumer) ProcessMessage(
	ctx context.Context,
	msg *kafka.Message,
) error {
	xpEarnedType := event.XpEarned{}.EventName()
	return processIdempotentCloudEvent(
		ctx,
		msg,
		nil,
		func(eventType string) bool { return eventType == xpEarnedType },
		func(ctx context.Context, ce *CloudEvent, _ *kafka.Message) error {
			var protoEv gamificationv1event.XpEarned
			if err := protojson.Unmarshal(ce.Data, &protoEv); err != nil {
				return fmt.Errorf("unmarshal proto XpEarned: %w", err)
			}
			if protoEv.GetUserId() == "" {
				return errXpEarnedMissingUserID
			}
			return c.refresher.Handle(
				ctx,
				command.RefreshLeaderboardScoreCommand{UserID: protoEv.GetUserId()},
			)
		},
	)
}
