package entity

import (
	"strings"
	"time"
)

// DefaultAvatarURL is the fallback avatar image URL used when user snapshot omits an avatar.
const DefaultAvatarURL = "https://lqhhwimtzrcrmyfhlizv.supabase.co/storage/v1/object/public/" +
	"gym-companion-assets/avatar_default/male/avatar-vo-tri-loopy-cute.jpg"

type UserSnapshot struct {
	id             string
	fullName       string
	avatarURL      string
	role           string
	followerCount  int32
	followingCount int32
	updatedAt      time.Time
}

func NewUserSnapshot(id, fullName, avatarURL, role string, updatedAt time.Time) *UserSnapshot {
	return NewUserSnapshotWithCounts(id, fullName, avatarURL, role, 0, 0, updatedAt)
}

func NewUserSnapshotWithCounts(
	id, fullName, avatarURL, role string,
	followerCount, followingCount int32,
	updatedAt time.Time,
) *UserSnapshot {
	if role == "" {
		role = "user"
	}
	if strings.TrimSpace(avatarURL) == "" {
		avatarURL = DefaultAvatarURL
	}
	if followerCount < 0 {
		followerCount = 0
	}
	if followingCount < 0 {
		followingCount = 0
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	return &UserSnapshot{
		id:             id,
		fullName:       fullName,
		avatarURL:      avatarURL,
		role:           role,
		followerCount:  followerCount,
		followingCount: followingCount,
		updatedAt:      updatedAt,
	}
}

func (u *UserSnapshot) ID() string            { return u.id }
func (u *UserSnapshot) FullName() string      { return u.fullName }
func (u *UserSnapshot) AvatarURL() string     { return u.avatarURL }
func (u *UserSnapshot) Role() string          { return u.role }
func (u *UserSnapshot) FollowerCount() int32  { return u.followerCount }
func (u *UserSnapshot) FollowingCount() int32 { return u.followingCount }
func (u *UserSnapshot) UpdatedAt() time.Time  { return u.updatedAt }

func (u *UserSnapshot) UpdateRole(role string) {
	if role != "" {
		u.role = role
		u.updatedAt = time.Now().UTC()
	}
}

func (u *UserSnapshot) UpdateCounts(followerCount, followingCount int32) {
	if followerCount < 0 {
		followerCount = 0
	}
	if followingCount < 0 {
		followingCount = 0
	}
	u.followerCount = followerCount
	u.followingCount = followingCount
	u.updatedAt = time.Now().UTC()
}
