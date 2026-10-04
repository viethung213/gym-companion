package kafka_test

import (
	"context"
	"strings"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	authKafka "github.com/viethung213/gym-companion/internal/auth/infrastructure/kafka"
)

func TestPublisher_PublishBatch_Empty(t *testing.T) {
	writer := &kafka.Writer{}
	pub := authKafka.NewPublisher(writer)

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

func TestPublisher_PublishBatch_WithPopulatedWriterTopic(t *testing.T) {
	writer := &kafka.Writer{
		Addr:  kafka.TCP("127.0.0.1:9092"),
		Topic: "auth.events",
	}
	pub := authKafka.NewPublisher(writer)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel context to prevent network write and test topic validation

	records := []*port.OutboxRecord{
		{
			ID:           "rec-1",
			EventType:    "contracts.generic.auth.v1.userRegistered",
			PartitionKey: "user-100",
			Payload:      []byte(`{"userId":"usr-1"}`),
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

func TestPublisher_HighPriorityEvent_StrictRouting(t *testing.T) {
	defaultWriter := &kafka.Writer{
		Addr:  kafka.TCP("127.0.0.1:9092"),
		Topic: "auth.events",
	}

	// 1. Khi highPriorityWriter là nil: Phải trả về lỗi, không được fallback sang default topic
	pubNoHighWriter := authKafka.NewPublisher(defaultWriter)
	highPriorityRecord := &port.OutboxRecord{
		ID:           "rec-high-1",
		EventType:    "contracts.generic.notification.v1.event.HighPriorityNotificationRequested",
		PartitionKey: "user-otp",
		Payload:      []byte(`{"target":{"userId":"user-otp"}}`),
	}

	err := pubNoHighWriter.PublishBatch(context.Background(), []*port.OutboxRecord{highPriorityRecord})
	if err == nil {
		t.Fatal("expected error when highPriorityWriter is nil, got nil")
	}
	if !strings.Contains(err.Error(), "high-priority kafka writer is not configured") {
		t.Errorf("unexpected error message: %v", err)
	}

	// 2. Khi highPriorityWriter được cung cấp: Định tuyến chính xác vào high-priority writer
	highWriter := &kafka.Writer{
		Addr:  kafka.TCP("127.0.0.1:9092"),
		Topic: "notification.events.high-priority",
	}
	pubWithHighWriter := authKafka.NewPublisher(defaultWriter, highWriter)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel to test dispatch logic without network write

	err = pubWithHighWriter.PublishBatch(ctx, []*port.OutboxRecord{highPriorityRecord})
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
	if strings.Contains(err.Error(), "high-priority kafka writer is not configured") {
		t.Errorf("got unconfigured error but highPriorityWriter was provided: %v", err)
	}
}
