// Package domain contains the Exercise aggregate and business rules.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	StatusDraft           Status = "DRAFT"
	StatusPendingApproval Status = "PENDING_APPROVAL"
	StatusActive          Status = "ACTIVE"
	StatusArchived        Status = "ARCHIVED"
)

var (
	ErrInvalidExercise      = errors.New("invalid exercise")
	ErrInvalidStatus        = errors.New("invalid exercise status")
	ErrInvalidTransition    = errors.New("invalid exercise status transition")
	ErrArchivedExercise     = errors.New("archived exercise cannot be changed")
	ErrExerciseNotFound     = errors.New("exercise not found")
	ErrUnauthorized         = errors.New("authentication required")
	ErrForbidden            = errors.New("admin role required")
	ErrInvalidOutboxPayload = errors.New("invalid outbox payload")
)

type Info struct {
	ID                 string
	Name               string
	BodyPartID         string
	EquipmentID        string
	TargetMuscleID     string
	Instructions       string
	SecondaryMuscleIDs []string
	ThumbnailURL       string
	MediaURL           string
	VideoURL           string
	Difficulty         string
	DefaultRestSeconds int32
	TagIDs             []string
	Status             Status
	ArchivedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Exercise struct {
	info Info
}

func NewExercise(info Info, now time.Time) (*Exercise, error) {
	info.ID = strings.TrimSpace(info.ID)
	info.Status = StatusDraft
	info.ArchivedAt = nil
	info.CreatedAt = now
	info.UpdatedAt = now

	exercise := &Exercise{info: normalizeInfo(info)}
	if err := exercise.validateRequired(); err != nil {
		return nil, err
	}

	return exercise, nil
}

func RehydrateExercise(info Info) (*Exercise, error) {
	info = normalizeInfo(info)
	if !info.Status.Valid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStatus, info.Status)
	}

	exercise := &Exercise{info: info}
	if err := exercise.validateRequired(); err != nil {
		return nil, err
	}

	return exercise, nil
}

func (e *Exercise) Info() Info {
	info := e.info
	info.SecondaryMuscleIDs = copyStrings(e.info.SecondaryMuscleIDs)
	info.TagIDs = copyStrings(e.info.TagIDs)

	return info
}

func (e *Exercise) UpdateInfo(info Info, now time.Time) error {
	if e.info.Status == StatusArchived {
		return ErrArchivedExercise
	}

	current := e.info
	current.Name = info.Name
	current.BodyPartID = info.BodyPartID
	current.EquipmentID = info.EquipmentID
	current.TargetMuscleID = info.TargetMuscleID
	current.Instructions = info.Instructions
	current.SecondaryMuscleIDs = copyStrings(info.SecondaryMuscleIDs)
	current.ThumbnailURL = info.ThumbnailURL
	current.MediaURL = info.MediaURL
	current.VideoURL = info.VideoURL
	current.Difficulty = info.Difficulty
	current.DefaultRestSeconds = info.DefaultRestSeconds
	current.TagIDs = copyStrings(info.TagIDs)
	current.UpdatedAt = now

	current = normalizeInfo(current)
	updated := &Exercise{info: current}
	if err := updated.validateRequired(); err != nil {
		return err
	}

	e.info = current
	return nil
}

func (e *Exercise) SubmitForApproval(now time.Time) error {
	if e.info.Status != StatusDraft {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, e.info.Status, StatusPendingApproval)
	}

	e.info.Status = StatusPendingApproval
	e.info.UpdatedAt = now

	return nil
}

func (e *Exercise) Approve(now time.Time) error {
	if e.info.Status != StatusPendingApproval {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, e.info.Status, StatusActive)
	}

	e.info.Status = StatusActive
	e.info.UpdatedAt = now

	return nil
}

func (e *Exercise) Archive(now time.Time) error {
	if e.info.Status == StatusArchived {
		return nil
	}

	e.info.Status = StatusArchived
	e.info.ArchivedAt = &now
	e.info.UpdatedAt = now

	return nil
}

func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusPendingApproval, StatusActive, StatusArchived:
		return true
	default:
		return false
	}
}

func (e *Exercise) validateRequired() error {
	info := e.info
	if info.ID == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidExercise)
	}
	if info.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidExercise)
	}
	if info.BodyPartID == "" {
		return fmt.Errorf("%w: body part id is required", ErrInvalidExercise)
	}
	if info.EquipmentID == "" {
		return fmt.Errorf("%w: equipment id is required", ErrInvalidExercise)
	}
	if info.TargetMuscleID == "" {
		return fmt.Errorf("%w: target muscle id is required", ErrInvalidExercise)
	}
	if !info.Status.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidStatus, info.Status)
	}

	return nil
}

func normalizeInfo(info Info) Info {
	info.ID = strings.TrimSpace(info.ID)
	info.Name = strings.TrimSpace(info.Name)
	info.BodyPartID = strings.TrimSpace(info.BodyPartID)
	info.EquipmentID = strings.TrimSpace(info.EquipmentID)
	info.TargetMuscleID = strings.TrimSpace(info.TargetMuscleID)
	info.Instructions = strings.TrimSpace(info.Instructions)
	info.ThumbnailURL = strings.TrimSpace(info.ThumbnailURL)
	info.MediaURL = strings.TrimSpace(info.MediaURL)
	info.VideoURL = strings.TrimSpace(info.VideoURL)
	info.Difficulty = strings.TrimSpace(info.Difficulty)
	if info.Difficulty == "" {
		info.Difficulty = "Beginner"
	}
	if info.DefaultRestSeconds == 0 {
		info.DefaultRestSeconds = 60
	}
	info.SecondaryMuscleIDs = copyStrings(info.SecondaryMuscleIDs)
	info.TagIDs = copyStrings(info.TagIDs)

	return info
}

func copyStrings(values []string) []string {
	if values == nil {
		return nil
	}

	copied := make([]string, len(values))
	copy(copied, values)

	return copied
}
