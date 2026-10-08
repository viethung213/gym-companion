package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
)

func TestIngestWorkoutActivityHandler(t *testing.T) {
	feedItemRepo := &mockFeedItemRepo{items: make(map[string]*aggregate.FeedItem)}
	tx := &mockTxManager{}
	handler := NewIngestWorkoutActivityHandler(feedItemRepo, tx)

	t.Run("valid workout activity ingestion", func(t *testing.T) {
		err := handler.Handle(context.Background(), IngestWorkoutActivityCommand{
			SessionID:       "sess-1",
			UserID:          "u-1",
			Title:           "Leg Day Hypertrophy",
			Caption:         "Crushed my PRs today!",
			MediaURLs:       []string{"https://img.com/squat.jpg"},
			DurationSeconds: 3600,
			TotalVolumeKg:   4500.5,
			TotalSets:       18,
			SharedAt:        time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(feedItemRepo.items) != 1 {
			t.Fatalf("expected 1 feed item ingested, got %d", len(feedItemRepo.items))
		}
	})

	t.Run("empty user id rejected", func(t *testing.T) {
		err := handler.Handle(context.Background(), IngestWorkoutActivityCommand{
			SessionID: "sess-1",
			UserID:    "",
			Title:     "Leg Day",
		})
		if !errors.Is(err, derror.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})
}
