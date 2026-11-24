package aggregate

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/event"
	"github.com/viethung213/gym-companion/internal/profile/domain/service"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

// DefaultAvatarURL is the fallback avatar image URL used when user profile omits an avatar.
const DefaultAvatarURL = "https://lqhhwimtzrcrmyfhlizv.supabase.co/storage/v1/object/public/" +
	"gym-companion-assets/avatar_default/male/avatar-vo-tri-loopy-cute.jpg"

type UserProfile struct {
	userID                string
	fullName              string
	avatarURL             string
	biologicalMetrics     vo.BiologicalMetrics
	experienceLevel       string
	goals                 []string
	preferredWorkoutTimes []string
	availableEquipment    []vo.Equipment
	preferredMuscleGroups []vo.MuscleGroup
	coachStyle            string
	targetWeightKg        float64
	targetBodyFatPercent  float64
	injuries              []*entity.Injury
	periodicMetrics       []vo.PeriodicMetric
	completionRate        float64
	aiCoachActivated      bool
	timezone              vo.Timezone
	createdAt             time.Time
	updatedAt             time.Time

	domainEvents []any
}

func NewUserProfile(
	userID string,
	bio vo.BiologicalMetrics,
	experienceLevel string,
	goals []string,
	preferredWorkoutTimes []string,
	availableEquipment []vo.Equipment,
	preferredMuscleGroups []vo.MuscleGroup,
	coachStyle string,
	targetWeightKg float64,
	targetBodyFatPercent float64,
	injuries []*entity.Injury,
) (*UserProfile, error) {
	if userID == "" {
		return nil, errors.New("user_id required")
	}
	if err := validateTraining(availableEquipment, preferredMuscleGroups); err != nil {
		return nil, err
	}
	if err := validateWorkoutTimes(preferredWorkoutTimes); err != nil {
		return nil, err
	}

	if coachStyle == "" {
		coachStyle = "FRIENDLY"
	}

	now := time.Now()
	calc := service.NewCompletionCalculator()
	result := calc.Calculate(
		bio,
		experienceLevel,
		goals,
		preferredWorkoutTimes,
		availableEquipment,
		preferredMuscleGroups,
		coachStyle,
		targetWeightKg,
		injuries,
	)

	periodicMetrics := make([]vo.PeriodicMetric, 0)
	if bio.WeightKg() > 0 {
		initialMetric, err := vo.NewPeriodicMetric(uuid.NewString(), bio.WeightKg(), 0, "", now)
		if err == nil {
			periodicMetrics = append(periodicMetrics, initialMetric)
		}
	}

	copiedGoals := copyStringSlice(goals)
	copiedTimes := copyStringSlice(preferredWorkoutTimes)
	copiedEquip := copySlice(availableEquipment)
	copiedMuscles := copySlice(preferredMuscleGroups)
	copiedInjuries := copyInjurySlice(injuries)

	p := &UserProfile{
		userID:                userID,
		avatarURL:             DefaultAvatarURL,
		biologicalMetrics:     bio,
		experienceLevel:       experienceLevel,
		goals:                 copiedGoals,
		preferredWorkoutTimes: copiedTimes,
		availableEquipment:    copiedEquip,
		preferredMuscleGroups: copiedMuscles,
		coachStyle:            coachStyle,
		targetWeightKg:        targetWeightKg,
		targetBodyFatPercent:  targetBodyFatPercent,
		injuries:              copiedInjuries,
		periodicMetrics:       periodicMetrics,
		completionRate:        result.CompletionRate,
		aiCoachActivated:      result.AICoachActivated,
		createdAt:             now,
		updatedAt:             now,
		domainEvents:          make([]any, 0),
	}

	// Always record ProfileUpdatedEvent
	p.RecordEvent(event.NewProfileUpdatedEvent(userID, bio, p.goals, result.CompletionRate, now))

	// Record ProfileCompletedEvent only if completionRate >= 80% or AI coach activated
	if result.AICoachActivated || result.CompletionRate >= 80.0 {
		p.RecordEvent(event.NewProfileCompletedEvent(
			userID, bio, p.goals, p.injuries, p.preferredWorkoutTimes, now,
		))
	}

	return p, nil
}

