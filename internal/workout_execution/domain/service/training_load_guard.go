package service

import (
	"context"
	"fmt"
)

// SessionVolumeHistoryProvider defines the port for fetching recent session volumes.

type SessionVolumeHistoryProvider interface {
	GetRecentVolumesForMuscleGroup(

		ctx context.Context,

		userID, muscleGroup string,

		limit int,

	) ([]float32, error)
}

// TrainingLoadGuard checks if training load exceeds safe thresholds.

type TrainingLoadGuard struct {
	historyProvider SessionVolumeHistoryProvider
}

// NewTrainingLoadGuard constructs a new TrainingLoadGuard instance.

func NewTrainingLoadGuard(provider SessionVolumeHistoryProvider) *TrainingLoadGuard {

	return &TrainingLoadGuard{

		historyProvider: provider,
	}

}

// IsOverloaded returns true if currentVolume > 250% of average of recent completed sessions.

func (g *TrainingLoadGuard) IsOverloaded(

	ctx context.Context,

	userID, muscleGroup string,

	currentVolume float32,

) (isOverloaded bool, avgVol float32, err error) {

	if g.historyProvider == nil {

		return false, 0, nil

	}

	recentVolumes, fetchErr := g.historyProvider.GetRecentVolumesForMuscleGroup(

		ctx, userID, muscleGroup, 5,
	)

	if fetchErr != nil {

		return false, 0, fmt.Errorf("failed to fetch recent volume history: %w", fetchErr)

	}

	if len(recentVolumes) == 0 {

		return false, 0, nil

	}

	var sum float32

	for _, v := range recentVolumes {

		sum += v

	}

	avgVolume := sum / float32(len(recentVolumes))

	if avgVolume > 0 && currentVolume > (avgVolume*2.5) {

		return true, avgVolume, nil

	}

	return false, avgVolume, nil

}
