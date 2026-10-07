package aggregate

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestNewFeedItem_Post(t *testing.T) {
	t.Run("valid post item", func(t *testing.T) {
		p, err := NewPostItem("item-1", "user-a", "Great workout today!", []string{"http://img.jpg"}, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.ItemType() != vo.ItemTypePost {
			t.Errorf("expected item type POST")
		}
		if p.ReactionCount() != 0 || p.CommentCount() != 0 {
			t.Errorf("expected initial counts 0")
		}

		p.IncrementReaction()
		if p.ReactionCount() != 1 {
			t.Errorf("expected reaction count 1")
		}
		p.DecrementReaction()
		if p.ReactionCount() != 0 {
			t.Errorf("expected reaction count 0")
		}
		// Decrement when 0 does not underflow
		p.DecrementReaction()
		if p.ReactionCount() != 0 {
			t.Errorf("expected reaction count still 0")
		}

		p.IncrementComment()
		if p.CommentCount() != 1 {
			t.Errorf("expected comment count 1")
		}
		p.DecrementComment()
		if p.CommentCount() != 0 {
			t.Errorf("expected comment count 0")
		}
		// Decrement when 0 does not underflow
		p.DecrementComment()
		if p.CommentCount() != 0 {
			t.Errorf("expected comment count still 0")
		}
	})

	t.Run("empty content and no media rejected", func(t *testing.T) {
		_, err := NewPostItem("item-1", "user-a", "   ", nil, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
		if err != derror.ErrEmptyContent {
			t.Errorf("expected ErrEmptyContent, got %v", err)
		}
	})

	t.Run("valid post with media only and empty caption", func(t *testing.T) {
		p, err := NewPostItem("item-1", "user-a", "", []string{"http://img.jpg"}, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Caption() != "" || len(p.MediaURLs()) != 1 {
			t.Errorf("unexpected post fields")
		}
	})

	t.Run("valid post with caption only and no media", func(t *testing.T) {
		p, err := NewPostItem("item-1", "user-a", "Just text", nil, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Caption() != "Just text" || len(p.MediaURLs()) != 0 {
			t.Errorf("unexpected post fields")
		}
	})
}

func TestNewFeedItem_WorkoutActivity(t *testing.T) {
	t.Run("valid workout activity item", func(t *testing.T) {
		metrics := vo.NewWorkoutMetrics("sess-1", "Leg Day", 3600, 5000, 5, 20, 2)
		a, err := NewWorkoutActivityItem("act-1", "user-a", "Heavy squats today!", nil, metrics, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.ItemType() != vo.ItemTypeWorkoutActivity {
			t.Errorf("expected item type WORKOUT_ACTIVITY")
		}
		if a.WorkoutData().TotalVolumeKg() != 5000 {
			t.Errorf("unexpected volume")
		}

		a.IncrementReaction()
		if a.ReactionCount() != 1 {
			t.Errorf("expected 1 reaction")
		}
		a.DecrementReaction()
		if a.ReactionCount() != 0 {
			t.Errorf("expected 0 reaction")
		}
	})

	t.Run("valid workout with workoutData only, empty caption and empty media", func(t *testing.T) {
		metrics := vo.NewWorkoutMetrics("sess-1", "Leg Day", 3600, 5000, 5, 20, 2)
		a, err := NewWorkoutActivityItem("act-1", "user-a", "", nil, metrics, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.Caption() != "" || len(a.MediaURLs()) != 0 || a.WorkoutData().IsZero() {
			t.Errorf("unexpected workout fields")
		}
	})

	t.Run("valid workout with caption only, empty workoutData and empty media", func(t *testing.T) {
		emptyMetrics := vo.NewWorkoutMetrics("", "", 0, 0, 0, 0, 0)
		a, err := NewWorkoutActivityItem("act-1", "user-a", "Only caption", nil, emptyMetrics, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.Caption() != "Only caption" {
			t.Errorf("unexpected caption")
		}
	})

	t.Run("valid workout with media only, empty caption and empty workoutData", func(t *testing.T) {
		emptyMetrics := vo.NewWorkoutMetrics("", "", 0, 0, 0, 0, 0)
		a, err := NewWorkoutActivityItem("act-1", "user-a", "", []string{"http://gym.jpg"}, emptyMetrics, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(a.MediaURLs()) != 1 {
			t.Errorf("unexpected mediaURLs")
		}
	})

	t.Run("all three empty rejected in workout activity", func(t *testing.T) {
		emptyMetrics := vo.NewWorkoutMetrics("", "", 0, 0, 0, 0, 0)
		_, err := NewWorkoutActivityItem("act-1", "user-a", "   ", nil, emptyMetrics, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
		if err != derror.ErrEmptyContent {
			t.Errorf("expected ErrEmptyContent when caption, mediaURLs and workoutData are all empty, got %v", err)
		}
	})

	t.Run("unauthorized when empty user", func(t *testing.T) {
		metrics := vo.NewWorkoutMetrics("sess-1", "Leg Day", 3600, 5000, 5, 20, 2)
		_, err := NewWorkoutActivityItem("act-1", "", "Leg Day", nil, metrics, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
		if err != derror.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})
}
