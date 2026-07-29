package vo

import (
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
)

type BiologicalMetrics struct {
	weightKg    float64
	heightCm    float64
	dateOfBirth time.Time
	ageFallback int32
	gender      string
}

func NewBiologicalMetrics(weightKg, heightCm float64, age int32, gender string) (BiologicalMetrics, error) {
	if weightKg <= 0 || weightKg > 500 {
		return BiologicalMetrics{}, fmt.Errorf("%w: weight must be between 0 and 500kg", derror.ErrInvalidBiological)
	}
	if heightCm <= 0 || heightCm > 300 {
		return BiologicalMetrics{}, fmt.Errorf("%w: height must be between 0 and 300cm", derror.ErrInvalidBiological)
	}
	return BiologicalMetrics{
		weightKg:    weightKg,
		heightCm:    heightCm,
		ageFallback: age,
		gender:      gender,
	}, nil
}

func NewBiologicalMetricsWithDOB(weightKg, heightCm float64, dateOfBirth time.Time, gender string) (BiologicalMetrics, error) {
	if weightKg <= 0 || weightKg > 500 {
		return BiologicalMetrics{}, fmt.Errorf("%w: weight must be between 0 and 500kg", derror.ErrInvalidBiological)
	}
	if heightCm <= 0 || heightCm > 300 {
		return BiologicalMetrics{}, fmt.Errorf("%w: height must be between 0 and 300cm", derror.ErrInvalidBiological)
	}
	return BiologicalMetrics{
		weightKg:    weightKg,
		heightCm:    heightCm,
		dateOfBirth: dateOfBirth,
		gender:      gender,
	}, nil
}

func (b BiologicalMetrics) WeightKg() float64 {
	return b.weightKg
}

func (b BiologicalMetrics) HeightCm() float64 {
	return b.heightCm
}

func (b BiologicalMetrics) DateOfBirth() time.Time {
	return b.dateOfBirth
}

func (b BiologicalMetrics) Gender() string {
	return b.gender
}

func (b BiologicalMetrics) Age() int32 {
	if b.dateOfBirth.IsZero() {
		return b.ageFallback
	}
	now := time.Now()
	years := now.Year() - b.dateOfBirth.Year()
	if now.YearDay() < b.dateOfBirth.YearDay() {
		years--
	}
	if years < 0 {
		return 0
	}
	return int32(years)
}
