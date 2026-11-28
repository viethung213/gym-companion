package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
	"github.com/viethung213/gym-companion/internal/gamification/transport/consumer"
	gamificationv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type recordingScoreProjector struct {
	cmds []command.RefreshLeaderboardScoreCommand
}

func (r *recordingScoreProjector) Handle(_ context.Context, cmd command.RefreshLeaderboardScoreCommand) error {
	r.cmds = append(r.cmds, cmd)
	return nil
}

func cloudEventMessage(t *testing.T, eventType string, data []byte) *kafka.Message {
	t.Helper()
	envelope, err := json.Marshal(map[string]any{
		"specversion":     "1.0",
		"id":              "evt-1",
		"source":          "services/gamification-service",
		"type":            eventType,
		"datacontenttype": "application/json",
		"data":            json.RawMessage(data),
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return &kafka.Message{Key: []byte("u-1"), Value: envelope}
}

func xpEarnedData(t *testing.T, ev *gamificationv1event.XpEarned) []byte {
	t.Helper()
	data, err := protojson.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal XpEarned: %v", err)
	}
	return data
}

func TestLeaderboardConsumer_RefreshesUserOnXpEarned(t *testing.T) {
	projector := &recordingScoreProjector{}
	c := consumer.NewLeaderboardXpEarnedConsumer(nil, projector)
	earnedAt := time.Date(2026, 11, 27, 6, 15, 0, 0, time.UTC)
	data := xpEarnedData(t, &gamificationv1event.XpEarned{
		UserId: "u-1", DeltaXp: 40, NewXp: 1240, NewLevel: 5, Source: "WORKOUT", EarnedAt: timestamppb.New(earnedAt),
	})

	if err := c.ProcessMessage(context.Background(), cloudEventMessage(t, event.XpEarned{}.EventName(), data)); err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	if len(projector.cmds) != 1 {
		t.Fatalf("projector calls = %d, want 1", len(projector.cmds))
	}
	if got := projector.cmds[0]; got.UserID != "u-1" {
		t.Errorf("command = %+v, want refresh of u-1", got)
	}
}

func TestLeaderboardConsumer_IgnoresOtherTypes(t *testing.T) {
	projector := &recordingScoreProjector{}
	c := consumer.NewLeaderboardXpEarnedConsumer(nil, projector)

	msg := cloudEventMessage(t, event.UserLeveledUp{}.EventName(), []byte(`{"userId":"u-1"}`))
	if err := c.ProcessMessage(context.Background(), msg); err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	if len(projector.cmds) != 0 {
		t.Fatalf("projector called for a non-XpEarned event: %+v", projector.cmds)
	}
}

func TestLeaderboardConsumer_MissingUserID_Error(t *testing.T) {
	projector := &recordingScoreProjector{}
	c := consumer.NewLeaderboardXpEarnedConsumer(nil, projector)
	data := xpEarnedData(t, &gamificationv1event.XpEarned{NewXp: 10})

	if err := c.ProcessMessage(context.Background(), cloudEventMessage(t, event.XpEarned{}.EventName(), data)); err == nil {
		t.Fatal("ProcessMessage err = nil, want error for missing user_id")
	}
	if len(projector.cmds) != 0 {
		t.Fatal("projector called despite invalid payload")
	}
}

func TestLeaderboardConsumer_InvalidJSON_Error(t *testing.T) {
	c := consumer.NewLeaderboardXpEarnedConsumer(nil, &recordingScoreProjector{})

	if err := c.ProcessMessage(context.Background(), &kafka.Message{Value: []byte("{not json")}); err == nil {
		t.Fatal("ProcessMessage err = nil, want unmarshal error")
	}
	bad := cloudEventMessage(t, event.XpEarned{}.EventName(), []byte(`{"newXp":"oops"}`))
	if err := c.ProcessMessage(context.Background(), bad); err == nil {
		t.Fatal("ProcessMessage err = nil, want payload unmarshal error")
	}
}
