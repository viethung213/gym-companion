// Package domain contains the Exercise aggregate and business rules.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Status is an exercise's lifecycle state:
//
//	DRAFT --Approve--> ACTIVE --Archive--> ARCHIVED --Restore--> ACTIVE
//
// Only ACTIVE exercises are visible to athletes and the coach; DRAFT is an
// exercise awaiting an admin's approval.
type Status string

const (
	StatusDraft    Status = "DRAFT"
	StatusActive   Status = "ACTIVE"
	StatusArchived Status = "ARCHIVED"
)

type MeasurementType string

const (
	MeasurementTypeReps     MeasurementType = "REPS"
	MeasurementTypeDuration MeasurementType = "DURATION"
)

func (m MeasurementType) Valid() bool {
	switch m {
	case MeasurementTypeReps, MeasurementTypeDuration:
		return true
	default:
		return false
	}
}

// Source tells where a catalog exercise comes from.
type Source string

const (
	SourceFreeExerciseDB Source = "free-exercise-db"
	SourceCustom         Source = "custom"
)

func (s Source) Valid() bool {
	return s == SourceFreeExerciseDB || s == SourceCustom
}

const defaultRestSeconds = 60

var (
	ErrInvalidExercise      = errors.New("invalid exercise")
	ErrInvalidStatus        = errors.New("invalid exercise status")
	ErrInvalidTransition    = errors.New("invalid exercise status transition")
	ErrArchivedExercise     = errors.New("archived exercise cannot be changed")
	ErrExerciseNotFound     = errors.New("exercise not found")
	ErrInvalidOutboxPayload = errors.New("invalid outbox payload")
	ErrDuplicateSlug        = errors.New("exercise slug already exists")
	ErrMotionSlugRequired   = errors.New("ai support requires a motion slug")
)

// Info is the exercise state. Force and Mechanic are empty when the source
// has no value; Images are paths relative to the media base URL or absolute URLs.
type Info struct {
	ID                 string
	Slug               string
	Name               string
	PrimaryMuscle      MuscleGroup
	SecondaryMuscles   []MuscleGroup
	Equipment          Equipment
	Category           Category
	Level              Level
	Force              Force
	Mechanic           Mechanic
	Instructions       []string
	Images             []string
	VideoURL           string
	MeasurementType    MeasurementType
	DefaultRestSeconds int32
	MotionSlug         string
	HasAISupported     bool
	Source             Source
	Status             Status
	ArchivedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Exercise struct {
	info Info
}

//nolint:gocritic // info is a value object copy to protect aggregate encapsulation
func NewExercise(info Info, now time.Time) (*Exercise, error) {
	if info.Status == "" {
		info.Status = StatusDraft
	}
	info.ArchivedAt = nil
	info.CreatedAt = now
	info.UpdatedAt = now

	exercise := &Exercise{info: normalizeInfo(info)}
	if err := exercise.validate(); err != nil {
		return nil, err
	}

	return exercise, nil
}

//nolint:gocritic // info is a value object copy to protect aggregate encapsulation
func RehydrateExercise(info Info) (*Exercise, error) {
	info = normalizeInfo(info)
	if !info.Status.Valid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStatus, info.Status)
	}

	exercise := &Exercise{info: info}
	if err := exercise.validate(); err != nil {
		return nil, err
	}

	return exercise, nil
}

// SlugFromName derives a custom exercise slug in the source's style
// ("3/4 Sit-Up" -> "3_4_Sit-Up").
func SlugFromName(name string) string {
	var b strings.Builder
	pendingSep := false
	for _, r := range strings.TrimSpace(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			if pendingSep && b.Len() > 0 {
				b.WriteByte('_')
			}
			pendingSep = false
			b.WriteRune(r)
			continue
		}
		pendingSep = true
	}

	return b.String()
}

func (e *Exercise) Info() Info {
	info := e.info
	info.SecondaryMuscles = copySlice(e.info.SecondaryMuscles)
	info.Instructions = copySlice(e.info.Instructions)
	info.Images = copySlice(e.info.Images)

	return info
}

// UpdateInfo replaces the editable fields. Slug, source, status and the AI
// flag are not editable here.
//
//nolint:gocritic // info is a value object copy to protect aggregate encapsulation
func (e *Exercise) UpdateInfo(info Info, now time.Time) error {
	if e.info.Status == StatusArchived {
		return ErrArchivedExercise
	}

	current := e.info
	current.Name = info.Name
	current.PrimaryMuscle = info.PrimaryMuscle
	current.SecondaryMuscles = copySlice(info.SecondaryMuscles)
	current.Equipment = info.Equipment
	current.Category = info.Category
	current.Level = info.Level
	current.Force = info.Force
	current.Mechanic = info.Mechanic
	current.Instructions = copySlice(info.Instructions)
	current.Images = copySlice(info.Images)
	current.VideoURL = info.VideoURL
	current.DefaultRestSeconds = info.DefaultRestSeconds
	current.MotionSlug = info.MotionSlug
	if info.MeasurementType != "" {
		current.MeasurementType = info.MeasurementType
	}
	current.UpdatedAt = now

	current = normalizeInfo(current)
	updated := &Exercise{info: current}
	if err := updated.validate(); err != nil {
		return err
	}

	e.info = current
	return nil
}

