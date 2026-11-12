package worker

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type mockFollowBuffer struct {
	tasks []port.FollowFlushTask
	acked []string
}

func (m *mockFollowBuffer) BufferFollow(ctx context.Context, followerID, followingID string, isFollowing bool, holdDuration, ttl time.Duration) error {
	return nil
}

func (m *mockFollowBuffer) GetPendingFollow(ctx context.Context, followerID, followingID string) (bool, bool, error) {
	return false, false, nil
}

func (m *mockFollowBuffer) PopMaturedFollows(ctx context.Context, now time.Time, limit int) ([]port.FollowFlushTask, error) {
	t := m.tasks
	m.tasks = nil
	return t, nil
}

func (m *mockFollowBuffer) AckFollow(ctx context.Context, followerID, followingID string) error {
	m.acked = append(m.acked, followerID+":"+followingID)
	return nil
}

type mockWorkerFollowRepo struct {
	repository.FollowRepository
	follows map[string]bool
}

func (m *mockWorkerFollowRepo) Follow(ctx context.Context, follow *aggregate.Follow) error {
	m.follows[follow.FollowerID()+":"+follow.FollowingID()] = true
	return nil
}

func (m *mockWorkerFollowRepo) Unfollow(ctx context.Context, followerID, followingID string) error {
	delete(m.follows, followerID+":"+followingID)
	return nil
}

func (m *mockWorkerFollowRepo) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	return m.follows[followerID+":"+followingID], nil
}

type mockWorkerUserSnapshotRepo struct {
	repository.UserSnapshotRepository
	counts map[string]int
}

func (m *mockWorkerUserSnapshotRepo) GetByID(ctx context.Context, id string) (*entity.UserSnapshot, error) {
	return entity.NewUserSnapshot(id, "Test User", "https://example.com/avatar.png", "user", time.Time{}), nil
}

func (m *mockWorkerUserSnapshotRepo) UpdateFollowCounts(ctx context.Context, followerID, followingID string, delta int) error {
	m.counts[followerID+":following"] += delta
	m.counts[followingID+":followers"] += delta
	return nil
}

func TestFollowDebounceWorker_ProcessBatch(t *testing.T) {
	followRepo := &mockWorkerFollowRepo{follows: make(map[string]bool)}
	uRepo := &mockWorkerUserSnapshotRepo{counts: make(map[string]int)}
	eventRec := &mockWorkerEventRecorder{}
	tx := &mockWorkerTxManager{}

	buf := &mockFollowBuffer{
		tasks: []port.FollowFlushTask{
			{FollowerID: "u-f1", FollowingID: "u-f2", IsFollowing: true},
		},
	}

	worker := NewFollowDebounceWorker(buf, followRepo, uRepo, eventRec, tx, 100*time.Millisecond)

	ctx := context.Background()

	// 1. Process Follow task
	if err := worker.ProcessBatch(ctx); err != nil {
		t.Fatalf("process batch 1 failed: %v", err)
	}

	if !followRepo.follows["u-f1:u-f2"] {
		t.Errorf("expected follow relation created in DB")
	}
	if len(eventRec.events) != 1 {
		t.Errorf("got %d events recorded, want 1", len(eventRec.events))
	}
	if len(buf.acked) != 1 || buf.acked[0] != "u-f1:u-f2" {
		t.Errorf("got acked %v, want [u-f1:u-f2]", buf.acked)
	}

	// 2. Process Unfollow task
	buf.tasks = []port.FollowFlushTask{
		{FollowerID: "u-f1", FollowingID: "u-f2", IsFollowing: false},
	}
	if err := worker.ProcessBatch(ctx); err != nil {
		t.Fatalf("process batch 2 failed: %v", err)
	}

	if followRepo.follows["u-f1:u-f2"] {
		t.Errorf("expected follow relation removed from DB")
	}
	if len(buf.acked) != 2 {
		t.Errorf("got %d acked tasks, want 2", len(buf.acked))
	}
}
