package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

type Repository interface {
	Save(ctx context.Context, exercise *domain.Exercise, event *domain.Event) error
	FindByID(ctx context.Context, id string) (*domain.Exercise, error)
	SearchActive(ctx context.Context, filters SearchFilters) ([]*domain.Exercise, error)
	GetMetadata(ctx context.Context) (Metadata, error)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() (string, error)
}

type SearchFilters struct {
	BodyPartID         string
	EquipmentID        string
	TargetMuscleID     string
	SecondaryMuscleIDs []string
	TagIDs             []string
	Keyword            string
	Difficulty         string
	Limit              int32
	Offset             int32
}

type Metadata struct {
	BodyParts  []BodyPart
	Equipments []Equipment
	Muscles    []Muscle
	Tags       []Tag
}

type BodyPart struct {
	ID   string
	Name string
}

type Equipment struct {
	ID   string
	Name string
}

type Muscle struct {
	ID         string
	Name       string
	BodyPartID string
}

type Tag struct {
	ID   string
	Name string
}

type Service struct {
	repository Repository
	clock      Clock
	ids        IDGenerator
}

func NewService(repository Repository, clock Clock, ids IDGenerator) *Service {
	return &Service{
		repository: repository,
		clock:      clock,
		ids:        ids,
	}
}

func (s *Service) CreateExercise(ctx context.Context, info domain.Info) (*domain.Exercise, error) {
	if _, err := RequireAdmin(ctx); err != nil {
		return nil, err
	}

	id, err := s.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate exercise id: %w", err)
	}

	now := s.clock.Now()
	info.ID = id
	exercise, err := domain.NewExercise(info, now)
	if err != nil {
		return nil, err
	}

	event, err := s.newEvent(domain.EventTypeExerciseCreated, exercise, now)
	if err != nil {
		return nil, err
	}
	if err := s.repository.Save(ctx, exercise, event); err != nil {
		return nil, fmt.Errorf("save exercise: %w", err)
	}

	return exercise, nil
}

func (s *Service) UpdateExercise(
	ctx context.Context,
	id string,
	info domain.Info,
) (*domain.Exercise, error) {
	if _, err := RequireAdmin(ctx); err != nil {
		return nil, err
	}

	exercise, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find exercise: %w", err)
	}

	if err := exercise.UpdateInfo(info, s.clock.Now()); err != nil {
		return nil, err
	}
	if err := s.repository.Save(ctx, exercise, nil); err != nil {
		return nil, fmt.Errorf("save exercise: %w", err)
	}

	return exercise, nil
}

func (s *Service) SubmitExerciseForApproval(
	ctx context.Context,
	id string,
) (*domain.Exercise, error) {
	if _, err := RequireAdmin(ctx); err != nil {
		return nil, err
	}

	exercise, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find exercise: %w", err)
	}

	now := s.clock.Now()
	if err := exercise.SubmitForApproval(now); err != nil {
		return nil, err
	}

	event, err := s.newEvent(domain.EventTypeExerciseSubmittedForApproval, exercise, now)
	if err != nil {
		return nil, err
	}
	if err := s.repository.Save(ctx, exercise, event); err != nil {
		return nil, fmt.Errorf("save exercise: %w", err)
	}

	return exercise, nil
}

func (s *Service) ApproveExercise(ctx context.Context, id string) (*domain.Exercise, error) {
	if _, err := RequireAdmin(ctx); err != nil {
		return nil, err
	}

	exercise, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find exercise: %w", err)
	}

	now := s.clock.Now()
	if err := exercise.Approve(now); err != nil {
		return nil, err
	}

	event, err := s.newEvent(domain.EventTypeExerciseApproved, exercise, now)
	if err != nil {
		return nil, err
	}
	if err := s.repository.Save(ctx, exercise, event); err != nil {
		return nil, fmt.Errorf("save exercise: %w", err)
	}

	return exercise, nil
}

