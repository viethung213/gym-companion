package kafka_test

import (
	"context"
	"strings"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	workoutKafka "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/kafka"
)

func TestPublisher_PublishBatch_Empty(t *testing.T) {
	writer := &kafka.Writer{}
	pub := workoutKafka.NewPublisher(writer)

	err := pub.PublishBatch(context.Background(), nil)
	if err != nil {
		t.Errorf("got err = %v, want nil for empty batch", err)
	}

	err = pub.PublishBatch(context.Background(), []*port.OutboxRecord{})
	if err != nil {
		t.Errorf("got err = %v, want nil for empty slice", err)
	}

	err = pub.Close()
	if err != nil {
		t.Errorf("got err = %v, want nil for Close()", err)
	}
}

func TestPublisher_PublishBatch_ErrorHandling(t *testing.T) {
	writer := &kafka.Writer{
		Addr: kafka.TCP("127.0.0.1:9092"),
	}
	pub := workoutKafka.NewPublisher(writer)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel context immediately to force WriteMessages failure

	records := []*port.OutboxRecord{
		{
			ID:           "rec-1",
			EventType:    "contracts.workout_execution.v1.WorkoutSessionStarted",
			PartitionKey: "user-100",
			Payload:      []byte(`{"sessionId":"sess-1"}`),
		},
	}

	err := pub.PublishBatch(ctx, records)
	if err == nil {
		t.Fatal("PublishBatch() got nil error, want error on canceled context")
	}
	if !strings.Contains(err.Error(), "write workout_execution kafka batch messages") {
		t.Errorf("got err = %v, want error wrapped with 'write workout_execution kafka batch messages'", err)
	}
}
