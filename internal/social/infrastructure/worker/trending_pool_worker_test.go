package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type mockFeedItemRepoForWorker struct {
	trendingIDs []string
	err         error
}

func (m *mockFeedItemRepoForWorker) Create(ctx context.Context, item *aggregate.FeedItem) error {
	return nil
}
func (m *mockFeedItemRepoForWorker) GetByID(ctx context.Context, id string) (*aggregate.FeedItem, error) {
	return nil, nil
}
func (m *mockFeedItemRepoForWorker) GetByIDs(ctx context.Context, ids []string) ([]*aggregate.FeedItem, error) {
	return nil, nil
}
func (m *mockFeedItemRepoForWorker) Delete(ctx context.Context, id, userID string) error { return nil }
func (m *mockFeedItemRepoForWorker) GetByUserID(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	return nil, "", nil
}
func (m *mockFeedItemRepoForWorker) GetByAuthorIDs(ctx context.Context, authorIDs []string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	return nil, "", nil
}
func (m *mockFeedItemRepoForWorker) GetTrendingCandidateIDs(ctx context.Context, limit, hours int) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.trendingIDs, nil
}
func (m *mockFeedItemRepoForWorker) GetPublicFeedItems(ctx context.Context, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	return nil, "", nil
}
func (m *mockFeedItemRepoForWorker) CountByUserID(ctx context.Context, userID string) (int32, error) {
	return 0, nil
}
func (m *mockFeedItemRepoForWorker) UpdateReactionCount(ctx context.Context, id string, delta int) error {
	return nil
}
func (m *mockFeedItemRepoForWorker) UpdateCommentCount(ctx context.Context, id string, delta int) error {
	return nil
}
func (m *mockFeedItemRepoForWorker) SearchPosts(ctx context.Context, filter repository.SearchPostsFilter) ([]*aggregate.FeedItem, string, int32, error) {
	return nil, "", 0, nil
}

type mockTrendingPoolCacheForWorker struct {
	savedPool []string
	saveErr   error
}

func (m *mockTrendingPoolCacheForWorker) SaveTrendingPool(ctx context.Context, itemIDs []string) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.savedPool = itemIDs
	return nil
}

func (m *mockTrendingPoolCacheForWorker) GetTrendingPool(ctx context.Context) ([]string, error) {
	return m.savedPool, nil
}

func TestTrendingPoolWorker_Refresh(t *testing.T) {
	t.Run("successful refresh", func(t *testing.T) {
		repo := &mockFeedItemRepoForWorker{trendingIDs: []string{"hot-1", "hot-2"}}
		cache := &mockTrendingPoolCacheForWorker{}

		w := NewTrendingPoolWorker(repo, cache, time.Second)
		if err := w.Refresh(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(cache.savedPool) != 2 || cache.savedPool[0] != "hot-1" {
			t.Fatalf("expected saved pool [hot-1, hot-2], got %v", cache.savedPool)
		}
	})

	t.Run("repo error returns error", func(t *testing.T) {
		repo := &mockFeedItemRepoForWorker{err: errors.New("db error")}
		cache := &mockTrendingPoolCacheForWorker{}

		w := NewTrendingPoolWorker(repo, cache, time.Second)
		if err := w.Refresh(context.Background()); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("trending cache save error returns error", func(t *testing.T) {
		repo := &mockFeedItemRepoForWorker{trendingIDs: []string{"hot-1"}}
		cache := &mockTrendingPoolCacheForWorker{saveErr: errors.New("redis save error")}

		w := NewTrendingPoolWorker(repo, cache, 0) // interval <= 0 tests default
		if err := w.Refresh(context.Background()); err == nil {
			t.Fatal("expected save error, got nil")
		}
	})

	t.Run("lifecycle start and cancel", func(t *testing.T) {
		repo := &mockFeedItemRepoForWorker{trendingIDs: []string{"hot-1"}}
		cache := &mockTrendingPoolCacheForWorker{}

		w := NewTrendingPoolWorker(repo, cache, 10*time.Millisecond)
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			w.Start(ctx)
			close(done)
		}()

		time.Sleep(30 * time.Millisecond)
		cancel()

		select {
		case <-done:
			// successfully exited
		case <-time.After(500 * time.Millisecond):
			t.Fatal("worker did not exit on cancel")
		}
	})
}
