package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormUserStreakRepository implements repository.UserStreakRepository.
type GormUserStreakRepository struct {
	db *gorm.DB
}

// Compile-time interface check.
var _ repository.UserStreakRepository = (*GormUserStreakRepository)(nil)

// NewGormUserStreakRepository constructs a new GormUserStreakRepository.
func NewGormUserStreakRepository(db *gorm.DB) *GormUserStreakRepository {
	return &GormUserStreakRepository{db: db}
}

// FindByID retrieves a UserStreak aggregate without locks.
func (r *GormUserStreakRepository) FindByID(ctx context.Context, userID string) (*aggregate.UserStreak, error) {
	db := GetDB(ctx, r.db)
	var model UserStreakModel
	err := db.Where("user_id = ?", userID).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find user_streak by id: %w", err)
	}
	return streakModelToDomain(&model), nil
}

// FindByIDForUpdate retrieves a UserStreak aggregate with pessimistic lock (SELECT ... FOR UPDATE).
// Fast-path: queries with SELECT ... FOR UPDATE first since existing records account for ~99.999% of requests.
// Fallback: if record does not exist (gorm.ErrRecordNotFound), seeds a baseline row (ON CONFLICT DO NOTHING)
// to prevent lost updates on brand-new users, then re-acquires the row lock.
func (r *GormUserStreakRepository) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserStreak, error) {
	db := GetDB(ctx, r.db)

	var model UserStreakModel
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&model).Error
	if err == nil {
		return streakModelToDomain(&model), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find user_streak for update: %w", err)
	}

	now := time.Now().UTC()
	initModel := &UserStreakModel{
		UserID:        userID,
		CurrentStreak: 0,
		LongestStreak: 0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if seedErr := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoNothing: true,
	}).Create(initModel).Error; seedErr != nil {
		return nil, fmt.Errorf("seed user_streak baseline for update: %w", seedErr)
	}

	err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&model).Error
	if err != nil {
		return nil, fmt.Errorf("find user_streak for update after seed: %w", err)
	}
	return streakModelToDomain(&model), nil
}

// Save persists the UserStreak aggregate state using UPSERT.
func (r *GormUserStreakRepository) Save(ctx context.Context, userStreak *aggregate.UserStreak) error {
	if userStreak == nil {
		return errors.New("nil user_streak aggregate")
	}

	model := streakDomainToModel(userStreak)
	db := GetDB(ctx, r.db)
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"current_streak", "longest_streak", "last_active_date", "updated_at"}),
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("save user_streak: %w", err)
	}
	return nil
}

// SaveHistory persists an active day entry into streak_history.
func (r *GormUserStreakRepository) SaveHistory(ctx context.Context, history *entity.StreakHistoryEntry) error {
	if history == nil {
		return errors.New("nil streak history entry")
	}

	id := history.ID
	if id == "" {
		id = uuid.NewString()
	}

	actDate, err := time.Parse("2006-01-02", history.ActivityDate)
	if err != nil {
		actDate = time.Now().UTC()
	}

	model := &StreakHistoryModel{
		ID:           id,
		UserID:       history.UserID,
		ActivityDate: actDate,
		ActivityType: history.ActivityType,
		CreatedAt:    history.CreatedAt,
	}

	db := GetDB(ctx, r.db)
	err = db.Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("save streak_history: %w", err)
	}
	return nil
}

// GetHistory retrieves paginated streak history for a user.
func (r *GormUserStreakRepository) GetHistory(
	ctx context.Context,
	userID string,
	limit, offset int,
) ([]*entity.StreakHistoryEntry, int64, error) {
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	db := GetDB(ctx, r.db)
	var total int64
	if err := db.Model(&StreakHistoryModel{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count streak_history: %w", err)
	}

	var models []*StreakHistoryModel
	err := db.Where("user_id = ?", userID).
		Order("activity_date DESC").
		Limit(limit).
		Offset(offset).
		Find(&models).Error
	if err != nil {
		return nil, 0, fmt.Errorf("fetch streak_history: %w", err)
	}

	entries := make([]*entity.StreakHistoryEntry, 0, len(models))
	for _, m := range models {
		entries = append(entries, &entity.StreakHistoryEntry{
			ID:           m.ID,
			UserID:       m.UserID,
			ActivityDate: m.ActivityDate.Format("2006-01-02"),
			ActivityType: m.ActivityType,
			CreatedAt:    m.CreatedAt,
		})
	}

	return entries, total, nil
}

func streakModelToDomain(m *UserStreakModel) *aggregate.UserStreak {
	lastDate := ""
	if m.LastActivityDate != nil {
		lastDate = m.LastActivityDate.Format("2006-01-02")
	}
	return aggregate.ReconstituteUserStreak(
		m.UserID,
		m.CurrentStreak,
		m.LongestStreak,
		lastDate,
		m.UpdatedAt,
	)
}

func streakDomainToModel(s *aggregate.UserStreak) *UserStreakModel {
	var actTime *time.Time
	if s.LastActivityDate() != "" {
		t, err := time.Parse("2006-01-02", s.LastActivityDate())
		if err == nil {
			actTime = &t
		}
	}
	createdAt := s.UpdatedAt()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	return &UserStreakModel{
		UserID:           s.UserID(),
		CurrentStreak:    s.CurrentStreak(),
		LongestStreak:    s.LongestStreak(),
		LastActivityDate: actTime,
		CreatedAt:        createdAt,
		UpdatedAt:        s.UpdatedAt(),
	}
}
