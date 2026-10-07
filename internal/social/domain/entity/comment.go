package entity

import (
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/derror"
)

type Comment struct {
	id         string
	userID     string
	feedItemID string
	parentID   *string
	content    string
	createdAt  time.Time
	updatedAt  time.Time
}

func NewComment(id, userID, feedItemID string, parentID *string, content string, createdAt, updatedAt time.Time) (*Comment, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, derror.ErrUnauthorized
	}
	if strings.TrimSpace(feedItemID) == "" {
		return nil, derror.ErrPostNotFound
	}
	if strings.TrimSpace(content) == "" {
		return nil, derror.ErrEmptyContent
	}

	now := time.Now().UTC()
	if createdAt.IsZero() {
		createdAt = now
	}
	if updatedAt.IsZero() {
		updatedAt = now
	}
	return &Comment{
		id:         id,
		userID:     userID,
		feedItemID: feedItemID,
		parentID:   parentID,
		content:    content,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}, nil
}

func (c *Comment) ID() string           { return c.id }
func (c *Comment) UserID() string       { return c.userID }
func (c *Comment) FeedItemID() string   { return c.feedItemID }
func (c *Comment) ParentID() *string    { return c.parentID }
func (c *Comment) Content() string      { return c.content }
func (c *Comment) CreatedAt() time.Time { return c.createdAt }
func (c *Comment) UpdatedAt() time.Time { return c.updatedAt }
