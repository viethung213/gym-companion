package query

import (
	"context"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

type mockUserSnapshotRepo struct {
	users map[string]*entity.UserSnapshot
}

func (m *mockUserSnapshotRepo) GetByID(ctx context.Context, id string) (*entity.UserSnapshot, error) {
	if m.users != nil {
		if u, ok := m.users[id]; ok {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserSnapshotRepo) GetByIDs(ctx context.Context, ids []string) (map[string]*entity.UserSnapshot, error) {
	res := make(map[string]*entity.UserSnapshot)
	for _, id := range ids {
		if u, ok := m.users[id]; ok {
			res[id] = u
		}
	}
	return res, nil
}

func (m *mockUserSnapshotRepo) Upsert(ctx context.Context, user *entity.UserSnapshot) error {
	if m.users == nil {
		m.users = make(map[string]*entity.UserSnapshot)
	}
	m.users[user.ID()] = user
	return nil
}

func (m *mockUserSnapshotRepo) UpdateRole(ctx context.Context, userID, newRole string) error {
	if m.users != nil {
		if u, ok := m.users[userID]; ok {
			u.UpdateRole(newRole)
		}
	}
	return nil
}

func (m *mockUserSnapshotRepo) SearchUsers(ctx context.Context, query string, role string, limit int, cursor string) ([]*entity.UserSnapshot, string, int32, error) {
	list := make([]*entity.UserSnapshot, 0, len(m.users))
	for _, u := range m.users {
		if u.Role() == "admin" {
			continue
		}
		if role != "" && u.Role() != role {
			continue
		}
		list = append(list, u)
	}
	return list, "", int32(len(list)), nil
}

type queryMockFollowRepo struct {
	followingIDs []string
	followers    []*aggregate.Follow
	following    []*aggregate.Follow
}

func (m *queryMockFollowRepo) Follow(ctx context.Context, follow *aggregate.Follow) error { return nil }
func (m *queryMockFollowRepo) Unfollow(ctx context.Context, followerID, followingID string) error {
	return nil
}
func (m *queryMockFollowRepo) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	return true, nil
}
func (m *queryMockFollowRepo) GetFollowers(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.Follow, string, int32, error) {
	if m.followers != nil {
		return m.followers, "", int32(len(m.followers)), nil
	}
	return nil, "", 0, nil
}
func (m *queryMockFollowRepo) GetFollowing(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.Follow, string, int32, error) {
	if m.following != nil {
		return m.following, "", int32(len(m.following)), nil
	}
	return nil, "", 0, nil
}
func (m *queryMockFollowRepo) GetFollowingIDs(ctx context.Context, followerID string) ([]string, error) {
	return m.followingIDs, nil
}
func (m *queryMockFollowRepo) CountFollowers(ctx context.Context, userID string) (int32, error) {
	return 10, nil
}
func (m *queryMockFollowRepo) CountFollowing(ctx context.Context, userID string) (int32, error) {
	return 5, nil
}

type queryMockFeedItemRepo struct {
	items []*aggregate.FeedItem
}

func (m *queryMockFeedItemRepo) Create(ctx context.Context, item *aggregate.FeedItem) error {
	return nil
}
func (m *queryMockFeedItemRepo) GetByID(ctx context.Context, id string) (*aggregate.FeedItem, error) {
	return nil, nil
}
func (m *queryMockFeedItemRepo) Delete(ctx context.Context, id, userID string) error { return nil }
func (m *queryMockFeedItemRepo) GetByUserID(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	return m.items, "", nil
}
func (m *queryMockFeedItemRepo) GetByAuthorIDs(ctx context.Context, authorIDs []string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	return m.items, "", nil
}
func (m *queryMockFeedItemRepo) CountByUserID(ctx context.Context, userID string) (int32, error) {
	return int32(len(m.items)), nil
}
func (m *queryMockFeedItemRepo) UpdateReactionCount(ctx context.Context, id string, delta int) error {
	return nil
}
func (m *queryMockFeedItemRepo) UpdateCommentCount(ctx context.Context, id string, delta int) error {
	return nil
}

type queryMockInteractionRepo struct {
	comments  []*entity.Comment
	reactions []*entity.Reaction
}

func (m *queryMockInteractionRepo) SaveReaction(ctx context.Context, reaction *entity.Reaction) error {
	return nil
}
func (m *queryMockInteractionRepo) DeleteReaction(ctx context.Context, userID, feedItemID string) error {
	return nil
}
func (m *queryMockInteractionRepo) GetReaction(ctx context.Context, userID, feedItemID string) (*entity.Reaction, error) {
	return nil, nil
}
func (m *queryMockInteractionRepo) GetReactionsByFeedItem(ctx context.Context, feedItemID string) ([]*entity.Reaction, error) {
	return m.reactions, nil
}
func (m *queryMockInteractionRepo) AddComment(ctx context.Context, comment *entity.Comment) error {
	return nil
}
func (m *queryMockInteractionRepo) GetCommentByID(ctx context.Context, id string) (*entity.Comment, error) {
	return nil, nil
}
func (m *queryMockInteractionRepo) DeleteComment(ctx context.Context, id, userID string) error {
	return nil
}
func (m *queryMockInteractionRepo) ListComments(ctx context.Context, feedItemID string, limit int, cursor string) ([]*entity.Comment, string, int32, error) {
	return m.comments, "", int32(len(m.comments)), nil
}
