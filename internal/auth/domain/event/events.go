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
	Email        string
	FullName     string
	RegisteredAt time.Time
}

// OccurredAt returns the time the event happened.
func (e UserRegisteredEvent) OccurredAt() time.Time {
	return e.RegisteredAt
}

// EventName returns the name identifier of the event.
func (e UserRegisteredEvent) EventName() string {
	return "UserRegistered"
}