func ReconstituteUserProfile(
	userID string,
	bio vo.BiologicalMetrics,
	experienceLevel string,
	goals []string,
	preferredWorkoutTimes []string,
	availableEquipment []vo.Equipment,
	preferredMuscleGroups []vo.MuscleGroup,
	coachStyle string,
	targetWeightKg float64,
	targetBodyFatPercent float64,
	injuries []*entity.Injury,
	periodicMetrics []vo.PeriodicMetric,
	completionRate float64,
	aiCoachActivated bool,
	createdAt time.Time,
	updatedAt time.Time,
) *UserProfile {
	return &UserProfile{
		userID:                userID,
		biologicalMetrics:     bio,
		experienceLevel:       experienceLevel,
		goals:                 copyStringSlice(goals),
		preferredWorkoutTimes: copyStringSlice(preferredWorkoutTimes),
		availableEquipment:    copySlice(availableEquipment),
		preferredMuscleGroups: copySlice(preferredMuscleGroups),
		coachStyle:            coachStyle,
		targetWeightKg:        targetWeightKg,
		targetBodyFatPercent:  targetBodyFatPercent,
		injuries:              copyInjurySlice(injuries),
		periodicMetrics:       copyPeriodicMetricSlice(periodicMetrics),
		completionRate:        completionRate,
		aiCoachActivated:      aiCoachActivated,
		createdAt:             createdAt,
		updatedAt:             updatedAt,
		domainEvents:          make([]any, 0),
	}
}

func (p *UserProfile) UserID() string    { return p.userID }
func (p *UserProfile) FullName() string  { return p.fullName }
func (p *UserProfile) AvatarURL() string { return p.avatarURL }
func (p *UserProfile) SetIdentity(fullName, avatarURL string) {
	if fullName != "" {
		p.fullName = fullName
	}
	url := strings.TrimSpace(avatarURL)
	if url == "" {
		url = DefaultAvatarURL
	}
	p.avatarURL = url
}

func (p *UserProfile) UpdateIdentity(fullName, avatarURL *string) error {
	if fullName == nil && avatarURL == nil {
		return derror.ErrEmptyIdentityUpdate
	}

	hasChange := false
	if fullName != nil {
		name := strings.TrimSpace(*fullName)
		if len(name) > 100 {
			return derror.ErrInvalidFullName
		}
		if name != "" && name != p.fullName {
			p.fullName = name
			hasChange = true
		}
	}

	if avatarURL != nil {
		url := strings.TrimSpace(*avatarURL)
		if url == "" {
			url = DefaultAvatarURL
		}
		if url != p.avatarURL {
			p.avatarURL = url
			hasChange = true
		}
	}

	if !hasChange {
		return nil
	}

	p.updatedAt = time.Now()
	p.RecordEvent(event.NewUserIdentityUpdatedEvent(p.userID, p.fullName, p.avatarURL, p.updatedAt))
	return nil
}
func (p *UserProfile) BiologicalMetrics() vo.BiologicalMetrics { return p.biologicalMetrics }
func (p *UserProfile) ExperienceLevel() string                 { return p.experienceLevel }
func (p *UserProfile) Goals() []string                         { return copyStringSlice(p.goals) }
func (p *UserProfile) PreferredWorkoutTimes() []string {
	return copyStringSlice(p.preferredWorkoutTimes)
}
func (p *UserProfile) AvailableEquipment() []vo.Equipment { return copySlice(p.availableEquipment) }
func (p *UserProfile) PreferredMuscleGroups() []vo.MuscleGroup {
	return copySlice(p.preferredMuscleGroups)
}
func (p *UserProfile) CoachStyle() string            { return p.coachStyle }
func (p *UserProfile) TargetWeightKg() float64       { return p.targetWeightKg }
func (p *UserProfile) TargetBodyFatPercent() float64 { return p.targetBodyFatPercent }
func (p *UserProfile) Injuries() []*entity.Injury    { return copyInjurySlice(p.injuries) }
func (p *UserProfile) PeriodicMetrics() []vo.PeriodicMetric {
	return copyPeriodicMetricSlice(p.periodicMetrics)
}
func (p *UserProfile) CompletionRate() float64 { return p.completionRate }
func (p *UserProfile) AICoachActivated() bool  { return p.aiCoachActivated }
func (p *UserProfile) CreatedAt() time.Time    { return p.createdAt }
func (p *UserProfile) UpdatedAt() time.Time    { return p.updatedAt }

// Timezone returns the athlete's IANA zone; slot times are wall clock in it.
func (p *UserProfile) Timezone() vo.Timezone {
	if p.timezone == "" {
		return vo.DefaultTimezone
	}
	return p.timezone
}

