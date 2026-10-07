package entity

import "time"

type UserSnapshot struct {
	id        string
	fullName  string
	avatarURL string
	role      string
	updatedAt time.Time
}

func NewUserSnapshot(id, fullName, avatarURL, role string, updatedAt time.Time) *UserSnapshot {
	if role == "" {
		role = "user"
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	return &UserSnapshot{
		id:        id,
		fullName:  fullName,
		avatarURL: avatarURL,
		role:      role,
		updatedAt: updatedAt,
	}
}

func (u *UserSnapshot) ID() string           { return u.id }
func (u *UserSnapshot) FullName() string     { return u.fullName }
func (u *UserSnapshot) AvatarURL() string    { return u.avatarURL }
func (u *UserSnapshot) Role() string         { return u.role }
func (u *UserSnapshot) UpdatedAt() time.Time { return u.updatedAt }

func (u *UserSnapshot) UpdateRole(role string) {
	if role != "" {
		u.role = role
		u.updatedAt = time.Now().UTC()
	}
}
