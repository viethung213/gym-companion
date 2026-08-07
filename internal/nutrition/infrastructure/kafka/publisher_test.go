package kafka_test

import (
	"context"
	"strings"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	nutritionKafka "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/kafka"
)

func TestPublisher_PublishBatch_Empty(t *testing.T) {
	t.Parallel()

	writer := &kafka.Writer{}
	pub := nutritionKafka.NewPublisher(writer)

	err := pub.PublishBatch(context.Background(), nil)
	if err != nil {
		t.Errorf("got err = %v, want nil for empty batch", err)
	}

	err = pub.PublishBatch(context.Background(), []port.OutboxRecord{})
	if err != nil {
		t.Errorf("got err = %v, want nil for empty slice", err)
	}

	err = pub.Close()
	if err != nil {
		t.Errorf("got err = %v, want nil for Close()", err)
	}
}

func TestPublisher_PublishBatch_WithPopulatedWriterTopic(t *testing.T) {
	t.Parallel()

	writer := &kafka.Writer{
		Addr:  kafka.TCP("127.0.0.1:9092"),
		Topic: "nutrition.events",
	}
	pub := nutritionKafka.NewPublisher(writer)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	records := []port.OutboxRecord{
		{
			ID:           "rec-1",
			EventType:    "contracts.nutrition.v1.MealLogged",
			PartitionKey: "nutr-100",
			Payload:      []byte(`{"mealId":"meal-1"}`),
		},
	}

	err := pub.PublishBatch(ctx, records)
	if err == nil {
		t.Fatal("PublishBatch() got nil error, want error on canceled context")
	}
	if strings.Contains(err.Error(), "Topic must not be specified for both Writer and Message") {
		t.Errorf("got topic conflict error = %v, want no topic conflict when writer.Topic is set", err)
	}
}
