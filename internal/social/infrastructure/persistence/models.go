package persistence

import (
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type SocialUserModel struct {
	ID        string    `gorm:"primaryKey;column:id"`
	FullName  string    `gorm:"column:full_name"`
	AvatarURL string    `gorm:"column:avatar_url"`
	Role      string    `gorm:"column:role;default:user"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (SocialUserModel) TableName() string {
	return "social.users"
}

type FollowModel struct {
	ID          string    `gorm:"primaryKey;column:id"`
	FollowerID  string    `gorm:"column:follower_id"`
	FollowingID string    `gorm:"column:following_id"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (FollowModel) TableName() string {
	return "social.follows"
}

type FeedItemModel struct {
	ID            string    `gorm:"primaryKey;column:id"`
	UserID        string    `gorm:"column:user_id"`
	ItemType      string    `gorm:"column:item_type"`
	Caption       string    `gorm:"column:caption"`
	MediaURLs     []byte    `gorm:"column:media_urls;type:jsonb"`
	Data          []byte    `gorm:"column:data;type:jsonb"`
	Visibility    string    `gorm:"column:visibility"`
	ReactionCount int32     `gorm:"column:reaction_count"`
	CommentCount  int32     `gorm:"column:comment_count"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (FeedItemModel) TableName() string {
	return "social.feed_items"
}

type ReactionModel struct {
	ID           string          `gorm:"primaryKey;column:id"`
	UserID       string          `gorm:"column:user_id"`
	FeedItemID   string          `gorm:"column:feed_item_id"`
	ReactionType vo.ReactionType `gorm:"column:reaction_type;type:social.reaction_type"`
	CreatedAt    time.Time       `gorm:"column:created_at"`
}

func (ReactionModel) TableName() string {
	return "social.reactions"
}

type CommentModel struct {
	ID         string    `gorm:"primaryKey;column:id"`
	UserID     string    `gorm:"column:user_id"`
	FeedItemID string    `gorm:"column:feed_item_id"`
	ParentID   *string   `gorm:"column:parent_id"`
	Content    string    `gorm:"column:content"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (CommentModel) TableName() string {
	return "social.comments"
}

type OutboxModel struct {
	ID           string     `gorm:"primaryKey;column:id"`
	EventID      string     `gorm:"column:event_id"`
	EventType    string     `gorm:"column:event_type"`
	Payload      []byte     `gorm:"column:payload;type:jsonb"`
	PartitionKey string     `gorm:"column:partition_key"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	Published    bool       `gorm:"column:published"`
	PublishedAt  *time.Time `gorm:"column:published_at"`
	Status       string     `gorm:"column:status"`
	LockedUntil  *time.Time `gorm:"column:locked_until"`
}

func (OutboxModel) TableName() string {
	return "social.outbox"
}

type OutboxLogModel struct {
	ID           string    `gorm:"primaryKey;column:id"`
	EventID      string    `gorm:"column:event_id"`
	EventType    string    `gorm:"column:event_type"`
	Payload      []byte    `gorm:"column:payload;type:jsonb"`
	PartitionKey string    `gorm:"column:partition_key"`
	ProcessedAt  time.Time `gorm:"column:processed_at"`
	Status       string    `gorm:"column:status"`
	ErrorMessage string    `gorm:"column:error_message"`
}

func (OutboxLogModel) TableName() string {
	return "social.outbox_log"
}
