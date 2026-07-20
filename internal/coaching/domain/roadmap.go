// Package domain contains the Coaching bounded context aggregates and business rules.
package domain

import (
	"errors"
	"fmt"
	"time"
)

// RoadmapStatus represents the lifecycle state of a WorkoutRoadmap.
type RoadmapStatus string

const (
	RoadmapStatusActive    RoadmapStatus = "ACTIVE"
	RoadmapStatusCompleted RoadmapStatus = "COMPLETED"
	RoadmapStatusPaused    RoadmapStatus = "PAUSED"
)

const roadmapDurationWeeks = 4

var (
	ErrInvalidRoadmap           = errors.New("invalid workout roadmap")
	ErrRoadmapAlreadyActive     = errors.New("user already has an active roadmap")
	ErrRoadmapInvalidStatus     = errors.New("invalid roadmap status")
	ErrRoadmapInvalidTransition = errors.New("invalid roadmap status transition")
)

// WorkoutRoadmap is the Aggregate Root for a training cycle strategy.
// It defines the overall 4-week plan duration and lifecycle.
type WorkoutRoadmap struct {
	id        string
	userID    string
	status    RoadmapStatus
	startDate time.Time
	endDate   time.Time
	createdAt time.Time
	updatedAt time.Time
}

// Initiate creates a new WorkoutRoadmap for a user starting from the given date.
func Initiate(id, userID string, startDate, now time.Time) (*WorkoutRoadmap, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidRoadmap)
	}
	if userID == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidRoadmap)
	}

	endDate := startDate.AddDate(0, 0, roadmapDurationWeeks*7-1)

	return &WorkoutRoadmap{
		id:        id,
		userID:    userID,
		status:    RoadmapStatusActive,
		startDate: startDate,
		endDate:   endDate,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// RehydrateRoadmap reconstructs a WorkoutRoadmap from persistence.
func RehydrateRoadmap(
	id, userID string,
	status RoadmapStatus,
	startDate, endDate time.Time,
	createdAt, updatedAt time.Time,
) (*WorkoutRoadmap, error) {
	if !status.Valid() {
		return nil, fmt.Errorf("%w: %s", ErrRoadmapInvalidStatus, status)
	}

	return &WorkoutRoadmap{
		id:        id,
		userID:    userID,
		status:    status,
		startDate: startDate,
		endDate:   endDate,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}, nil
}

func (r *WorkoutRoadmap) ID() string            { return r.id }
func (r *WorkoutRoadmap) UserID() string        { return r.userID }
func (r *WorkoutRoadmap) Status() RoadmapStatus { return r.status }
func (r *WorkoutRoadmap) StartDate() time.Time  { return r.startDate }
func (r *WorkoutRoadmap) EndDate() time.Time    { return r.endDate }
func (r *WorkoutRoadmap) CreatedAt() time.Time  { return r.createdAt }
func (r *WorkoutRoadmap) UpdatedAt() time.Time  { return r.updatedAt }

// Complete transitions the roadmap to COMPLETED status.
func (r *WorkoutRoadmap) Complete(now time.Time) error {
	if r.status != RoadmapStatusActive {
		return fmt.Errorf("%w: %s to %s", ErrRoadmapInvalidTransition, r.status, RoadmapStatusCompleted)
	}
	r.status = RoadmapStatusCompleted
	r.updatedAt = now
	return nil
}

// Pause transitions the roadmap to PAUSED status.
func (r *WorkoutRoadmap) Pause(now time.Time) error {
	if r.status != RoadmapStatusActive {
		return fmt.Errorf("%w: %s to %s", ErrRoadmapInvalidTransition, r.status, RoadmapStatusPaused)
	}
	r.status = RoadmapStatusPaused
	r.updatedAt = now
	return nil
}

// Resume transitions the roadmap back to ACTIVE from PAUSED.
func (r *WorkoutRoadmap) Resume(now time.Time) error {
	if r.status != RoadmapStatusPaused {
		return fmt.Errorf("%w: %s to %s", ErrRoadmapInvalidTransition, r.status, RoadmapStatusActive)
	}
	r.status = RoadmapStatusActive
	r.updatedAt = now
	return nil
}

func (s RoadmapStatus) Valid() bool {
	switch s {
	case RoadmapStatusActive, RoadmapStatusCompleted, RoadmapStatusPaused:
		return true
	default:
		return false
	}
}
