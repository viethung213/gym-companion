package event_test

import (
	"encoding/json"
	"testing"
	"time"

	domainEvent "github.com/viethung213/gym-companion/internal/social/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/social/infrastructure/event"
)

func TestPackNotifications_UserFollowed(t *testing.T) {
	ev := &domainEvent.UserFollowedEvent{
		FollowerID:        "user-follower",
		FollowingID:       "user-following",
		FollowerName:      "Alice",
		FollowerAvatarURL: "https://example.com/avatar.png",
		FollowedAt:        time.Now().UTC(),
	}

	records, err := infraEvent.PackNotifications(ev)
	if err != nil {
		t.Fatalf("PackNotifications failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.PartitionKey != "user-following" {
		t.Errorf("got partitionKey %s, want user-following", rec.PartitionKey)
	}
	if rec.AggregateType != "SocialNotification" {
		t.Errorf("got aggregateType %s, want SocialNotification", rec.AggregateType)
	}

	var envelope infraEvent.CloudEventEnvelope
	if err := json.Unmarshal(rec.Payload, &envelope); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if envelope.SpecVersion != "1.0" {
		t.Errorf("got specversion %s, want 1.0", envelope.SpecVersion)
	}
	if envelope.Source != "services/social-service" {
		t.Errorf("got source %s, want services/social-service", envelope.Source)
	}
}
