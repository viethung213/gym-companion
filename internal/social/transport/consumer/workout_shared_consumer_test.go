package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	workoutexecutionv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/event"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	socialConsumer "github.com/viethung213/gym-companion/internal/social/transport/consumer"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockFeedRepoForConsumer struct {
	items map[string]*aggregate.FeedItem
}

func (m *mockFeedRepoForConsumer) Create(ctx context.Context, item *aggregate.FeedItem) error {
	m.items[item.ID()] = item
	return nil
}

func (m *mockFeedRepoForConsumer) GetByID(ctx context.Context, id string) (*aggregate.FeedItem, error) {
	return m.items[id], nil
}

func (m *mockFeedRepoForConsumer) Delete(ctx context.Context, id, userID string) error {
	delete(m.items, id)
	return nil
}

func (m *mockFeedRepoForConsumer) GetByUserID(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	return nil, "", nil
}

func (m *mockFeedRepoForConsumer) GetByAuthorIDs(ctx context.Context, authorIDs []string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	return nil, "", nil
}

func (m *mockFeedRepoForConsumer) CountByUserID(ctx context.Context, userID string) (int32, error) {
	return int32(len(m.items)), nil
}

func (m *mockFeedRepoForConsumer) UpdateReactionCount(ctx context.Context, id string, delta int) error {
	return nil
}

func (m *mockFeedRepoForConsumer) UpdateCommentCount(ctx context.Context, id string, delta int) error {
	return nil
}

type mockTxForWorkoutConsumer struct{}

func (m *mockTxForWorkoutConsumer) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestWorkoutSharedConsumer_ProcessMessage(t *testing.T) {
	feedRepo := &mockFeedRepoForConsumer{items: make(map[string]*aggregate.FeedItem)}
	tx := &mockTxForWorkoutConsumer{}
	ingestHandler := command.NewIngestWorkoutActivityHandler(feedRepo, tx)
	consumer := socialConsumer.NewWorkoutSharedConsumer(nil, ingestHandler, nil, tx)

	data := &workoutexecutionv1event.WorkoutSessionShared{
		SessionId:       "sess-123",
		UserId:          "u-athlete-1",
		Title:           "Morning Leg Blast",
		Caption:         "Felt awesome today!",
		MediaUrls:       []string{"https://media.com/workout.jpg"},
		Visibility:      "PUBLIC",
		DurationSeconds: 3600,
		TotalSets:       18,
		TotalVolumeKg:   5200.5,
		PrCount:         2,
		SharedAt:        timestamppb.New(time.Now().UTC()),
	}
	dataBytes, _ := protojson.Marshal(data)

	cloudEvent := map[string]any{
		"specversion":     "1.0",
		"id":              "evt-shared-1",
		"source":          "services/workout-service",
		"type":            "contracts.core.workout_execution.v1.workoutSessionShared",
		"datacontenttype": "application/json",
		"data":            json.RawMessage(dataBytes),
	}
	envelopeBytes, _ := json.Marshal(cloudEvent)

	msg := kafka.Message{
		Key:   []byte("u-athlete-1"),
		Value: envelopeBytes,
	}

	err := consumer.ProcessMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(feedRepo.items) != 1 {
		t.Fatalf("expected 1 feed item ingested, got %d", len(feedRepo.items))
	}

	for _, item := range feedRepo.items {
		if item.UserID() != "u-athlete-1" {
			t.Errorf("expected user id u-athlete-1, got %s", item.UserID())
		}
		if item.Caption() != "Felt awesome today!" {
			t.Errorf("expected caption, got %s", item.Caption())
		}
		if item.WorkoutData().WorkoutTitle() != "Morning Leg Blast" {
			t.Errorf("expected workout title, got %s", item.WorkoutData().WorkoutTitle())
		}
	}
}

func TestWorkoutSharedConsumer_StartCancellation(t *testing.T) {
	consumer := socialConsumer.NewWorkoutSharedConsumer(nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		consumer.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("consumer did not stop upon cancelled context")
	}
}

func TestWorkoutSharedConsumer_EdgeCases(t *testing.T) {
	consumer := socialConsumer.NewWorkoutSharedConsumer(nil, nil, nil, nil)

	// Invalid JSON
	err := consumer.ProcessMessage(context.Background(), kafka.Message{
		Value: []byte("{not-json"),
	})
	if err == nil {
		t.Errorf("expected error for invalid json, got nil")
	}

	// Ignored Event Type
	b, _ := json.Marshal(map[string]any{"type": "other.event"})
	err = consumer.ProcessMessage(context.Background(), kafka.Message{Value: b})
	if err != nil {
		t.Errorf("expected nil for ignored event type, got %v", err)
	}
}
