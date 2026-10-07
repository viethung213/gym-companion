package kafka_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/infrastructure/kafka"
)

func TestPublisher_EmptyBatch(t *testing.T) {
	pub := kafka.NewPublisher(nil)
	err := pub.PublishBatch(context.Background(), []*port.OutboxRecord{})
	if err != nil {
		t.Fatalf("expected nil error on empty batch, got %v", err)
	}
}

func TestPublisher_RecordsWithNilWriters(t *testing.T) {
	pub := kafka.NewPublisher(nil, nil)
	records := []*port.OutboxRecord{
		{
			EventType:    "contracts.supporting.social.v1.postCreated",
			PartitionKey: "u-1",
			Payload:      []byte(`{}`),
		},
		{
			EventType:    "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested",
			PartitionKey: "u-2",
			Payload:      []byte(`{}`),
		},
	}
	err := pub.PublishBatch(context.Background(), records)
	if err != nil {
		t.Fatalf("expected nil error when writers are nil, got %v", err)
	}
}
