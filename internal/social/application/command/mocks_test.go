package command

import (
	"context"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

type mockTxManager struct{}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type mockEventPublisher struct {
	published []any
}

func (m *mockEventPublisher) PublishEvents(ctx context.Context, events []any) error {
	m.published = append(m.published, events...)
	return nil
}

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

type mockFollowRepo struct {
	follows map[string]*aggregate.Follow
}

func (m *mockFollowRepo) Follow(ctx context.Context, follow *aggregate.Follow) error {
	m.follows[follow.FollowerID()+":"+follow.FollowingID()] = follow
	return nil
}

func (m *mockFollowRepo) Unfollow(ctx context.Context, followerID, followingID string) error {
	delete(m.follows, followerID+":"+followingID)
	return nil
}

func (m *mockFollowRepo) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	_, ok := m.follows[followerID+":"+followingID]
	return ok, nil
}

func (m *mockFollowRepo) GetFollowers(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.Follow, string, int32, error) {
	var list []*aggregate.Follow
	for _, f := range m.follows {
		if f.FollowingID() == userID {
			list = append(list, f)
		}
	}
	return list, "", int32(len(list)), nil
}

func (m *mockFollowRepo) GetFollowing(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.Follow, string, int32, error) {
	return nil, "", 0, nil
}

func (m *mockFollowRepo) GetFollowingIDs(ctx context.Context, followerID string) ([]string, error) {
	var ids []string
	for _, f := range m.follows {
		if f.FollowerID() == followerID {
			ids = append(ids, f.FollowingID())
		}
	}
	return ids, nil
}

func (m *mockFollowRepo) CountFollowers(ctx context.Context, userID string) (int32, error) {
	return 0, nil
}

func (m *mockFollowRepo) CountFollowing(ctx context.Context, userID string) (int32, error) {
	return 0, nil
}

type mockFeedItemRepo struct {
	items map[string]*aggregate.FeedItem
}

func (m *mockFeedItemRepo) Create(ctx context.Context, item *aggregate.FeedItem) error {
	m.items[item.ID()] = item
	return nil
}

func (m *mockFeedItemRepo) GetByID(ctx context.Context, id string) (*aggregate.FeedItem, error) {
	return m.items[id], nil
}

func (m *mockFeedItemRepo) Delete(ctx context.Context, id, userID string) error {
	delete(m.items, id)
	return nil
}

func (m *mockFeedItemRepo) GetByUserID(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	return nil, "", nil
}

func (m *mockFeedItemRepo) GetByAuthorIDs(ctx context.Context, authorIDs []string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	return nil, "", nil
}

func (m *mockFeedItemRepo) CountByUserID(ctx context.Context, userID string) (int32, error) {
	return int32(len(m.items)), nil
}

func (m *mockFeedItemRepo) UpdateReactionCount(ctx context.Context, id string, delta int) error {
	if item, ok := m.items[id]; ok {
		if delta > 0 {
			item.IncrementReaction()
		} else {
			item.DecrementReaction()
		}
	}
	return nil
}

func (m *mockFeedItemRepo) UpdateCommentCount(ctx context.Context, id string, delta int) error {
	if item, ok := m.items[id]; ok {
		if delta > 0 {
			item.IncrementComment()
		} else {
			item.DecrementComment()
		}
	}
	return nil
}

type mockInteractionRepo struct {
	reactions map[string]*entity.Reaction
	comments  map[string]*entity.Comment
}

func (m *mockInteractionRepo) SaveReaction(ctx context.Context, reaction *entity.Reaction) error {
	key := reaction.UserID() + ":" + reaction.FeedItemID()
	m.reactions[key] = reaction
	return nil
}

func (m *mockInteractionRepo) DeleteReaction(ctx context.Context, userID, feedItemID string) error {
	key := userID + ":" + feedItemID
	delete(m.reactions, key)
	return nil
}

func (m *mockInteractionRepo) GetReaction(ctx context.Context, userID, feedItemID string) (*entity.Reaction, error) {
	key := userID + ":" + feedItemID
	return m.reactions[key], nil
}

func (m *mockInteractionRepo) GetReactionsByFeedItem(ctx context.Context, feedItemID string) ([]*entity.Reaction, error) {
	return nil, nil
}

func (m *mockInteractionRepo) AddComment(ctx context.Context, comment *entity.Comment) error {
	m.comments[comment.ID()] = comment
	return nil
}

func (m *mockInteractionRepo) GetCommentByID(ctx context.Context, id string) (*entity.Comment, error) {
	return m.comments[id], nil
}

func (m *mockInteractionRepo) DeleteComment(ctx context.Context, id, userID string) error {
	delete(m.comments, id)
	return nil
}

func (m *mockInteractionRepo) ListComments(ctx context.Context, feedItemID string, limit int, cursor string) ([]*entity.Comment, string, int32, error) {
	return nil, "", 0, nil
}
