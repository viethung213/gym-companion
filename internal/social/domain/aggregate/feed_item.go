package aggregate

import (
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type FeedItem struct {
	id            string
	userID        string
	itemType      vo.ItemType
	caption       string
	mediaURLs     []string
	workoutData   vo.WorkoutMetrics
	visibility    vo.Visibility
	reactionCount int32
	commentCount  int32
	createdAt     time.Time
	updatedAt     time.Time
}

func NewPostItem(
	id string,
	userID string,
	caption string,
	mediaURLs []string,
	visibility vo.Visibility,
	reactionCount int32,
	commentCount int32,
	createdAt time.Time,
	updatedAt time.Time,
) (*FeedItem, error) {
	if strings.TrimSpace(caption) == "" && len(mediaURLs) == 0 {
		return nil, derror.ErrEmptyContent
	}
	if strings.TrimSpace(userID) == "" {
		return nil, derror.ErrUnauthorized
	}

	now := time.Now().UTC()
	if createdAt.IsZero() {
		createdAt = now
	}
	if updatedAt.IsZero() {
		updatedAt = now
	}

	copiedURLs := make([]string, len(mediaURLs))
	copy(copiedURLs, mediaURLs)

	return &FeedItem{
		id:            id,
		userID:        userID,
		itemType:      vo.ItemTypePost,
		caption:       caption,
		mediaURLs:     copiedURLs,
		visibility:    visibility,
		reactionCount: reactionCount,
		commentCount:  commentCount,
		createdAt:     createdAt,
		updatedAt:     updatedAt,
	}, nil
}

func NewWorkoutActivityItem(
	id string,
	userID string,
	caption string,
	mediaURLs []string,
	workoutData vo.WorkoutMetrics,
	visibility vo.Visibility,
	reactionCount int32,
	commentCount int32,
	createdAt time.Time,
	updatedAt time.Time,
) (*FeedItem, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, derror.ErrUnauthorized
	}
	if strings.TrimSpace(caption) == "" && len(mediaURLs) == 0 && workoutData.IsZero() {
		return nil, derror.ErrEmptyContent
	}

	now := time.Now().UTC()
	if createdAt.IsZero() {
		createdAt = now
	}
	if updatedAt.IsZero() {
		updatedAt = now
	}

	copiedURLs := make([]string, len(mediaURLs))
	copy(copiedURLs, mediaURLs)

	return &FeedItem{
		id:            id,
		userID:        userID,
		itemType:      vo.ItemTypeWorkoutActivity,
		caption:       caption,
		mediaURLs:     copiedURLs,
		workoutData:   workoutData,
		visibility:    visibility,
		reactionCount: reactionCount,
		commentCount:  commentCount,
		createdAt:     createdAt,
		updatedAt:     updatedAt,
	}, nil
}

func (f *FeedItem) ID() string            { return f.id }
func (f *FeedItem) UserID() string        { return f.userID }
func (f *FeedItem) ItemType() vo.ItemType { return f.itemType }
func (f *FeedItem) Caption() string       { return f.caption }
func (f *FeedItem) MediaURLs() []string {
	res := make([]string, len(f.mediaURLs))
	copy(res, f.mediaURLs)
	return res
}
func (f *FeedItem) WorkoutData() vo.WorkoutMetrics { return f.workoutData }
func (f *FeedItem) Visibility() vo.Visibility      { return f.visibility }
func (f *FeedItem) ReactionCount() int32           { return f.reactionCount }
func (f *FeedItem) CommentCount() int32            { return f.commentCount }
func (f *FeedItem) CreatedAt() time.Time           { return f.createdAt }
func (f *FeedItem) UpdatedAt() time.Time           { return f.updatedAt }

func (f *FeedItem) IncrementReaction() {
	f.reactionCount++
}

func (f *FeedItem) DecrementReaction() {
	if f.reactionCount > 0 {
		f.reactionCount--
	}
}

func (f *FeedItem) IncrementComment() {
	f.commentCount++
}

func (f *FeedItem) DecrementComment() {
	if f.commentCount > 0 {
		f.commentCount--
	}
}
