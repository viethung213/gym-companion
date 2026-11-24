package entity

import (
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

// Injury is pain reported on a joint region; muscle is optional (empty when
// the user reported the region only).
type Injury struct {
	id          string
	region      vo.JointRegion
	muscle      vo.MuscleGroup
	side        vo.BodySide
	severity    vo.InjurySeverity
	notes       string
	reportedAt  time.Time
	isRecovered bool
	recoveredAt *time.Time
}

// NewInjury validates a newly reported injury; an empty side means BOTH.
func NewInjury(
	id string,
	region vo.JointRegion,
	muscle vo.MuscleGroup,
	side vo.BodySide,
	severity vo.InjurySeverity,
	notes string,
	reportedAt time.Time,
) (*Injury, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: injury id required", derror.ErrInjuryNotFound)
	}
	if side == "" {
		side = vo.BodySideBoth
	}
	if !region.Valid() {
		return nil, fmt.Errorf("%w: unknown joint region %q", derror.ErrInvalidTaxonomy, region)
	}
	if muscle != "" && !muscle.Valid() {
		return nil, fmt.Errorf("%w: unknown muscle group %q", derror.ErrInvalidTaxonomy, muscle)
	}
	if !side.Valid() {
		return nil, fmt.Errorf("%w: unknown body side %q", derror.ErrInvalidTaxonomy, side)
	}
	if !severity.Valid() {
		return nil, fmt.Errorf("%w: unknown injury severity %q", derror.ErrInvalidTaxonomy, severity)
	}
	if reportedAt.IsZero() {
		reportedAt = time.Now()
	}
	return &Injury{
		id:         id,
		region:     region,
		muscle:     muscle,
		side:       side,
		severity:   severity,
		notes:      notes,
		reportedAt: reportedAt,
	}, nil
}

func ReconstituteInjury(
	id string,
	region vo.JointRegion,
	muscle vo.MuscleGroup,
	side vo.BodySide,
	severity vo.InjurySeverity,
	notes string,
	reportedAt time.Time,
	isRecovered bool,
	recoveredAt *time.Time,
) *Injury {
	return &Injury{
		id:          id,
		region:      region,
		muscle:      muscle,
		side:        side,
		severity:    severity,
		notes:       notes,
		reportedAt:  reportedAt,
		isRecovered: isRecovered,
		recoveredAt: recoveredAt,
	}
}

func (i *Injury) ID() string                  { return i.id }
func (i *Injury) Region() vo.JointRegion      { return i.region }
func (i *Injury) Muscle() vo.MuscleGroup      { return i.muscle }
func (i *Injury) Side() vo.BodySide           { return i.side }
func (i *Injury) Severity() vo.InjurySeverity { return i.severity }
func (i *Injury) Notes() string               { return i.notes }
func (i *Injury) ReportedAt() time.Time       { return i.reportedAt }
func (i *Injury) IsRecovered() bool           { return i.isRecovered }
func (i *Injury) RecoveredAt() *time.Time {
	if i.recoveredAt == nil {
		return nil
	}
	t := *i.recoveredAt
	return &t
}

// SameLocation reports whether both injuries cover the same region, muscle and side.
func (i *Injury) SameLocation(other *Injury) bool {
	return i.region == other.region && i.muscle == other.muscle && i.side == other.side
}

func (i *Injury) Clone() *Injury {
	if i == nil {
		return nil
	}
	clone := *i
	clone.recoveredAt = i.RecoveredAt()
	return &clone
}

func (i *Injury) Recover(recoveredAt time.Time) error {
	if i.isRecovered {
		return derror.ErrInjuryAlreadyClosed
	}
	i.isRecovered = true
	if recoveredAt.IsZero() {
		recoveredAt = time.Now()
	}
	i.recoveredAt = &recoveredAt
	return nil
}
