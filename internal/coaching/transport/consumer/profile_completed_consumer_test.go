package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

func makeProfileCE(t *testing.T, id, typeStr string, dataObj any) []byte {
	t.Helper()

	dataBytes, err := json.Marshal(dataObj)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}

	env := map[string]any{
		"specversion": "1.0",
		"id":          id,
		"source":      "services/profile-service",
		"type":        typeStr,
		"data":        json.RawMessage(dataBytes),
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal env: %v", err)
	}

	return b
}

func TestProfileCompletedConsumer_UnknownEventType_Ignored(t *testing.T) {
	// Consumer should not error on unrelated event types on the same topic.
	c := NewProfileCompletedConsumer(nil, nil, nil, &stubOutbox{})

	raw := makeProfileCE(t, "evt-1", "some.unrelated.event",
		map[string]string{"userId": "user-1"})

	if err := c.HandleMessage(context.Background(), raw); err != nil {
		t.Errorf("expected nil for unknown type, got %v", err)
	}
}

func TestProfileCompletedConsumer_MissingEventID(t *testing.T) {
	c := NewProfileCompletedConsumer(nil, nil, nil, &stubOutbox{})

	raw := makeProfileCE(t, "", profileCompletedEventType,
		ProfileEventPayload{UserID: "user-1"})

	if err := c.HandleMessage(context.Background(), raw); err == nil {
		t.Errorf("expected error for missing event id")
	}
}

func TestProfileCompletedConsumer_DuplicateEvent_Skipped(t *testing.T) {
	stub := &stubOutbox{}
	c := NewProfileCompletedConsumer(nil, nil, nil, stub)

	// Use empty userId so that even if handler were called, it would no-op.
	raw := makeProfileCE(t, "evt-dup", profileCompletedEventType,
		ProfileEventPayload{UserID: ""})

	// First call: fresh — handler receives empty userId and ignores.
	if err := c.HandleMessage(context.Background(), raw); err != nil {
		t.Fatalf("first: %v", err)
	}

	// Second call: duplicate — outbox_log dedup returns fresh=false.
	if err := c.HandleMessage(context.Background(), raw); err != nil {
		t.Errorf("second (duplicate): %v", err)
	}
}

func TestProfileCompletedConsumer_MissingUserID_Ignored(t *testing.T) {
	c := NewProfileCompletedConsumer(nil, nil, nil, &stubOutbox{})

	raw := makeProfileCE(t, "evt-no-uid", profileCompletedEventType,
		ProfileEventPayload{UserID: ""})

	if err := c.HandleMessage(context.Background(), raw); err != nil {
		t.Errorf("expected nil for missing user_id, got %v", err)
	}
}

func TestProfileCompletedConsumer_ProfileUpdatedAndInjuryEvents(t *testing.T) {
	c := NewProfileCompletedConsumer(nil, nil, nil, &stubOutbox{})

	// Test ProfileUpdated
	rawUpd := makeProfileCE(t, "evt-upd", profileUpdatedEventType,
		ProfileEventPayload{UserID: "usr-1"})
	if err := c.HandleMessage(context.Background(), rawUpd); err != nil {
		t.Errorf("expected nil for ProfileUpdated with nil handler, got %v", err)
	}

	// Test InjuryReported
	rawInj := makeProfileCE(t, "evt-inj", injuryReportedEventType,
		ProfileEventPayload{UserID: "usr-1", MuscleGroup: "Chest"})
	if err := c.HandleMessage(context.Background(), rawInj); err != nil {
		t.Errorf("expected nil for InjuryReported with nil handler, got %v", err)
	}

	// Test InjuryRecovered
	rawRec := makeProfileCE(t, "evt-rec", injuryRecoveredEventType,
		ProfileEventPayload{UserID: "usr-1"})
	if err := c.HandleMessage(context.Background(), rawRec); err != nil {
		t.Errorf("expected nil for InjuryRecovered with nil handler, got %v", err)
	}
}

func TestProfileCompletedConsumer_InvalidJSON(t *testing.T) {
	c := NewProfileCompletedConsumer(nil, nil, nil, &stubOutbox{})

	if err := c.HandleMessage(context.Background(), []byte("not json")); err == nil {
		t.Errorf("expected error for invalid json")
	}
}

// --- outbox error stub ---

type failOutbox struct {
	stubOutbox
}

func (f *failOutbox) LogProcessed(context.Context, string, string, string, []byte) (bool, error) {
	return false, errors.New("outbox db error")
}

func TestProfileCompletedConsumer_OutboxError(t *testing.T) {
	c := NewProfileCompletedConsumer(nil, nil, nil, &failOutbox{})

	raw := makeProfileCE(t, "evt-err", profileCompletedEventType,
		ProfileEventPayload{UserID: "user-1"})

	err := c.HandleMessage(context.Background(), raw)
	if err == nil {
		t.Errorf("expected error from outbox failure")
	}

	if got, want := err.Error(), "log processed"; !contains(got, want) {
		t.Errorf("error = %q, want containing %q", got, want)
	}
}

// --- mock roadmap repo for ErrActiveRoadmapExists test ---

type alwaysExistsRepo struct{ port.RoadmapRepository }

func (alwaysExistsRepo) FindActiveByUser(context.Context, string) (*roadmap.Roadmap, error) {
	return &roadmap.Roadmap{}, nil
}

func (alwaysExistsRepo) Save(context.Context, *roadmap.Roadmap) error { return nil }
func (alwaysExistsRepo) FindByID(context.Context, string) (*roadmap.Roadmap, error) {
	return nil, roadmap.ErrRoadmapNotFound
}
func (alwaysExistsRepo) ListByUser(context.Context, string, roadmap.Status, int, int) ([]*roadmap.Roadmap, error) {
	return nil, nil
}
func (alwaysExistsRepo) FindSessionByID(context.Context, string) (*roadmap.Roadmap, error) {
	return nil, roadmap.ErrSessionNotFound
}
func (alwaysExistsRepo) FindPendingSessionsByDate(context.Context, time.Time) ([]*roadmap.SessionPlanInfo, error) {
	return nil, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
