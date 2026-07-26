package domain

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidUser      = errors.New("user_id cannot be empty")
	ErrInvalidDates     = errors.New("start_date must be before end_date")
	ErrRoadmapNotActive = errors.New("roadmap is not active")
)

type RoadmapStatus int32

const (
	RoadmapStatusUnspecified RoadmapStatus = 0
	RoadmapStatusActive      RoadmapStatus = 1
	RoadmapStatusCompleted   RoadmapStatus = 2
)

// WorkoutRoadmap represents a 4-week high-level training plan strategy Aggregate Root.
type WorkoutRoadmap struct {
	id        string
	userID    string
	status    RoadmapStatus
	startDate time.Time
	endDate   time.Time
	createdAt time.Time
	updatedAt time.Time
}

func NewWorkoutRoadmap(id string, userID string, startDate time.Time, endDate time.Time) (*WorkoutRoadmap, error) {
	if userID == "" {
		return nil, ErrInvalidUser
	}
	if startDate.IsZero() || endDate.IsZero() || !startDate.Before(endDate) {
		return nil, ErrInvalidDates
	}
	if id == "" {
		id = fmt.Sprintf("rdp_%d", time.Now().UnixNano())
	}

	now := time.Now().UTC()
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

func ReconstituteWorkoutRoadmap(id, userID string, status RoadmapStatus, startDate, endDate, createdAt, updatedAt time.Time) *WorkoutRoadmap {
	return &WorkoutRoadmap{
		id:        id,
		userID:    userID,
		status:    status,
		startDate: startDate,
		endDate:   endDate,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

func (r *WorkoutRoadmap) ID() string             { return r.id }
func (r *WorkoutRoadmap) UserID() string         { return r.userID }
func (r *WorkoutRoadmap) Status() RoadmapStatus  { return r.status }
func (r *WorkoutRoadmap) StartDate() time.Time   { return r.startDate }
func (r *WorkoutRoadmap) EndDate() time.Time     { return r.endDate }
func (r *WorkoutRoadmap) CreatedAt() time.Time   { return r.createdAt }
func (r *WorkoutRoadmap) UpdatedAt() time.Time   { return r.updatedAt }

func (r *WorkoutRoadmap) IsActive() bool {
	return r.status == RoadmapStatusActive
}

func (r *WorkoutRoadmap) Complete() error {
	if !r.IsActive() {
		return ErrRoadmapNotActive
	}
	r.status = RoadmapStatusCompleted
	r.updatedAt = time.Now().UTC()
	return nil
}
