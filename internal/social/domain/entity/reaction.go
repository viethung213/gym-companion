package entity

import (
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type Reaction struct {
	id           string
	userID       string
	feedItemID   string
	reactionType vo.ReactionType
	createdAt    time.Time
}

func NewReaction(id, userID, feedItemID string, reactionType vo.ReactionType, createdAt time.Time) (*Reaction, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, derror.ErrUnauthorized
	}
	if strings.TrimSpace(feedItemID) == "" {
		return nil, derror.ErrPostNotFound
	}
	if strings.TrimSpace(string(reactionType)) == "" {
		return nil, derror.ErrInvalidReactionType
	}

	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	return &Reaction{
		id:           id,
		userID:       userID,
		feedItemID:   feedItemID,
		reactionType: reactionType,
		createdAt:    createdAt,
	}, nil
}

func (r *Reaction) ID() string                    { return r.id }
func (r *Reaction) UserID() string                { return r.userID }
func (r *Reaction) FeedItemID() string            { return r.feedItemID }
func (r *Reaction) ReactionType() vo.ReactionType { return r.reactionType }
func (r *Reaction) CreatedAt() time.Time          { return r.createdAt }

func (r *Reaction) ChangeReactionType(newType vo.ReactionType) {
	r.reactionType = newType
}
