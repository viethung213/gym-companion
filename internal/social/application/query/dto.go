package query

import (
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type FeedItemDTO struct {
	ID              string
	UserID          string
	AuthorName      string
	AuthorAvatarURL string
	ItemType        string // "POST" | "WORKOUT_ACTIVITY"
	Caption         string
	MediaURLs       []string
	Visibility      string
	ReactionCount   int32
	CommentCount    int32
	UserReaction    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	WorkoutData     vo.WorkoutMetrics
}

type CommentItemDTO struct {
	ID              string
	UserID          string
	AuthorName      string
	AuthorAvatarURL string
	FeedItemID      string
	ParentID        *string
	Content         string
	CreatedAt       time.Time
}

type SocialSummaryDTO struct {
	UserID         string
	FollowerCount  int32
	FollowingCount int32
	PostCount      int32
	IsFollowing    bool
}

type UserSocialSummaryDTO struct {
	UserID      string
	FullName    string
	AvatarURL   string
	IsFollowing bool
}

type ReactionItemDTO struct {
	UserID          string
	AuthorName      string
	AuthorAvatarURL string
	ReactionType    string
	CreatedAt       time.Time
}
