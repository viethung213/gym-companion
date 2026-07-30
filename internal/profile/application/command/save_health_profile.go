package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/repository"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type InjuryInput struct {
	ID          string
	MuscleGroup string
	Severity    string
	Notes       string
}

type SaveHealthProfileCommand struct {
	UserID                string
	WeightKg              float64
	HeightCm              float64
	BodyFatPercent        float64
	DateOfBirth           string
	Age                   int32
	Gender                string
	Goals                 []string
	ExperienceLevel       string
	PreferredWorkoutTimes []string
	AvailableEquipment    []string
	PreferredMuscleGroups []string
	CoachStyle            string
	TargetWeightKg        float64
	TargetBodyFatPercent  float64
	Injuries              []InjuryInput
}

type SaveHealthProfileResult struct {
	UserID           string
	CompletionRate   float64
	AICoachActivated bool
}

type SaveHealthProfileHandler struct {
	repo      repository.UserProfileRepository
	eventPub  port.EventPublisher
	txManager port.TransactionManager
}

func NewSaveHealthProfileHandler(
	repo repository.UserProfileRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
) *SaveHealthProfileHandler {
	return &SaveHealthProfileHandler{
		repo:      repo,
		eventPub:  eventPub,
		txManager: txManager,
	}
}

//nolint:gocritic // Command value object passed by value for CQRS pattern consistency
func (h *SaveHealthProfileHandler) Handle(ctx context.Context, cmd SaveHealthProfileCommand) (*SaveHealthProfileResult, error) {
	var bio vo.BiologicalMetrics
	var err error

	if cmd.DateOfBirth != "" {
		dob, parseErr := time.Parse("2006-01-02", cmd.DateOfBirth)
		if parseErr == nil {
			bio, err = vo.NewBiologicalMetricsWithDOB(cmd.WeightKg, cmd.HeightCm, dob, cmd.Gender)
		} else {
			bio, err = vo.NewBiologicalMetrics(cmd.WeightKg, cmd.HeightCm, cmd.Age, cmd.Gender)
		}
	} else {
		bio, err = vo.NewBiologicalMetrics(cmd.WeightKg, cmd.HeightCm, cmd.Age, cmd.Gender)
	}

	if err != nil {
		return nil, fmt.Errorf("create biological metrics: %w", err)
	}

	injuries := make([]*entity.Injury, 0, len(cmd.Injuries))
	for _, in := range cmd.Injuries {
		inj, injErr := entity.NewInjury(in.ID, in.MuscleGroup, in.Severity, in.Notes, time.Now())
		if injErr != nil {
			return nil, fmt.Errorf("create injury: %w", injErr)
		}
		injuries = append(injuries, inj)
	}

	existingProfile, findErr := h.repo.FindByUserID(ctx, cmd.UserID)
	var profile *aggregate.UserProfile

	if findErr == nil && existingProfile != nil {
		// Update existing profile (such as blank profile created at registration)
		existingProfile.UpdateProfile(
			bio,
			cmd.ExperienceLevel,
			cmd.Goals,
			cmd.PreferredWorkoutTimes,
			cmd.AvailableEquipment,
			cmd.PreferredMuscleGroups,
			cmd.CoachStyle,
			cmd.TargetWeightKg,
			cmd.TargetBodyFatPercent,
			cmd.BodyFatPercent,
		)
		for _, inj := range injuries {
			_ = existingProfile.AddInjury(inj)
		}
		profile = existingProfile
	} else {
		var createErr error
		profile, createErr = aggregate.NewUserProfile(
			cmd.UserID,
			bio,
			cmd.ExperienceLevel,
			cmd.Goals,
			cmd.PreferredWorkoutTimes,
			cmd.AvailableEquipment,
			cmd.PreferredMuscleGroups,
			cmd.CoachStyle,
			cmd.TargetWeightKg,
			cmd.TargetBodyFatPercent,
			injuries,
		)
		if createErr != nil {
			return nil, fmt.Errorf("create user profile: %w", createErr)
		}
	}

	var eventsToPublish []any
	err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if saveErr := h.repo.Save(txCtx, profile); saveErr != nil {
			return fmt.Errorf("save profile: %w", saveErr)
		}
		eventsToPublish = profile.PopEvents()
		if h.eventPub != nil && len(eventsToPublish) > 0 {
			if pubErr := h.eventPub.PublishEvents(txCtx, eventsToPublish); pubErr != nil {
				return fmt.Errorf("publish events: %w", pubErr)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &SaveHealthProfileResult{
		UserID:           profile.UserID(),
		CompletionRate:   profile.CompletionRate(),
		AICoachActivated: profile.AICoachActivated(),
	}, nil
}
