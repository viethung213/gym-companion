package kafka_test

import (
	"context"
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
