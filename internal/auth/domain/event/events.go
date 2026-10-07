package event

import "time"

// DomainEvent defines the contract for all domain events.
type DomainEvent interface {
	OccurredAt() time.Time
	EventName() string
}

// UserRegisteredEvent is triggered when a new user successfully signs up.
type UserRegisteredEvent struct {
	UserID       string
	IdentityType string
	Identifier   string
	FullName     string
	Gender       string
	DateOfBirth  string
	AvatarURL    string
	Role         string
	RegisteredAt time.Time
}

// OccurredAt returns the time the event happened.
func (e UserRegisteredEvent) OccurredAt() time.Time {
	return e.RegisteredAt
}

// EventName returns the name identifier of the event.
func (e UserRegisteredEvent) EventName() string {
	return "contracts.generic.auth.v1.userRegistered"
}

// OTPSentEvent is triggered when a new OTP code is generated for verification.
type OTPSentEvent struct {
	Identifier       string
	Code             string
	ExpiresInSeconds int64
	SentAt           time.Time
}

// OccurredAt returns the time the event happened.
func (e OTPSentEvent) OccurredAt() time.Time {
	return e.SentAt
}

// EventName returns the name identifier of the event.
func (e OTPSentEvent) EventName() string {
	return "contracts.generic.notification.v1.event.HighPriorityNotificationRequested"
}

// UserRoleUpdatedEvent is triggered when a user's role is updated (e.g. approved as Brand).
type UserRoleUpdatedEvent struct {
	UserID    string
	OldRole   string
	NewRole   string
	UpdatedAt time.Time
}

// OccurredAt returns the time the event happened.
func (e UserRoleUpdatedEvent) OccurredAt() time.Time {
	return e.UpdatedAt
}

// EventName returns the name identifier of the event.
func (e UserRoleUpdatedEvent) EventName() string {
	return "contracts.generic.auth.v1.userRoleUpdated"
}
