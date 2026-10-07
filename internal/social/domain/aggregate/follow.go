package aggregate

import (
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/derror"
)

type Follow struct {
	id          string
	followerID  string
	followingID string
	createdAt   time.Time
}

func NewFollow(id, followerID, followingID string, createdAt time.Time) (*Follow, error) {
	if strings.TrimSpace(followerID) == "" || strings.TrimSpace(followingID) == "" {
		return nil, derror.ErrUnauthorized
	}
	if followerID == followingID {
		return nil, derror.ErrSelfFollow
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	return &Follow{
		id:          id,
		followerID:  followerID,
		followingID: followingID,
		createdAt:   createdAt,
	}, nil
}

func (f *Follow) ID() string           { return f.id }
func (f *Follow) FollowerID() string   { return f.followerID }
func (f *Follow) FollowingID() string  { return f.followingID }
func (f *Follow) CreatedAt() time.Time { return f.createdAt }