func (s *Service) ArchiveExercise(ctx context.Context, id string) error {
	if _, err := RequireAdmin(ctx); err != nil {
		return err
	}

	exercise, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find exercise: %w", err)
	}

	now := s.clock.Now()
	if err := exercise.Archive(now); err != nil {
		return err
	}

	event, err := s.newEvent(domain.EventTypeExerciseArchived, exercise, now)
	if err != nil {
		return err
	}
	if err := s.repository.Save(ctx, exercise, event); err != nil {
		return fmt.Errorf("save exercise: %w", err)
	}

	return nil
}

func (s *Service) GetExercise(ctx context.Context, id string) (*domain.Exercise, error) {
	if _, err := RequireAuthenticated(ctx); err != nil {
		return nil, err
	}

	exercise, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find exercise: %w", err)
	}
	if exercise.Info().Status != domain.StatusActive {
		return nil, domain.ErrExerciseNotFound
	}

	return exercise, nil
}

func (s *Service) SearchExercises(
	ctx context.Context,
	filters SearchFilters,
) ([]*domain.Exercise, error) {
	if _, err := RequireAuthenticated(ctx); err != nil {
		return nil, err
	}

	exercises, err := s.repository.SearchActive(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("search exercises: %w", err)
	}

	return exercises, nil
}

func (s *Service) GetCatalogMetadata(ctx context.Context) (Metadata, error) {
	if _, err := RequireAuthenticated(ctx); err != nil {
		return Metadata{}, err
	}

	metadata, err := s.repository.GetMetadata(ctx)
	if err != nil {
		return Metadata{}, fmt.Errorf("get metadata: %w", err)
	}

	return metadata, nil
}

func (s *Service) newEvent(
	eventType string,
	exercise *domain.Exercise,
	now time.Time,
) (*domain.Event, error) {
	payload, err := json.Marshal(newExerciseEventPayload(exercise.Info()))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidOutboxPayload, err)
	}

	id, err := s.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate event id: %w", err)
	}

	return &domain.Event{
		ID:           id,
		Type:         eventType,
		PartitionKey: exercise.Info().ID,
		Payload:      payload,
		CreatedAt:    now,
	}, nil
}

type exerciseEventPayload struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	BodyPartID         string   `json:"bodyPartId"`
	EquipmentID        string   `json:"equipmentId"`
	TargetMuscleID     string   `json:"targetMuscleId"`
	Instructions       string   `json:"instructions"`
	SecondaryMuscleIDs []string `json:"secondaryMuscleIds"`
	ThumbnailURL       string   `json:"thumbnailUrl"`
	MediaURL           string   `json:"mediaUrl"`
	VideoURL           string   `json:"videoUrl"`
	Difficulty         string   `json:"difficulty"`
	DefaultRestSeconds int32    `json:"defaultRestSeconds"`
	TagIDs             []string `json:"tagIds"`
	Status             string   `json:"status"`
}

func newExerciseEventPayload(info domain.Info) exerciseEventPayload {
	return exerciseEventPayload{
		ID:                 info.ID,
		Name:               info.Name,
		BodyPartID:         info.BodyPartID,
		EquipmentID:        info.EquipmentID,
		TargetMuscleID:     info.TargetMuscleID,
		Instructions:       info.Instructions,
		SecondaryMuscleIDs: info.SecondaryMuscleIDs,
		ThumbnailURL:       info.ThumbnailURL,
		MediaURL:           info.MediaURL,
		VideoURL:           info.VideoURL,
		Difficulty:         info.Difficulty,
		DefaultRestSeconds: info.DefaultRestSeconds,
		TagIDs:             info.TagIDs,
		Status:             string(info.Status),
	}
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

type RandomIDGenerator struct{}

func (RandomIDGenerator) NewID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	encoded := hex.EncodeToString(bytes[:])
	return encoded[0:8] + "-" +
		encoded[8:12] + "-" +
		encoded[12:16] + "-" +
		encoded[16:20] + "-" +
		encoded[20:32], nil
}