// SetTimezone records the athlete's zone, parsed by vo.ParseTimezone.
// The zero value keeps the current zone, so an update that omits it changes nothing.
func (p *UserProfile) SetTimezone(tz vo.Timezone) {
	if tz != "" {
		p.timezone = tz
	}
}

// validateTraining rejects unknown equipment or muscle group codes, which may
// reach the aggregate from stored JSONB or callers.
// MaxWorkoutSlotsPerWeek caps the weekly slots an athlete may select. The coach
// schedules one session per slot: up to six training sessions (BR-AC-01), the
// rest active recovery. Must match coaching roadmap.MaxSlotsPerWeek.
const MaxWorkoutSlotsPerWeek = 10

// validateWorkoutTimes rejects more than MaxWorkoutSlotsPerWeek distinct slots.
func validateWorkoutTimes(times []string) error {
	distinct := make(map[string]struct{}, len(times))
	for _, t := range times {
		if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
			distinct[t] = struct{}{}
		}
	}
	if len(distinct) > MaxWorkoutSlotsPerWeek {
		return fmt.Errorf("%w: %d selected, at most %d", derror.ErrTooManyWorkoutSlots, len(distinct), MaxWorkoutSlotsPerWeek)
	}
	return nil
}

func validateTraining(equipment []vo.Equipment, muscles []vo.MuscleGroup) error {
	for _, e := range equipment {
		if !e.Valid() {
			return fmt.Errorf("%w: unknown equipment %q", derror.ErrInvalidTaxonomy, e)
		}
	}
	for _, m := range muscles {
		if !m.Valid() {
			return fmt.Errorf("%w: unknown muscle group %q", derror.ErrInvalidTaxonomy, m)
		}
	}
	return nil
}

func copySlice[T any](s []T) []T {
	if s == nil {
		return nil
	}
	c := make([]T, len(s))
	copy(c, s)
	return c
}

func copyStringSlice(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	res := make([]string, len(s))
	copy(res, s)
	return res
}

func copyInjurySlice(injuries []*entity.Injury) []*entity.Injury {
	if len(injuries) == 0 {
		return nil
	}
	res := make([]*entity.Injury, len(injuries))
	for i, inj := range injuries {
		if inj != nil {
			res[i] = inj.Clone()
		}
	}
	return res
}

func copyPeriodicMetricSlice(metrics []vo.PeriodicMetric) []vo.PeriodicMetric {
	if len(metrics) == 0 {
		return nil
	}
	res := make([]vo.PeriodicMetric, len(metrics))
	copy(res, metrics)
	return res
}

func (p *UserProfile) PopEvents() []any {
	events := p.domainEvents
	p.domainEvents = make([]any, 0)
	return events
}

func (p *UserProfile) RecordEvent(event any) {
	p.domainEvents = append(p.domainEvents, event)
}

