package event

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
)

type stubOutbox struct {
	records []*port.OutboxRecord
}

func (s *stubOutbox) Save(ctx context.Context, rec *port.OutboxRecord) error {

	s.records = append(s.records, rec)

	return nil

}

func (s *stubOutbox) FetchUnpublished(context.Context, int) ([]*port.OutboxRecord, error) {

	return s.records, nil

}

func (s *stubOutbox) MarkPublished(context.Context, []string) error { return nil }

func (s *stubOutbox) ExecuteInLock(ctx context.Context, _ int64, fn func(context.Context) error) error {

	return fn(ctx)

}

func (s *stubOutbox) LogProcessed(context.Context, string, string, string, []byte) (bool, error) {

	return true, nil

}

func TestOutboxWriter_Enqueue_CloudEventsEnvelope(t *testing.T) {

	stub := &stubOutbox{}

	w := NewOutboxWriter(stub)

	ev := &domainevent.RoadmapInitiated{

		RoadmapID: "rm-1",

		UserID: "user-1",

		InitiatedAt: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
	}

	if err := w.Enqueue(context.Background(), "user-1", ev); err != nil {

		t.Fatalf("Enqueue: %v", err)

	}

	if len(stub.records) != 1 {

		t.Fatalf("expected 1 record, got %d", len(stub.records))

	}

	rec := stub.records[0]

	if rec.EventType != ev.EventName() {

		t.Errorf("event_type=%s, want %s", rec.EventType, ev.EventName())

	}

	if rec.PartitionKey != "user-1" {

		t.Errorf("partition_key=%s, want user-1", rec.PartitionKey)

	}

	var envelope map[string]any

	if err := json.Unmarshal(rec.Payload, &envelope); err != nil {

		t.Fatalf("unmarshal envelope: %v", err)

	}

	if envelope["specversion"] != "1.0" {

		t.Errorf("specversion=%v", envelope["specversion"])

	}

	if envelope["source"] != Source {

		t.Errorf("source=%v", envelope["source"])

	}

	if envelope["type"] != ev.EventName() {

		t.Errorf("type=%v", envelope["type"])

	}

	if envelope["data"] == nil {

		t.Errorf("missing data field")

	}

}

func TestOutboxWriter_Enqueue_MultipleEvents(t *testing.T) {

	stub := &stubOutbox{}

	w := NewOutboxWriter(stub)

	err := w.Enqueue(context.Background(), "u1",

		&domainevent.RoadmapInitiated{RoadmapID: "rm-1", UserID: "u1"},

		&domainevent.RoadmapAdjusted{RoadmapID: "rm-1", UserID: "u1", Reason: "test"},
	)

	if err != nil {

		t.Fatalf("Enqueue: %v", err)

	}

	if len(stub.records) != 2 {

		t.Errorf("expected 2 records, got %d", len(stub.records))

	}

}
