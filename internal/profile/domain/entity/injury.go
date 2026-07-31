package entity

import (
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
)

type Injury struct {
	id          string
	muscleGroup string
	severity    string
	notes       string
	reportedAt  time.Time
	isRecovered bool
	recoveredAt *time.Time
}

func NewInjury(id, muscleGroup, severity, notes string, reportedAt time.Time) (*Injury, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: injury id required", derror.ErrInjuryNotFound)
	}
	if muscleGroup == "" {
		return nil, errors.New("muscle group cannot be empty")
	}
	if reportedAt.IsZero() {
		reportedAt = time.Now()
	}
	return &Injury{
		id:          id,
		muscleGroup: muscleGroup,
		severity:    severity,
		notes:       notes,
		reportedAt:  reportedAt,
		isRecovered: false,
		recoveredAt: nil,
	}, nil
}

func ReconstituteInjury(id, muscleGroup, severity, notes string, reportedAt time.Time, isRecovered bool, recoveredAt *time.Time) *Injury {
	return &Injury{
		id:          id,
		muscleGroup: muscleGroup,
		severity:    severity,
		notes:       notes,
		reportedAt:  reportedAt,
		isRecovered: isRecovered,
		recoveredAt: recoveredAt,
	}
}

func (i *Injury) ID() string              { return i.id }
func (i *Injury) MuscleGroup() string     { return i.muscleGroup }
func (i *Injury) Severity() string        { return i.severity }
func (i *Injury) Notes() string           { return i.notes }
func (i *Injury) ReportedAt() time.Time   { return i.reportedAt }
func (i *Injury) IsRecovered() bool       { return i.isRecovered }
func (i *Injury) RecoveredAt() *time.Time {
	if i.recoveredAt == nil {
		return nil
	}
	t := *i.recoveredAt
	return &t
}

func (i *Injury) Clone() *Injury {
	if i == nil {
		return nil
	}
	var recAt *time.Time
	if i.recoveredAt != nil {
		t := *i.recoveredAt
		recAt = &t
	}
	return &Injury{
		id:          i.id,
		muscleGroup: i.muscleGroup,
		severity:    i.severity,
		notes:       i.notes,
		reportedAt:  i.reportedAt,
		isRecovered: i.isRecovered,
		recoveredAt: recAt,
	}
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
