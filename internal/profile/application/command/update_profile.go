package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/repository"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type UpdateProfileCommand struct {
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
}

type UpdateProfileHandler struct {
	repo      repository.UserProfileRepository
	eventPub  port.EventPublisher
	txManager port.TransactionManager
}

func NewUpdateProfileHandler(
	repo repository.UserProfileRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
) *UpdateProfileHandler {
	return &UpdateProfileHandler{
		repo:      repo,
		eventPub:  eventPub,
		txManager: txManager,
	}
}

//nolint:gocritic // Command value object passed by value for CQRS pattern consistency
func (h *UpdateProfileHandler) Handle(ctx context.Context, cmd UpdateProfileCommand) error {
	profile, err := h.repo.FindByUserID(ctx, cmd.UserID)
	if err != nil {
		return fmt.Errorf("find profile for update: %w", err)
	}

	if cmd.WeightKg < 0 || cmd.WeightKg > 500 || cmd.HeightCm < 0 || cmd.HeightCm > 300 {
		return fmt.Errorf("invalid bio metrics: %w", derror.ErrInvalidBiological)
	}

	var bio vo.BiologicalMetrics
	if cmd.DateOfBirth != "" {
		dob, parseErr := time.Parse("2006-01-02", cmd.DateOfBirth)
		if parseErr == nil {
			bio, _ = vo.NewBiologicalMetricsWithDOB(cmd.WeightKg, cmd.HeightCm, dob, cmd.Gender)
		} else {
			bio, _ = vo.NewBiologicalMetrics(cmd.WeightKg, cmd.HeightCm, cmd.Age, cmd.Gender)
		}
	} else {
		bio, _ = vo.NewBiologicalMetrics(cmd.WeightKg, cmd.HeightCm, cmd.Age, cmd.Gender)
	}

	profile.UpdateProfile(
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
	events := profile.PopEvents()

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Update(txCtx, profile); err != nil {
			return err
		}
		if h.eventPub != nil && len(events) > 0 {
			if err := h.eventPub.PublishEvents(txCtx, events); err != nil {
				return fmt.Errorf("publish events on profile update: %w", err)
			}
		}
		return nil
	})
}
