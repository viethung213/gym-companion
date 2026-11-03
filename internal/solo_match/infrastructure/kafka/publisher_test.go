package kafka_test

import (
	"context"
	"strings"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	soloMatchKafka "github.com/viethung213/gym-companion/internal/solo_match/infrastructure/kafka"
)

func TestPublisher_PublishBatch_Empty(t *testing.T) {
	writer := &kafka.Writer{}
	pub := soloMatchKafka.NewPublisher(writer)

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
	pub := soloMatchKafka.NewPublisher(writer)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	records := []*port.OutboxRecord{
		{
			ID:           "rec-1",
			EventType:    "contracts.core.solo_match.v1.event.SoloMatchStarted",
			PartitionKey: "p1-user",
			Payload:      []byte(`{"matchId":"m-1"}`),
		},
	}

	err := pub.PublishBatch(ctx, records)
	if err == nil {
		t.Fatal("PublishBatch() got nil error, want error on canceled context")
	}
	if !strings.Contains(err.Error(), "write solo_match kafka batch messages") {
		t.Errorf("got err = %v, want error wrapped with 'write solo_match kafka batch messages'", err)
	}
}
