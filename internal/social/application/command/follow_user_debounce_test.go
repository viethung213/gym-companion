package command

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

type mockFollowDebounceBuffer struct {
	buffered map[string]bool // key = followerID:followingID -> isFollowing
	lastHold time.Duration
	lastTTL  time.Duration
}

func newMockFollowDebounceBuffer() *mockFollowDebounceBuffer {
	return &mockFollowDebounceBuffer{
		buffered: make(map[string]bool),
	}
}

func (m *mockFollowDebounceBuffer) BufferFollow(
	ctx context.Context,
	followerID, followingID string,
	isFollowing bool,
	holdDuration, ttl time.Duration,
) error {
	m.buffered[followerID+":"+followingID] = isFollowing
	m.lastHold = holdDuration
	m.lastTTL = ttl
	return nil
}

func (m *mockFollowDebounceBuffer) GetPendingFollow(
	ctx context.Context,
	followerID, followingID string,
) (bool, bool, error) {
	val, ok := m.buffered[followerID+":"+followingID]
	return val, ok, nil
}

func (m *mockFollowDebounceBuffer) PopMaturedFollows(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]port.FollowFlushTask, error) {
	return nil, nil
}

func (m *mockFollowDebounceBuffer) AckFollow(ctx context.Context, followerID, followingID string) error {
	delete(m.buffered, followerID+":"+followingID)
	return nil
}

func TestFollowUser_DebounceSpam(t *testing.T) {
	followRepo := &mockFollowRepo{follows: make(map[string]*aggregate.Follow)}
	uInfo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"u-follower":  entity.NewUserSnapshot("u-follower", "Follower", "https://example.com/f.png", "user", time.Time{}),
			"u-following": entity.NewUserSnapshot("u-following", "Following", "https://example.com/t.png", "user", time.Time{}),
		},
	}
	pub := &mockEventPublisher{}
	tx := &mockTxManager{}
	buf := newMockFollowDebounceBuffer()

	followHandler := NewFollowUserHandler(
		followRepo,
		uInfo,
		pub,
		tx,
		WithFollowDebounceBuffer(buf),
	)

	unfollowHandler := NewUnfollowUserHandler(
		followRepo,
		uInfo,
		tx,
		WithUnfollowDebounceBuffer(buf),
	)

	ctx := context.Background()

	// 1. User clicks Follow
	if err := followHandler.Handle(ctx, FollowUserCommand{FollowerID: "u-follower", FollowingID: "u-following"}); err != nil {
		t.Fatalf("follow 1 failed: %v", err)
	}
	isFollowing, exists, _ := buf.GetPendingFollow(ctx, "u-follower", "u-following")
	if !exists || !isFollowing {
		t.Errorf("got isFollowing=%v, exists=%v, want true, true", isFollowing, exists)
	}

	// 2. User quickly clicks Unfollow
	if err := unfollowHandler.Handle(ctx, UnfollowUserCommand{FollowerID: "u-follower", FollowingID: "u-following"}); err != nil {
		t.Fatalf("unfollow failed: %v", err)
	}
	isFollowing, exists, _ = buf.GetPendingFollow(ctx, "u-follower", "u-following")
	if !exists || isFollowing {
		t.Errorf("got isFollowing=%v, exists=%v, want false, true", isFollowing, exists)
	}

	// 3. User quickly clicks Follow again (Final state = Follow)
	if err := followHandler.Handle(ctx, FollowUserCommand{FollowerID: "u-follower", FollowingID: "u-following"}); err != nil {
		t.Fatalf("follow 2 failed: %v", err)
	}
	isFollowing, exists, _ = buf.GetPendingFollow(ctx, "u-follower", "u-following")
	if !exists || !isFollowing {
		t.Errorf("got isFollowing=%v, exists=%v, want true, true", isFollowing, exists)
	}

	// Verify hold duration and TTL
	if buf.lastHold != 5*time.Second {
		t.Errorf("got hold duration %v, want 5s", buf.lastHold)
	}
	if buf.lastTTL != 60*time.Second {
		t.Errorf("got timeout TTL %v, want 60s", buf.lastTTL)
	}

	// DB is untouched yet and no event published
	if len(pub.published) != 0 {
		t.Errorf("expected 0 events published immediately during debounce, got %d", len(pub.published))
	}
}
