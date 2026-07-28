// Package event contains value-object domain events emitted by the Coaching
// aggregate. Each event carries an EventName() matching the CloudEvent type
// used across the platform.
package event

import "time"

// Event is the marker interface all coaching domain events satisfy.
type Event interface {
	EventName() string
}

// RoadmapInitiated is published after a fresh 4-week roadmap is stored.
type RoadmapInitiated struct {
	RoadmapID   string    `json:"roadmapId"`
	UserID      string    `json:"userId"`
	InitiatedAt time.Time `json:"initiatedAt"`
}

// EventName returns the CloudEvent type identifier.
func (e *RoadmapInitiated) EventName() string {
	return "contracts.core.coaching.v1.event.RoadmapInitiated"
}

// RoadmapAdjusted is published after a roadmap mutation (Regenerate, adaptive
// cycle, signal handling, injury recovery).
type RoadmapAdjusted struct {
	RoadmapID  string    `json:"roadmapId"`
	UserID     string    `json:"userId"`
	Reason     string    `json:"reason"`
	AdjustedAt time.Time `json:"adjustedAt"`
}

// EventName returns the CloudEvent type identifier.
func (e *RoadmapAdjusted) EventName() string {
	return "contracts.core.coaching.v1.event.RoadmapAdjusted"
}

// SessionPlanExecuted is published after a SessionPlan transitions to COMPLETED
// and SCR / ΔRPE have been computed.
type SessionPlanExecuted struct {
	SessionPlanID   string    `json:"sessionPlanId"`
	RoadmapID       string    `json:"roadmapId"`
	UserID          string    `json:"userId"`
	ExecutedAt      time.Time `json:"executedAt"`
	SessionSCR      float32   `json:"sessionScr"`
	SessionDeltaRPE float32   `json:"sessionDeltaRpe"`
}

// EventName returns the CloudEvent type identifier.
func (e *SessionPlanExecuted) EventName() string {
	return "contracts.core.coaching.v1.event.SessionPlanExecuted"
}
