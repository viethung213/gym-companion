package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"gorm.io/gorm"
)

// userProfileDBModel represents the database model for profile.users table.
type userProfileDBModel struct {
	UserID                string     `gorm:"column:user_id;primaryKey"`
	DateOfBirth           *time.Time `gorm:"column:date_of_birth"`
	ExperienceLevel       string     `gorm:"column:experience_level"`
	Goals                 string     `gorm:"column:goals"`
	PreferredWorkoutTimes string     `gorm:"column:preferred_workout_times"`
	AvailableEquipment    string     `gorm:"column:available_equipment"`
	PreferredMuscleGroups string     `gorm:"column:preferred_muscle_groups"`
	UpdatedAt             time.Time  `gorm:"column:updated_at"`
}

func (userProfileDBModel) TableName() string {
	return "profile.users"
}

// bodyMetricsDBModel represents the database model for profile.body_metrics table.
type bodyMetricsDBModel struct {
	ID       string    `gorm:"column:id;primaryKey"`
	UserID   string    `gorm:"column:user_id"`
	WeightKg float64   `gorm:"column:weight_kg"`
	HeightCm float64   `gorm:"column:height_cm"`
	LoggedAt time.Time `gorm:"column:logged_at"`
}

func (bodyMetricsDBModel) TableName() string {
	return "profile.body_metrics"
}

// injuryDBModel represents the database model for profile.injuries table.
type injuryDBModel struct {
	ID          string     `gorm:"column:id;primaryKey"`
	UserID      string     `gorm:"column:user_id"`
	MuscleGroup string     `gorm:"column:muscle_group"`
	Notes       string     `gorm:"column:notes"`
	ReportedAt  time.Time  `gorm:"column:reported_at"`
	IsRecovered bool       `gorm:"column:is_recovered"`
	RecoveredAt *time.Time `gorm:"column:recovered_at"`
}

func (injuryDBModel) TableName() string {
	return "profile.injuries"
}

type rawSlot struct {
	DayOfWeek int    `json:"day_of_week"`
	Day       int    `json:"day"`
	StartTime string `json:"start_time"`
	Start     string `json:"start"`
	EndTime   string `json:"end_time"`
	End       string `json:"end"`
}

// PostgresUserProfileReader implements port.UserProfileReader reading from PostgreSQL profile schema.
type PostgresUserProfileReader struct {
	db *gorm.DB
}

var _ port.UserProfileReader = (*PostgresUserProfileReader)(nil)

// NewPostgresUserProfileReader creates a new PostgresUserProfileReader.
func NewPostgresUserProfileReader(db *gorm.DB) *PostgresUserProfileReader {
	return &PostgresUserProfileReader{db: db}
}

// GetProfile fetches user profile details from profile.users, profile.body_metrics, and profile.injuries.
func (r *PostgresUserProfileReader) GetProfile(ctx context.Context, userID string) (port.Profile, error) {
	if r.db == nil {
		log.Printf("[PostgresUserProfileReader] DB pool is nil for userID=%s, falling back to mock profile", userID)
		var mockReader MockUserProfileReader
		return mockReader.GetProfile(ctx, userID)
	}

	var userRecord userProfileDBModel
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&userRecord).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[PostgresUserProfileReader] User profile record not found for userID=%s, returning default safe profile", userID)
			var mockReader MockUserProfileReader
			return mockReader.GetProfile(ctx, userID)
		}
		return port.Profile{}, fmt.Errorf("fetch user profile from DB: %w", err)
	}

	profile := port.Profile{
		UserID:    userID,
		WeightKg:  75.0, // default fallback
		HeightCm:  170.0,
		UpdatedAt: userRecord.UpdatedAt,
	}

	// 1. Calculate Age if DateOfBirth is set
	if userRecord.DateOfBirth != nil {
		now := time.Now()
		age := now.Year() - userRecord.DateOfBirth.Year()
		if now.YearDay() < userRecord.DateOfBirth.YearDay() {
			age--
		}
		if age > 0 {
			profile.Age = age
		}
	}

	// 2. Parse JSON string fields
	if userRecord.Goals != "" {
		var goals []string
		if parseErr := json.Unmarshal([]byte(userRecord.Goals), &goals); parseErr == nil && len(goals) > 0 {
			profile.PrimaryGoal = goals[0]
		}
	}
	if profile.PrimaryGoal == "" {
		profile.PrimaryGoal = "hypertrophy"
	}

	if userRecord.AvailableEquipment != "" {
		var equipment []string
		if parseErr := json.Unmarshal([]byte(userRecord.AvailableEquipment), &equipment); parseErr == nil {
			profile.AvailableEquipment = equipment
		}
	}
	if len(profile.AvailableEquipment) == 0 {
		profile.AvailableEquipment = []string{"barbell", "bench", "cable", "dumbbell", "pull-up-bar", "rack"}
	}

	if userRecord.PreferredMuscleGroups != "" {
		var muscles []string
		if parseErr := json.Unmarshal([]byte(userRecord.PreferredMuscleGroups), &muscles); parseErr == nil {
			profile.PreferredMuscleGroups = muscles
		}
	}
	if len(profile.PreferredMuscleGroups) == 0 {
		profile.PreferredMuscleGroups = []string{"chest", "back", "legs", "shoulders"}
	}

	if userRecord.PreferredWorkoutTimes != "" {
		var rawSlots []rawSlot
		if parseErr := json.Unmarshal([]byte(userRecord.PreferredWorkoutTimes), &rawSlots); parseErr == nil {
			slots := make([]port.Slot, 0, len(rawSlots))
			for _, s := range rawSlots {
				day := s.DayOfWeek
				if day == 0 && s.Day != 0 {
					day = s.Day
				}
				startTime := s.StartTime
				if startTime == "" {
					startTime = s.Start
				}
				endTime := s.EndTime
				if endTime == "" {
					endTime = s.End
				}
				slots = append(slots, port.Slot{
					DayOfWeek: time.Weekday(day),
					StartTime: startTime,
					EndTime:   endTime,
				})
			}
			profile.AvailableSlots = slots
		}
	}
	if len(profile.AvailableSlots) == 0 {
		profile.AvailableSlots = []port.Slot{
			{DayOfWeek: time.Monday, StartTime: "06:00", EndTime: "07:30"},
			{DayOfWeek: time.Wednesday, StartTime: "06:00", EndTime: "07:30"},
			{DayOfWeek: time.Friday, StartTime: "06:00", EndTime: "07:30"},
		}
	}

	// 3. Fetch latest body metrics
	var metric bodyMetricsDBModel
	if mErr := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("logged_at DESC").First(&metric).Error; mErr == nil {
		if metric.WeightKg > 0 {
			profile.WeightKg = metric.WeightKg
		}
		if metric.HeightCm > 0 {
			profile.HeightCm = metric.HeightCm
		}
	}

	// 4. Fetch active injuries
	var injuryRecords []injuryDBModel
	if iErr := r.db.WithContext(ctx).Where("user_id = ? AND (is_recovered IS FALSE OR is_recovered IS NULL)", userID).Find(&injuryRecords).Error; iErr == nil {
		injuries := make([]port.InjuryStatus, 0, len(injuryRecords))
		for _, inj := range injuryRecords {
			injuries = append(injuries, port.InjuryStatus{
				MuscleGroup: inj.MuscleGroup,
				ReportedAt:  inj.ReportedAt,
				RecoveredAt: inj.RecoveredAt,
				Notes:       inj.Notes,
			})
		}
		profile.ActiveInjuries = injuries
	}

	return profile, nil
}
