package event

import "time"

type UserIdentityUpdatedEvent struct {
	userID    string
	fullName  string
	avatarURL string
	updatedAt time.Time
}

func NewUserIdentityUpdatedEvent(userID, fullName, avatarURL string, updatedAt time.Time) *UserIdentityUpdatedEvent {
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	return &UserIdentityUpdatedEvent{
		userID:    userID,
		fullName:  fullName,
		avatarURL: avatarURL,
		updatedAt: updatedAt,
	}
}

func (e *UserIdentityUpdatedEvent) UserID() string       { return e.userID }
func (e *UserIdentityUpdatedEvent) FullName() string     { return e.fullName }
func (e *UserIdentityUpdatedEvent) AvatarURL() string    { return e.avatarURL }
func (e *UserIdentityUpdatedEvent) UpdatedAt() time.Time { return e.updatedAt }
