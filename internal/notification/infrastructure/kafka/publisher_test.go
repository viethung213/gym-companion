package kafka_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/kafka"
)

func TestPublisherNilWriter(t *testing.T) {
	t.Parallel()

	pub := kafka.NewPublisher(nil)
	if pub == nil {
		t.Fatal("expected non-nil publisher")
	}

	ctx := context.Background()
	err := pub.PublishBatch(ctx, []*port.OutboxRecord{
		{ID: "1", PartitionKey: "k1", Payload: []byte("v1")},
	})
	if err != nil {
		t.Errorf("expected nil error for nil writer, got %v", err)
	}

	_ = pub.Close()
}