func (p *UserProfile) UpdateProfile(
	bio vo.BiologicalMetrics,
	experienceLevel string,
	goals []string,
	preferredWorkoutTimes []string,
	availableEquipment []vo.Equipment,
	preferredMuscleGroups []vo.MuscleGroup,
	coachStyle string,
	targetWeightKg float64,
	targetBodyFatPercent float64,
	bodyFatPercent ...float64,
) error {
	if err := validateTraining(availableEquipment, preferredMuscleGroups); err != nil {
		return err
	}
	// Only a changed selection is checked, so writes that pass the stored slots
	// through (e.g. logging body metrics) keep working for older profiles.
	if !slices.Equal(preferredWorkoutTimes, p.preferredWorkoutTimes) {
		if err := validateWorkoutTimes(preferredWorkoutTimes); err != nil {
			return err
		}
	}

	prevActivated := p.aiCoachActivated
	prevRate := p.completionRate

	if bio.WeightKg() > 0 || bio.HeightCm() > 0 || !bio.DateOfBirth().IsZero() || bio.Gender() != "" {
		w := bio.WeightKg()
		if w <= 0 {
			w = p.biologicalMetrics.WeightKg()
		}
		h := bio.HeightCm()
		if h <= 0 {
			h = p.biologicalMetrics.HeightCm()
		}
		dob := bio.DateOfBirth()
		if dob.IsZero() {
			dob = p.biologicalMetrics.DateOfBirth()
		}
		g := bio.Gender()
		if g == "" {
			g = p.biologicalMetrics.Gender()
		}
		if dob.IsZero() {
			newBio, err := vo.NewBiologicalMetrics(w, h, bio.Age(), g)
			if err == nil {
				p.biologicalMetrics = newBio
			}
		} else {
			newBio, err := vo.NewBiologicalMetricsWithDOB(w, h, dob, g)
			if err == nil {
				p.biologicalMetrics = newBio
			}
		}

		bf := float64(0)
		if len(bodyFatPercent) > 0 && bodyFatPercent[0] > 0 {
			bf = bodyFatPercent[0]
		}

		if len(p.periodicMetrics) > 0 {
			latestIdx := len(p.periodicMetrics) - 1
			latest := p.periodicMetrics[latestIdx]
			if bf <= 0 {
				bf = latest.BodyFatPercent()
			}
			updatedMetric, err := vo.NewPeriodicMetric(
				latest.ID(),
				w,
				bf,
				latest.ProgressPhotoURL(),
				latest.LoggedAt(),
			)
			if err == nil {
				p.periodicMetrics[latestIdx] = updatedMetric
			}
		} else if w > 0 {
			newMetric, err := vo.NewPeriodicMetric(
				uuid.New().String(),
				w,
				bf,
				"",
				time.Now(),
			)
			if err == nil {
				p.periodicMetrics = append(p.periodicMetrics, newMetric)
			}
		}
	}

	if experienceLevel != "" {
		p.experienceLevel = experienceLevel
	}
	if len(goals) > 0 {
		p.goals = copyStringSlice(goals)
	}
	if len(preferredWorkoutTimes) > 0 {
		p.preferredWorkoutTimes = copyStringSlice(preferredWorkoutTimes)
	}
	if len(availableEquipment) > 0 {
		p.availableEquipment = copySlice(availableEquipment)
	}
	if len(preferredMuscleGroups) > 0 {
		p.preferredMuscleGroups = copySlice(preferredMuscleGroups)
	}
	if coachStyle != "" {
		p.coachStyle = coachStyle
	}
	if targetWeightKg > 0 {
		p.targetWeightKg = targetWeightKg
	}
	if targetBodyFatPercent > 0 {
		p.targetBodyFatPercent = targetBodyFatPercent
	}
	p.updatedAt = time.Now()

	calc := service.NewCompletionCalculator()
	result := calc.Calculate(
		p.biologicalMetrics,
		p.experienceLevel,
		p.goals,
		p.preferredWorkoutTimes,
		p.availableEquipment,
		p.preferredMuscleGroups,
		p.coachStyle,
		p.targetWeightKg,
		p.injuries,
	)
	p.completionRate = result.CompletionRate
	p.aiCoachActivated = result.AICoachActivated

	p.RecordEvent(event.NewProfileUpdatedEvent(p.userID, p.biologicalMetrics, p.goals, result.CompletionRate, p.updatedAt))

	if (!prevActivated || prevRate < 80.0) && (result.AICoachActivated || result.CompletionRate >= 80.0) {
		p.RecordEvent(event.NewProfileCompletedEvent(
			p.userID, p.biologicalMetrics, p.goals, p.injuries, p.preferredWorkoutTimes, p.updatedAt,
		))
	}

	return nil
}

//nolint:gocritic // heavy value object passed by value
func (p *UserProfile) AddPeriodicMetric(metric vo.PeriodicMetric) {
	p.periodicMetrics = append(p.periodicMetrics, metric)
	p.updatedAt = time.Now()
}

func (p *UserProfile) AddInjury(injury *entity.Injury) error {
	for _, existing := range p.injuries {
		if existing.SameLocation(injury) && !existing.IsRecovered() {
			return derror.ErrInjuryAlreadyActive
		}
	}
	injCopy := injury.Clone()
	p.injuries = append(p.injuries, injCopy)
	p.updatedAt = time.Now()
	p.RecordEvent(event.NewInjuryReportedEvent(p.userID, injCopy))
	return nil
}

func (p *UserProfile) RecoverInjury(injuryID string, recoveredAt time.Time) error {
	var target *entity.Injury
	for _, inj := range p.injuries {
		if inj.ID() == injuryID {
			target = inj
			break
		}
	}
	if target == nil {
		return derror.ErrInjuryNotFound
	}

	err := target.Recover(recoveredAt)
	if err != nil {
		return err
	}
	p.updatedAt = time.Now()
	p.RecordEvent(event.NewInjuryRecoveredEvent(p.userID, target, recoveredAt))
	return nil
}
