package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"github.com/viethung213/gym-companion/internal/profile/domain/repository"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type LogPeriodicMetricsCommand struct {
	LogID            string
	UserID           string
	WeightKg         float64
	HeightCm         float64
	BodyFatPercent   float64
	ProgressPhotoURL string
}

type LogPeriodicMetricsResult struct {
	LogID          string
	UserID         string
	WeightKg       float64
	HeightCm       float64
	BodyFatPercent float64
	SyncStatus     string
}

type LogPeriodicMetricsHandler struct {
	repo      repository.UserProfileRepository
	eventPub  port.EventPublisher
	txManager port.TransactionManager
}

func NewLogPeriodicMetricsHandler(
	repo repository.UserProfileRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
) *LogPeriodicMetricsHandler {
	return &LogPeriodicMetricsHandler{
		repo:      repo,
		eventPub:  eventPub,
		txManager: txManager,
	}
}

func (h *LogPeriodicMetricsHandler) Handle(ctx context.Context, cmd LogPeriodicMetricsCommand) (*LogPeriodicMetricsResult, error) {
	profile, err := h.repo.FindByUserID(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user profile: %w", err)
	}

	metric, metricErr := vo.NewPeriodicMetric(cmd.LogID, cmd.WeightKg, cmd.BodyFatPercent, cmd.ProgressPhotoURL, time.Now())
	if metricErr != nil {
		return nil, fmt.Errorf("create periodic metric: %w", metricErr)
	}

	profile.AddPeriodicMetric(metric)

	// Update current weight and height in BiologicalMetrics if provided
	currentBio := profile.BiologicalMetrics()
	newWeight := currentBio.WeightKg()
	if cmd.WeightKg > 0 {
		newWeight = cmd.WeightKg
	}
	newHeight := currentBio.HeightCm()
	if cmd.HeightCm > 0 {
		newHeight = cmd.HeightCm
	}

	if (newWeight > 0 && newWeight != currentBio.WeightKg()) || (newHeight > 0 && newHeight != currentBio.HeightCm()) {
		var updatedBio vo.BiologicalMetrics
		var bioErr error
		if !currentBio.DateOfBirth().IsZero() {
			updatedBio, bioErr = vo.NewBiologicalMetricsWithDOB(newWeight, newHeight, currentBio.DateOfBirth(), currentBio.Gender())
		} else {
			updatedBio, bioErr = vo.NewBiologicalMetrics(newWeight, newHeight, currentBio.Age(), currentBio.Gender())
		}
		if bioErr == nil {
			profile.UpdateProfile(
				updatedBio,
				profile.ExperienceLevel(),
				profile.Goals(),
				profile.PreferredWorkoutTimes(),
				profile.AvailableEquipment(),
				profile.PreferredMuscleGroups(),
				profile.CoachStyle(),
				profile.TargetWeightKg(),
				profile.TargetBodyFatPercent(),
			)
		}
	}

	events := profile.PopEvents()
	err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if saveErr := h.repo.Update(txCtx, profile); saveErr != nil {
			return saveErr
		}
		if h.eventPub != nil && len(events) > 0 {
			if pubErr := h.eventPub.PublishEvents(txCtx, events); pubErr != nil {
				return fmt.Errorf("publish events on metric log: %w", pubErr)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &LogPeriodicMetricsResult{
		LogID:          metric.ID(),
		UserID:         profile.UserID(),
		WeightKg:       metric.WeightKg(),
		HeightCm:       newHeight,
		BodyFatPercent: metric.BodyFatPercent(),
		SyncStatus:     "SYNCED",
	}, nil
}
