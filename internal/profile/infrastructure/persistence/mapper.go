package persistence

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

func ToPersistenceModels(domain *aggregate.UserProfile) (*UserProfileModel, []*BodyMetricModel, []*InjuryModel, error) {
	goalsJSON, err := json.Marshal(domain.Goals())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal goals: %w", err)
	}

	timesJSON, err := json.Marshal(domain.PreferredWorkoutTimes())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal preferred workout times: %w", err)
	}

	equipmentJSON, err := json.Marshal(domain.AvailableEquipment())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal available equipment: %w", err)
	}

	muscleGroupsJSON, err := json.Marshal(domain.PreferredMuscleGroups())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal preferred muscle groups: %w", err)
	}

	bio := domain.BiologicalMetrics()
	var dob *time.Time
	if !bio.DateOfBirth().IsZero() {
		t := bio.DateOfBirth()
		dob = &t
	}

	userModel := &UserProfileModel{
		UserID:                domain.UserID(),
		FullName:              domain.FullName(),
		AvatarURL:             domain.AvatarURL(),
		DateOfBirth:           dob,
		Gender:                bio.Gender(),
		ExperienceLevel:       domain.ExperienceLevel(),
		Goals:                 goalsJSON,
		PreferredWorkoutTimes: timesJSON,
		AvailableEquipment:    equipmentJSON,
		PreferredMuscleGroups: muscleGroupsJSON,
		CoachStyle:            domain.CoachStyle(),
		TargetWeightKg:        domain.TargetWeightKg(),
		TargetBodyFatPercent:  domain.TargetBodyFatPercent(),
		CompletionRate:        domain.CompletionRate(),
		AICoachActivated:      domain.AICoachActivated(),
		Timezone:              domain.Timezone().String(),
		CreatedAt:             domain.CreatedAt(),
		UpdatedAt:             domain.UpdatedAt(),
	}

	metricModels := make([]*BodyMetricModel, 0, len(domain.PeriodicMetrics()))
	for _, m := range domain.PeriodicMetrics() {
		h := m.HeightCm()
		if h <= 0 {
			h = bio.HeightCm()
		}
		metricModels = append(metricModels, &BodyMetricModel{
			ID:               m.ID(),
			UserID:           domain.UserID(),
			WeightKg:         m.WeightKg(),
			HeightCm:         h,
			BodyFatPercent:   m.BodyFatPercent(),
			ProgressPhotoURL: m.ProgressPhotoURL(),
			LoggedAt:         m.LoggedAt(),
		})
	}

	injuryModels := make([]*InjuryModel, 0, len(domain.Injuries()))
	for _, inj := range domain.Injuries() {
		injuryModels = append(injuryModels, newInjuryModel(domain.UserID(), inj, domain.UpdatedAt()))
	}

	return userModel, metricModels, injuryModels, nil
}

