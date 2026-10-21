package query

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type searchTestMockFeedRepo struct {
	items      []*aggregate.FeedItem
	err        error
	lastFilter repository.SearchPostsFilter
}

func (m *searchTestMockFeedRepo) Create(ctx context.Context, item *aggregate.FeedItem) error {
	return nil
}
func (m *searchTestMockFeedRepo) GetByID(ctx context.Context, id string) (*aggregate.FeedItem, error) {
	return nil, nil
}
func (m *searchTestMockFeedRepo) Delete(ctx context.Context, id, userID string) error { return nil }
func (m *searchTestMockFeedRepo) GetByUserID(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	return nil, "", nil
}
func (m *searchTestMockFeedRepo) GetByAuthorIDs(ctx context.Context, authorIDs []string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	return nil, "", nil
}
func (m *searchTestMockFeedRepo) CountByUserID(ctx context.Context, userID string) (int32, error) {
	return 0, nil
}
func (m *searchTestMockFeedRepo) UpdateReactionCount(ctx context.Context, id string, delta int) error {
	return nil
}
func (m *searchTestMockFeedRepo) UpdateCommentCount(ctx context.Context, id string, delta int) error {
	return nil
}
func (m *searchTestMockFeedRepo) SearchPosts(ctx context.Context, filter repository.SearchPostsFilter) ([]*aggregate.FeedItem, string, int32, error) {
	m.lastFilter = filter
	if m.err != nil {
		return nil, "", 0, m.err
	}
	return m.items, "next-cur", int32(len(m.items)), nil
}

type searchTestMockInteractionRepo struct {
	reactions map[string]*entity.Reaction
	err       error
}

func (m *searchTestMockInteractionRepo) SaveReaction(ctx context.Context, r *entity.Reaction) error {
	return nil
}
func (m *searchTestMockInteractionRepo) DeleteReaction(ctx context.Context, userID, feedItemID string) error {
	return nil
}
func (m *searchTestMockInteractionRepo) GetReaction(ctx context.Context, userID, feedItemID string) (*entity.Reaction, error) {
	if m.reactions != nil {
		return m.reactions[userID+":"+feedItemID], nil
	}
	return nil, nil
}
func (m *searchTestMockInteractionRepo) GetReactionsByFeedItem(ctx context.Context, feedItemID string) ([]*entity.Reaction, error) {
	return nil, nil
}
func (m *searchTestMockInteractionRepo) GetUserReactions(ctx context.Context, userID string, feedItemIDs []string) (map[string]*entity.Reaction, error) {
	if m.err != nil {
		return nil, m.err
	}
	res := make(map[string]*entity.Reaction)
	if m.reactions != nil {
		for _, fID := range feedItemIDs {
			if r, ok := m.reactions[userID+":"+fID]; ok {
				res[fID] = r
			}
		}
	}
	return res, nil
}
func (m *searchTestMockInteractionRepo) AddComment(ctx context.Context, comment *entity.Comment) error {
	return nil
}
func (m *searchTestMockInteractionRepo) GetCommentByID(ctx context.Context, id string) (*entity.Comment, error) {
	return nil, nil
}
func (m *searchTestMockInteractionRepo) DeleteComment(ctx context.Context, id, userID string) error {
	return nil
}
func (m *searchTestMockInteractionRepo) ListComments(ctx context.Context, feedItemID string, limit int, cursor string) ([]*entity.Comment, string, int32, error) {
	return nil, "", 0, nil
}

