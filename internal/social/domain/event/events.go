package event

import (
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type UserFollowedEvent struct {
	FollowerID        string
	FollowingID       string
	FollowedAt        time.Time
	FollowerName      string
	FollowerAvatarURL string
}

type PostCreatedEvent struct {
	PostID          string
	UserID          string
	UserName        string
	UserAvatarURL   string
	Content         string
	MediaURLs       []string
	Visibility      string
	FollowerUserIDs []string
	CreatedAt       time.Time
}

type PostReactedEvent struct {
	FeedItemID    string
	PostOwnerID   string
	UserID        string
	UserName      string
	UserAvatarURL string
	ReactionType  vo.ReactionType
	ReactedAt     time.Time
}

type PostCommentedEvent struct {
	CommentID            string
	FeedItemID           string
	PostOwnerID          string
	ParentCommentOwnerID *string
	UserID               string
	UserName             string
	UserAvatarURL        string
	Content              string
	ParentID             *string
	CommentedAt          time.Time
}
