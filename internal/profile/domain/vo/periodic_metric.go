package vo

import (
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
)

type PeriodicMetric struct {
	id               string
	weightKg         float64
	heightCm         float64
	bodyFatPercent   float64
	progressPhotoURL string
	loggedAt         time.Time
}

func NewPeriodicMetric(id string, weightKg, bodyFatPercent float64, progressPhotoURL string, loggedAt time.Time, heightCm ...float64) (PeriodicMetric, error) {
	if id == "" {
		return PeriodicMetric{}, fmt.Errorf("%w: metric id cannot be empty", derror.ErrInvalidMetric)
	}
	if weightKg <= 0 || weightKg > 500 {
		return PeriodicMetric{}, fmt.Errorf("%w: weight_kg invalid", derror.ErrInvalidMetric)
	}
	if bodyFatPercent < 0 || bodyFatPercent > 100 {
		return PeriodicMetric{}, fmt.Errorf("%w: body_fat_percent invalid", derror.ErrInvalidMetric)
	}
	if loggedAt.IsZero() {
		loggedAt = time.Now()
	}
	h := float64(0)
	if len(heightCm) > 0 && heightCm[0] >= 0 {
		h = heightCm[0]
	}
	return PeriodicMetric{
		id:               id,
		weightKg:         weightKg,
		heightCm:         h,
		bodyFatPercent:   bodyFatPercent,
		progressPhotoURL: progressPhotoURL,
		loggedAt:         loggedAt,
	}, nil
}

func (p *PeriodicMetric) ID() string {
	return p.id
}

func (p *PeriodicMetric) WeightKg() float64 {
	return p.weightKg
}

func (p *PeriodicMetric) HeightCm() float64 {
	return p.heightCm
}

func (p *PeriodicMetric) BodyFatPercent() float64 {
	return p.bodyFatPercent
}

func (p *PeriodicMetric) ProgressPhotoURL() string {
	return p.progressPhotoURL
}

func (p *PeriodicMetric) LoggedAt() time.Time {
	return p.loggedAt
}
