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

// GormUserXpRepository implements repository.UserXpRepository.
type GormUserXpRepository struct {
	db *gorm.DB
}

// Compile-time interface check.
var _ repository.UserXpRepository = (*GormUserXpRepository)(nil)

// NewGormUserXpRepository constructs a new GormUserXpRepository.
func NewGormUserXpRepository(db *gorm.DB) *GormUserXpRepository {
	return &GormUserXpRepository{db: db}
}

// FindByID retrieves a UserXp aggregate without locks.
func (r *GormUserXpRepository) FindByID(ctx context.Context, userID string) (*aggregate.UserXp, error) {
	db := GetDB(ctx, r.db)
	var model UserXpModel
	err := db.Where("user_id = ?", userID).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find user_xp by id: %w", err)
	}
	return xpModelToDomain(&model), nil
}

// FindByIDForUpdate retrieves a UserXp aggregate with pessimistic row lock (SELECT ... FOR UPDATE).
// Fast-path: queries with SELECT ... FOR UPDATE first since existing records account for ~99.999% of requests.
// Fallback: if record does not exist (gorm.ErrRecordNotFound), seeds a baseline row (ON CONFLICT DO NOTHING)
// to prevent lost updates on brand-new users, then re-acquires the row lock.
func (r *GormUserXpRepository) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserXp, error) {
	db := GetDB(ctx, r.db)

	var model UserXpModel
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&model).Error
	if err == nil {
		return xpModelToDomain(&model), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find user_xp for update: %w", err)
	}

	now := time.Now().UTC()
	initModel := &UserXpModel{
		UserID:    userID,
		Xp:        0,
		Level:     1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if seedErr := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoNothing: true,
	}).Create(initModel).Error; seedErr != nil {
		return nil, fmt.Errorf("seed user_xp baseline for update: %w", seedErr)
	}

	err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&model).Error
	if err != nil {
		return nil, fmt.Errorf("find user_xp for update after seed: %w", err)
	}
	return xpModelToDomain(&model), nil
}

// Save persists the UserXp aggregate state using UPSERT.
func (r *GormUserXpRepository) Save(ctx context.Context, userXp *aggregate.UserXp) error {
	if userXp == nil {
		return errors.New("nil user_xp aggregate")
	}

	model := xpDomainToModel(userXp)
	db := GetDB(ctx, r.db)
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		UpdateAll: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("save user_xp: %w", err)
	}
	return nil
}

// SaveHistory persists an immutable XP change entry in xp_history.
func (r *GormUserXpRepository) SaveHistory(ctx context.Context, history *entity.XpHistoryEntry) error {
	if history == nil {
		return errors.New("nil xp history entry")
	}

	id := history.ID
	if id == "" {
		id = uuid.NewString()
	}

	actDate, err := time.Parse("2006-01-02", history.ActivityDate)
	if err != nil {
		actDate = time.Now().UTC()
	}

	model := &XpHistoryModel{
		ID:           id,
		UserID:       history.UserID,
		DeltaXp:      history.DeltaXp,
		Source:       history.Source,
		SessionID:    history.SessionID,
		ActivityDate: actDate,
		CreatedAt:    history.CreatedAt,
	}

	db := GetDB(ctx, r.db)
	err = db.Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("save xp_history: %w", err)
	}
	return nil
}

// GetHistory retrieves paginated XP history for a user.
func (r *GormUserXpRepository) GetHistory(
	ctx context.Context,
	userID string,
	limit, offset int,
) ([]*entity.XpHistoryEntry, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	db := GetDB(ctx, r.db)
	var total int64
	if err := db.Model(&XpHistoryModel{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count xp_history: %w", err)
	}

	var models []*XpHistoryModel
	err := db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&models).Error
	if err != nil {
		return nil, 0, fmt.Errorf("fetch xp_history: %w", err)
	}

	entries := make([]*entity.XpHistoryEntry, 0, len(models))
	for _, m := range models {
		entries = append(entries, &entity.XpHistoryEntry{
			ID:           m.ID,
			UserID:       m.UserID,
			DeltaXp:      m.DeltaXp,
			Source:       m.Source,
			SessionID:    m.SessionID,
			ActivityDate: m.ActivityDate.Format("2006-01-02"),
			CreatedAt:    m.CreatedAt,
		})
	}

	return entries, total, nil
}

// HasSession checks if an XP history entry for the given workout session already exists.
func (r *GormUserXpRepository) HasSession(ctx context.Context, userID, sessionID string) (bool, error) {
	if sessionID == "" {
		return false, nil
	}
	db := GetDB(ctx, r.db)
	var count int64
	err := db.Model(&XpHistoryModel{}).
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check session existence: %w", err)
	}
	return count > 0, nil
}

func xpModelToDomain(m *UserXpModel) *aggregate.UserXp {
	var dateStr *string
	if m.LastNutritionRewardDate != nil {
		s := m.LastNutritionRewardDate.Format("2006-01-02")
		dateStr = &s
	}
	return aggregate.ReconstituteUserXp(m.UserID, m.Xp, m.Level, dateStr, m.UpdatedAt)
}

func xpDomainToModel(u *aggregate.UserXp) *UserXpModel {
	var actTime *time.Time
	if u.LastNutritionRewardDate() != nil {
		t, err := time.Parse("2006-01-02", *u.LastNutritionRewardDate())
		if err == nil {
			actTime = &t
		}
	}
	createdAt := u.UpdatedAt()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	return &UserXpModel{
		UserID:                  u.UserID(),
		Xp:                      u.Xp(),
		Level:                   u.Level(),
		LastNutritionRewardDate: actTime,
		CreatedAt:               createdAt,
		UpdatedAt:               u.UpdatedAt(),
	}
}
