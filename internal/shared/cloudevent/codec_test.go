package cloudevent_test

import (
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/shared/cloudevent"
)

func TestEncodeDecode(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 19, 1, 2, 3, 0, time.UTC)
	encoded, err := cloudevent.Encode(
		"event-1",
		"services/coaching-service",
		"contracts.core.coaching.v1.roadmapInitiated",
		now,
		[]byte(`{"roadmapId":"roadmap-1"}`),
	)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	got, err := cloudevent.Decode(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != "event-1" {
		t.Errorf("ID got = %q, want = %q", got.ID, "event-1")
	}
}

func TestDecodeRejectsMissingID(t *testing.T) {
	t.Parallel()

	_, err := cloudevent.Decode([]byte(`{
		"specversion":"1.0",
		"source":"services/profile-service",
		"type":"contracts.supporting.profile.v1.profileCompleted",
		"time":"2026-07-19T00:00:00Z",
		"datacontenttype":"application/json",
		"data":{}
	}`))
	if !errors.Is(err, cloudevent.ErrInvalidEnvelope) {
		t.Fatalf("error got = %v, want ErrInvalidEnvelope", err)
	}
}
