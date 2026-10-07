package persistence_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
	"github.com/viethung213/gym-companion/internal/social/infrastructure/persistence"
)

func TestMapper_Follow(t *testing.T) {
	now := time.Now().UTC()
	f, err := aggregate.NewFollow("f-1", "u-1", "u-2", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	model := persistence.ToPersistenceFollow(f)
	if model.ID != "f-1" || model.FollowerID != "u-1" || model.FollowingID != "u-2" {
		t.Fatalf("unexpected persistence model: %+v", model)
	}

	domainFollow, err := persistence.ToDomainFollow(model)
	if err != nil {
		t.Fatalf("unexpected error mapping to domain: %v", err)
	}
	if domainFollow.ID() != f.ID() || domainFollow.FollowerID() != f.FollowerID() {
		t.Fatalf("mismatched domain follow: %+v", domainFollow)
	}
}

func TestMapper_FeedItem_Post(t *testing.T) {
	now := time.Now().UTC()
	post, err := aggregate.NewPostItem(
		"item-1",
		"u-1",
		"Test Post Caption",
		[]string{"https://img.com/1.png"},
		vo.VisibilityPublic,
		5,
		2,
		now,
		now,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	model := persistence.ToPersistenceFeedItem(post)
	if model.ID != "item-1" || model.Caption != "Test Post Caption" || model.ItemType != "POST" {
		t.Fatalf("unexpected model: %+v", model)
	}

	domainItem, err := persistence.ToDomainFeedItem(model)
	if err != nil {
		t.Fatalf("unexpected domain conversion error: %v", err)
	}
	if domainItem.ID() != post.ID() || domainItem.Caption() != post.Caption() {
		t.Fatalf("domain item mismatch")
	}
}

func TestMapper_FeedItem_WorkoutActivity(t *testing.T) {
	now := time.Now().UTC()
	metrics := vo.NewWorkoutMetrics("sess-1", "Leg Workout", 3600, 5000.0, 5, 20, 1)
	workoutItem, err := aggregate.NewWorkoutActivityItem(
		"item-2",
		"u-2",
		"Workout Caption",
		[]string{"https://img.com/workout.png"},
		metrics,
		vo.VisibilityFollowersOnly,
		10,
		4,
		now,
		now,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	model := persistence.ToPersistenceFeedItem(workoutItem)
	if model.ItemType != "WORKOUT_ACTIVITY" {
		t.Fatalf("expected WORKOUT_ACTIVITY, got %s", model.ItemType)
	}

	domainItem, err := persistence.ToDomainFeedItem(model)
	if err != nil {
		t.Fatalf("unexpected domain conversion error: %v", err)
	}
	if domainItem.WorkoutData().WorkoutTitle() != "Leg Workout" {
		t.Fatalf("workout title mismatch: %s", domainItem.WorkoutData().WorkoutTitle())
	}
}

func TestMapper_Comment(t *testing.T) {
	now := time.Now().UTC()
	c, err := entity.NewComment("c-1", "u-1", "item-1", nil, "Great job!", now, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	model := persistence.ToPersistenceComment(c)
	if model.ID != "c-1" || model.Content != "Great job!" {
		t.Fatalf("unexpected comment model: %+v", model)
	}

	domainComment, err := persistence.ToDomainComment(model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domainComment.Content() != c.Content() {
		t.Fatalf("content mismatch")
	}
}

func TestMapper_Reaction(t *testing.T) {
	now := time.Now().UTC()
	r, err := entity.NewReaction("r-1", "u-1", "item-1", vo.ReactionTypeFire, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	model := persistence.ToPersistenceReaction(r)
	if model.ID != "r-1" || model.ReactionType != "FIRE" {
		t.Fatalf("unexpected reaction model: %+v", model)
	}

	domainReaction, err := persistence.ToDomainReaction(model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domainReaction.ReactionType() != r.ReactionType() {
		t.Fatalf("reaction type mismatch")
	}
}

func TestMapper_UserSnapshot(t *testing.T) {
	now := time.Now().UTC()
	u := entity.NewUserSnapshot("u-1", "Alice", "https://avatar.png", "user", now)

	model := persistence.ToPersistenceUserSnapshot(u)
	if model.ID != "u-1" || model.FullName != "Alice" || model.Role != "user" {
		t.Fatalf("unexpected snapshot model: %+v", model)
	}

	domainUser := persistence.ToDomainUserSnapshot(model)
	if domainUser.FullName() != u.FullName() || domainUser.Role() != "user" {
		t.Fatalf("fullname or role mismatch")
	}
}
