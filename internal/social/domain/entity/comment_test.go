package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

func TestNewComment(t *testing.T) {
	now := time.Now().UTC()

	t.Run("valid comment", func(t *testing.T) {
		c, err := entity.NewComment("c-1", "u-1", "f-1", nil, "Great job!", now, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.ID() != "c-1" || c.UserID() != "u-1" || c.FeedItemID() != "f-1" || c.Content() != "Great job!" {
			t.Errorf("unexpected comment properties")
		}
	})

	t.Run("empty user id rejected", func(t *testing.T) {
		_, err := entity.NewComment("c-1", "   ", "f-1", nil, "Great job!", now, now)
		if !errors.Is(err, derror.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("empty feed item id rejected", func(t *testing.T) {
		_, err := entity.NewComment("c-1", "u-1", "", nil, "Great job!", now, now)
		if !errors.Is(err, derror.ErrPostNotFound) {
			t.Errorf("expected ErrPostNotFound, got %v", err)
		}
	})

	t.Run("empty content rejected", func(t *testing.T) {
		_, err := entity.NewComment("c-1", "u-1", "f-1", nil, "   ", now, now)
		if !errors.Is(err, derror.ErrEmptyContent) {
			t.Errorf("expected ErrEmptyContent, got %v", err)
		}
	})
}