func TestSearchPostsHandler(t *testing.T) {
	now := time.Now().UTC()
	authorUUID1 := "00000000-0000-0000-0000-000000000001"
	authorUUID2 := "00000000-0000-0000-0000-000000000002"

	item1, err := aggregate.NewPostItem(
		"item-1",
		authorUUID1,
		"Leg day heavy squat",
		[]string{"https://img.com/1.jpg"},
		vo.VisibilityPublic,
		0,
		0,
		now,
		now,
	)
	if err != nil {
		t.Fatalf("failed to create item1: %v", err)
	}

	item2, err := aggregate.NewPostItem(
		"item-2",
		authorUUID2,
		"Bench press PR session",
		nil,
		vo.VisibilityFollowersOnly,
		0,
		0,
		now,
		now,
	)
	if err != nil {
		t.Fatalf("failed to create item2: %v", err)
	}

	userRepo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			authorUUID1: entity.NewUserSnapshot(authorUUID1, "John Doe", "https://avatar.com/john.jpg", "user", now),
		},
	}

	reaction, _ := entity.NewReaction("r-1", "u-viewer", "item-1", vo.ReactionTypeFire, now)
	interactionRepo := &searchTestMockInteractionRepo{
		reactions: map[string]*entity.Reaction{
			"u-viewer:item-1": reaction,
		},
	}

	t.Run("successfully search posts and enrich authors and reactions via batch", func(t *testing.T) {
		feedRepo := &searchTestMockFeedRepo{
			items: []*aggregate.FeedItem{item1, item2},
		}
		handler := NewSearchPostsHandler(feedRepo, interactionRepo, userRepo)

		res, err := handler.Handle(context.Background(), SearchPostsQuery{
			ViewerID: "u-viewer",
			AuthorID: authorUUID1,
			Query:    "squat",
			ItemType: "POST",
			PageSize: 10,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.TotalCount != 2 {
			t.Errorf("expected total_count = 2, got %d", res.TotalCount)
		}
		if res.NextCursor != "next-cur" {
			t.Errorf("expected next_cursor = 'next-cur', got %s", res.NextCursor)
		}
		if len(res.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(res.Items))
		}

		if res.Items[0].AuthorName != "John Doe" {
			t.Errorf("expected John Doe, got %s", res.Items[0].AuthorName)
		}
		if res.Items[0].AuthorAvatarURL != "https://avatar.com/john.jpg" {
			t.Errorf("expected custom avatar, got %s", res.Items[0].AuthorAvatarURL)
		}
		if res.Items[0].UserReaction != "FIRE" {
			t.Errorf("expected FIRE reaction, got %s", res.Items[0].UserReaction)
		}

		if res.Items[1].AuthorName != "" {
			t.Errorf("expected empty author name, got %s", res.Items[1].AuthorName)
		}
		if res.Items[1].AuthorAvatarURL != entity.DefaultAvatarURL {
			t.Errorf("expected DefaultAvatarURL, got %s", res.Items[1].AuthorAvatarURL)
		}
		if res.Items[1].UserReaction != "" {
			t.Errorf("expected empty reaction, got %s", res.Items[1].UserReaction)
		}

		if feedRepo.lastFilter.ViewerID != "u-viewer" || feedRepo.lastFilter.AuthorID != authorUUID1 {
			t.Errorf("filter mismatch: %+v", feedRepo.lastFilter)
		}
		if feedRepo.lastFilter.Query != "squat" || feedRepo.lastFilter.ItemType != "POST" || feedRepo.lastFilter.Limit != 10 {
			t.Errorf("filter mismatch: %+v", feedRepo.lastFilter)
		}
	})

	t.Run("returns ErrEmptySearchQuery when both query and author_id are empty", func(t *testing.T) {
		feedRepo := &searchTestMockFeedRepo{}
		handler := NewSearchPostsHandler(feedRepo, interactionRepo, userRepo)

		_, err := handler.Handle(context.Background(), SearchPostsQuery{
			Query:    "   ",
			AuthorID: "",
		})
		if !errors.Is(err, derror.ErrEmptySearchQuery) {
			t.Fatalf("expected ErrEmptySearchQuery, got %v", err)
		}
	})

	t.Run("returns ErrSearchQueryTooShort when query is less than 2 characters and author_id is empty", func(t *testing.T) {
		feedRepo := &searchTestMockFeedRepo{}
		handler := NewSearchPostsHandler(feedRepo, interactionRepo, userRepo)

		_, err := handler.Handle(context.Background(), SearchPostsQuery{
			Query:    "s",
			AuthorID: "",
		})
		if !errors.Is(err, derror.ErrSearchQueryTooShort) {
			t.Fatalf("expected ErrSearchQueryTooShort, got %v", err)
		}
	})

	t.Run("returns ErrInvalidUserID when author_id is not a valid UUID", func(t *testing.T) {
		feedRepo := &searchTestMockFeedRepo{}
		handler := NewSearchPostsHandler(feedRepo, interactionRepo, userRepo)

		_, err := handler.Handle(context.Background(), SearchPostsQuery{
			Query:    "squat",
			AuthorID: "invalid-uuid-string",
		})
		if !errors.Is(err, derror.ErrInvalidUserID) {
			t.Fatalf("expected ErrInvalidUserID, got %v", err)
		}
	})

	t.Run("default page size clamps correctly", func(t *testing.T) {
		feedRepo := &searchTestMockFeedRepo{}
		handler := NewSearchPostsHandler(feedRepo, interactionRepo, userRepo)

		_, _ = handler.Handle(context.Background(), SearchPostsQuery{Query: "test", PageSize: 0})
		if feedRepo.lastFilter.Limit != 20 {
			t.Errorf("expected limit = 20, got %d", feedRepo.lastFilter.Limit)
		}

		_, _ = handler.Handle(context.Background(), SearchPostsQuery{Query: "test", PageSize: 150})
		if feedRepo.lastFilter.Limit != 100 {
			t.Errorf("expected limit = 100, got %d", feedRepo.lastFilter.Limit)
		}
	})

	t.Run("returns wrapped error on repository failure", func(t *testing.T) {
		feedRepo := &searchTestMockFeedRepo{
			err: errors.New("db connection failure"),
		}
		handler := NewSearchPostsHandler(feedRepo, interactionRepo, userRepo)

		_, err := handler.Handle(context.Background(), SearchPostsQuery{Query: "bench"})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("returns error when batch reactions fetch fails", func(t *testing.T) {
		feedRepo := &searchTestMockFeedRepo{
			items: []*aggregate.FeedItem{item1},
		}
		failingInteractionRepo := &searchTestMockInteractionRepo{
			err: errors.New("interaction db error"),
		}
		handler := NewSearchPostsHandler(feedRepo, failingInteractionRepo, userRepo)

		_, err := handler.Handle(context.Background(), SearchPostsQuery{
			ViewerID: "u-viewer",
			Query:    "squat",
		})
		if err == nil {
			t.Fatalf("expected error on batch reactions failure, got nil")
		}
	})
}
