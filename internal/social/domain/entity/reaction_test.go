package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestNewReaction(t *testing.T) {
	now := time.Now().UTC()

	t.Run("valid reaction", func(t *testing.T) {
		r, err := entity.NewReaction("r-1", "u-1", "f-1", vo.ReactionTypeLike, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.ID() != "r-1" || r.UserID() != "u-1" || r.FeedItemID() != "f-1" || r.ReactionType() != vo.ReactionTypeLike {
			t.Errorf("unexpected reaction properties")
		}
		r.ChangeReactionType(vo.ReactionTypeFire)
		if r.ReactionType() != vo.ReactionTypeFire {
			t.Errorf("expected reaction type changed to FIRE")
		}
	})

	t.Run("empty user id rejected", func(t *testing.T) {
		_, err := entity.NewReaction("r-1", "", "f-1", vo.ReactionTypeLike, now)
		if !errors.Is(err, derror.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("empty feed item id rejected", func(t *testing.T) {
		_, err := entity.NewReaction("r-1", "u-1", "", vo.ReactionTypeLike, now)
		if !errors.Is(err, derror.ErrPostNotFound) {
			t.Errorf("expected ErrPostNotFound, got %v", err)
		}
	})

	t.Run("empty reaction type rejected", func(t *testing.T) {
		_, err := entity.NewReaction("r-1", "u-1", "f-1", "", now)
		if !errors.Is(err, derror.ErrInvalidReactionType) {
			t.Errorf("expected ErrInvalidReactionType, got %v", err)
		}
	})
}