func ToDomainAggregate(
	userModel *UserProfileModel,
	metricModels []*BodyMetricModel,
	injuryModels []*InjuryModel,
) (*aggregate.UserProfile, error) {
	var latestWeight float64
	var latestHeight float64

	periodicMetrics := make([]vo.PeriodicMetric, 0, len(metricModels))
	for _, mm := range metricModels {
		pm, err := vo.NewPeriodicMetric(mm.ID, mm.WeightKg, mm.BodyFatPercent, mm.ProgressPhotoURL, mm.LoggedAt, mm.HeightCm)
		if err != nil {
			return nil, fmt.Errorf("create periodic metric valobj: %w", err)
		}
		periodicMetrics = append(periodicMetrics, pm)
		if mm.WeightKg > 0 {
			latestWeight = mm.WeightKg
		}
		if mm.HeightCm > 0 {
			latestHeight = mm.HeightCm
		}
	}

	var bio vo.BiologicalMetrics
	var err error
	if userModel.DateOfBirth != nil && !userModel.DateOfBirth.IsZero() {
		bio, err = vo.NewBiologicalMetricsWithDOB(latestWeight, latestHeight, *userModel.DateOfBirth, userModel.Gender)
	} else {
		bio, err = vo.NewBiologicalMetrics(latestWeight, latestHeight, 0, userModel.Gender)
	}

	if err != nil {
		bio, _ = vo.NewBiologicalMetrics(1.0, 1.0, 0, userModel.Gender)
	}

	var goals []string
	if len(userModel.Goals) > 0 {
		if err = json.Unmarshal(userModel.Goals, &goals); err != nil {
			return nil, fmt.Errorf("unmarshal goals: %w", err)
		}
	}

	var workoutTimes []string
	if len(userModel.PreferredWorkoutTimes) > 0 {
		if err = json.Unmarshal(userModel.PreferredWorkoutTimes, &workoutTimes); err != nil {
			return nil, fmt.Errorf("unmarshal preferred workout times: %w", err)
		}
	}

	equipment, err := unmarshalCodes(userModel.AvailableEquipment, vo.ParseEquipmentList)
	if err != nil {
		return nil, fmt.Errorf("available equipment: %w", err)
	}
	muscleGroups, err := unmarshalCodes(userModel.PreferredMuscleGroups, vo.ParseMuscleGroupList)
	if err != nil {
		return nil, fmt.Errorf("preferred muscle groups: %w", err)
	}

	injuries := make([]*entity.Injury, 0, len(injuryModels))
	for _, im := range injuryModels {
		inj, injErr := im.toDomain()
		if injErr != nil {
			return nil, injErr
		}
		injuries = append(injuries, inj)
	}

	domainProfile := aggregate.ReconstituteUserProfile(
		userModel.UserID,
		bio,
		userModel.ExperienceLevel,
		goals,
		workoutTimes,
		equipment,
		muscleGroups,
		userModel.CoachStyle,
		userModel.TargetWeightKg,
		userModel.TargetBodyFatPercent,
		injuries,
		periodicMetrics,
		userModel.CompletionRate,
		userModel.AICoachActivated,
		userModel.CreatedAt,
		userModel.UpdatedAt,
	)
	domainProfile.SetIdentity(userModel.FullName, userModel.AvatarURL)
	tz, err := vo.ParseTimezone(userModel.Timezone)
	if err != nil {
		return nil, fmt.Errorf("timezone: %w", err)
	}
	domainProfile.SetTimezone(tz)
	return domainProfile, nil
}

// unmarshalCodes decodes a JSONB array of taxonomy codes and validates it.
func unmarshalCodes[T any](raw []byte, parse func([]string) ([]T, error)) ([]T, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var codes []string
	if err := json.Unmarshal(raw, &codes); err != nil {
		return nil, fmt.Errorf("unmarshal codes: %w", err)
	}
	return parse(codes)
}

func newInjuryModel(userID string, inj *entity.Injury, updatedAt time.Time) *InjuryModel {
	var muscle *string
	if inj.Muscle() != "" {
		m := string(inj.Muscle())
		muscle = &m
	}
	return &InjuryModel{
		ID:          inj.ID(),
		UserID:      userID,
		Region:      string(inj.Region()),
		Muscle:      muscle,
		Side:        string(inj.Side()),
		Severity:    string(inj.Severity()),
		Notes:       inj.Notes(),
		ReportedAt:  inj.ReportedAt(),
		IsRecovered: inj.IsRecovered(),
		RecoveredAt: inj.RecoveredAt(),
		CreatedAt:   inj.ReportedAt(),
		UpdatedAt:   updatedAt,
	}
}

func (m *InjuryModel) toDomain() (*entity.Injury, error) {
	region, err := vo.ParseJointRegion(m.Region)
	if err != nil {
		return nil, fmt.Errorf("injury %s: %w", m.ID, err)
	}
	var muscle vo.MuscleGroup
	if m.Muscle != nil {
		if muscle, err = vo.ParseMuscleGroup(*m.Muscle); err != nil {
			return nil, fmt.Errorf("injury %s: %w", m.ID, err)
		}
	}
	side, err := vo.ParseBodySide(m.Side)
	if err != nil {
		return nil, fmt.Errorf("injury %s: %w", m.ID, err)
	}
	severity, err := vo.ParseInjurySeverity(m.Severity)
	if err != nil {
		return nil, fmt.Errorf("injury %s: %w", m.ID, err)
	}

	return entity.ReconstituteInjury(
		m.ID, region, muscle, side, severity, m.Notes, m.ReportedAt, m.IsRecovered, m.RecoveredAt,
	), nil
}