// Approve publishes a draft.
func (e *Exercise) Approve(now time.Time) error {
	if e.info.Status != StatusDraft {
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

func (e *Exercise) Restore(now time.Time) error {
	if e.info.Status != StatusArchived {
		return fmt.Errorf("%w: cannot restore exercise with status %s", ErrInvalidTransition, e.info.Status)
	}

	e.info.Status = StatusActive
	e.info.ArchivedAt = nil
	e.info.UpdatedAt = now

	return nil
}

// SetAISupported toggles AI support; enabling it requires a motion slug (D7).
func (e *Exercise) SetAISupported(supported bool, now time.Time) error {
	if e.info.Status == StatusArchived {
		return ErrArchivedExercise
	}
	if supported && e.info.MotionSlug == "" {
		return ErrMotionSlugRequired
	}

	e.info.HasAISupported = supported
	e.info.UpdatedAt = now

	return nil
}

func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusActive, StatusArchived:
		return true
	default:
		return false
	}
}

func (e *Exercise) validate() error {
	info := e.info
	if info.ID == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidExercise)
	}
	if info.Slug == "" {
		return fmt.Errorf("%w: slug is required", ErrInvalidExercise)
	}
	if info.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidExercise)
	}
	if !info.Source.Valid() {
		return fmt.Errorf("%w: unknown source %q", ErrInvalidExercise, info.Source)
	}
	if err := validateTaxonomy(&info); err != nil {
		return err
	}
	if !info.Status.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidStatus, info.Status)
	}
	if !info.MeasurementType.Valid() {
		return fmt.Errorf("%w: invalid measurement type %s", ErrInvalidExercise, info.MeasurementType)
	}
	if info.HasAISupported && info.MotionSlug == "" {
		return ErrMotionSlugRequired
	}

	return nil
}

func validateTaxonomy(info *Info) error {
	if !isKnown(info.PrimaryMuscle, AllMuscleGroups()) {
		return fmt.Errorf("%w: unknown primary muscle %q", ErrInvalidExercise, info.PrimaryMuscle)
	}
	for _, m := range info.SecondaryMuscles {
		if !isKnown(m, AllMuscleGroups()) {
			return fmt.Errorf("%w: unknown secondary muscle %q", ErrInvalidExercise, m)
		}
	}
	if !isKnown(info.Equipment, AllEquipment()) {
		return fmt.Errorf("%w: unknown equipment %q", ErrInvalidExercise, info.Equipment)
	}
	if !isKnown(info.Category, AllCategories()) {
		return fmt.Errorf("%w: unknown category %q", ErrInvalidExercise, info.Category)
	}
	if !isKnown(info.Level, AllLevels()) {
		return fmt.Errorf("%w: unknown level %q", ErrInvalidExercise, info.Level)
	}
	if info.Force != "" && !isKnown(info.Force, AllForces()) {
		return fmt.Errorf("%w: unknown force %q", ErrInvalidExercise, info.Force)
	}
	if info.Mechanic != "" && !isKnown(info.Mechanic, AllMechanics()) {
		return fmt.Errorf("%w: unknown mechanic %q", ErrInvalidExercise, info.Mechanic)
	}

	return nil
}

//nolint:gocritic // info is a value object copy to protect aggregate encapsulation
func normalizeInfo(info Info) Info {
	info.ID = strings.TrimSpace(info.ID)
	info.Slug = strings.TrimSpace(info.Slug)
	info.Name = strings.TrimSpace(info.Name)
	info.VideoURL = strings.TrimSpace(info.VideoURL)
	info.MotionSlug = strings.TrimSpace(info.MotionSlug)
	info.SecondaryMuscles = normalizeSecondary(info.PrimaryMuscle, info.SecondaryMuscles)
	info.Instructions = trimNonEmpty(info.Instructions)
	info.Images = trimNonEmpty(info.Images)
	if info.DefaultRestSeconds == 0 {
		info.DefaultRestSeconds = defaultRestSeconds
	}
	if info.MeasurementType == "" {
		info.MeasurementType = MeasurementTypeReps
	}

	return info
}

// normalizeSecondary drops duplicates and the primary muscle, keeping order.
func normalizeSecondary(primary MuscleGroup, muscles []MuscleGroup) []MuscleGroup {
	out := make([]MuscleGroup, 0, len(muscles))
	seen := map[MuscleGroup]bool{primary: true}
	for _, m := range muscles {
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}

	return out
}

func trimNonEmpty(values []string) []string {
	var out []string
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}

	return out
}

func copySlice[T any](values []T) []T {
	if values == nil {
		return nil
	}

	copied := make([]T, len(values))
	copy(copied, values)

	return copied
}
