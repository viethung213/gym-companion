package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormUserSnapshotRepository struct {
	db *gorm.DB
}

var _ repository.UserSnapshotRepository = (*GormUserSnapshotRepository)(nil)

func NewGormUserSnapshotRepository(db *gorm.DB) *GormUserSnapshotRepository {
	return &GormUserSnapshotRepository{db: db}
}

func (r *GormUserSnapshotRepository) GetByID(ctx context.Context, id string) (*entity.UserSnapshot, error) {
	var m SocialUserModel
	err := getDB(ctx, r.db).Where("id = ?", id).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.NewUserSnapshot(id, "User", "", "user", time.Time{}), nil
		}
		return nil, fmt.Errorf("find social user snapshot: %w", err)
	}

	return ToDomainUserSnapshot(&m), nil
}

func (r *GormUserSnapshotRepository) GetByIDs(ctx context.Context, ids []string) (map[string]*entity.UserSnapshot, error) {
	res := make(map[string]*entity.UserSnapshot)
	if len(ids) == 0 {
		return res, nil
	}

	var models []SocialUserModel
	err := getDB(ctx, r.db).Where("id IN ?", ids).Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("find batch social user snapshots: %w", err)
	}

	for i := range models {
		res[models[i].ID] = ToDomainUserSnapshot(&models[i])
	}

	// Fallback cho user chưa có snapshot
	for _, id := range ids {
		if _, ok := res[id]; !ok {
			res[id] = entity.NewUserSnapshot(id, "User", "", "user", time.Time{})
		}
	}

	return res, nil
}

func (r *GormUserSnapshotRepository) Upsert(ctx context.Context, user *entity.UserSnapshot) error {
	if user == nil || user.ID() == "" {
		return nil
	}

	now := time.Now().UTC()
	role := user.Role()
	if role == "" {
		role = "user"
	}

	m := &SocialUserModel{
		ID:        user.ID(),
		FullName:  user.FullName(),
		AvatarURL: user.AvatarURL(),
		Role:      role,
		UpdatedAt: now,
	}

	err := getDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"full_name":  gorm.Expr("CASE WHEN EXCLUDED.full_name <> '' THEN EXCLUDED.full_name ELSE social.users.full_name END"),
			"avatar_url": gorm.Expr("CASE WHEN EXCLUDED.avatar_url <> '' THEN EXCLUDED.avatar_url ELSE social.users.avatar_url END"),
			"role":       gorm.Expr("CASE WHEN EXCLUDED.role <> '' THEN EXCLUDED.role ELSE social.users.role END"),
			"updated_at": gorm.Expr("EXCLUDED.updated_at"),
		}),
	}).Create(m).Error

	if err != nil {
		return fmt.Errorf("upsert social user snapshot: %w", err)
	}

	return nil
}

func (r *GormUserSnapshotRepository) UpdateRole(ctx context.Context, userID, newRole string) error {
	if userID == "" || newRole == "" {
		return nil
	}
	now := time.Now().UTC()
	err := getDB(ctx, r.db).Model(&SocialUserModel{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"role":       newRole,
			"updated_at": now,
		}).Error
	if err != nil {
		return fmt.Errorf("update social user role: %w", err)
	}
	return nil
}

func (r *GormUserSnapshotRepository) SearchUsers(
	ctx context.Context,
	query string,
	role string,
	limit int,
	cursor string,
) (users []*entity.UserSnapshot, nextCursor string, totalCount int32, err error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	db := getDB(ctx, r.db).Model(&SocialUserModel{})

	// Luôn loại trừ role 'admin' theo yêu cầu nghiệp vụ
	db = db.Where("role <> 'admin'")

	// Lọc theo role nếu được chỉ định
	if role != "" {
		db = db.Where("role = ?", role)
	}

	// Tìm kiếm theo họ tên (hỗ trợ cả PostgreSQL và SQLite)
	if strings.TrimSpace(query) != "" {
		searchTerm := "%" + strings.TrimSpace(query) + "%"
		db = db.Where("LOWER(full_name) LIKE LOWER(?)", searchTerm)
	}

	var total int64
	if countErr := db.Count(&total).Error; countErr != nil {
		return nil, "", 0, fmt.Errorf("count social users: %w", countErr)
	}

	if cursor != "" {
		cursorTime, parseErr := time.Parse(time.RFC3339Nano, cursor)
		if parseErr == nil {
			db = db.Where("updated_at < ?", cursorTime)
		}
	}

	var models []SocialUserModel
	findErr := db.Order("updated_at DESC, id DESC").Limit(limit).Find(&models).Error
	if findErr != nil {
		return nil, "", 0, fmt.Errorf("search social users: %w", findErr)
	}

	if len(models) == limit {
		nextCursor = models[len(models)-1].UpdatedAt.Format(time.RFC3339Nano)
	}

	results := make([]*entity.UserSnapshot, len(models))
	for i := range models {
		results[i] = ToDomainUserSnapshot(&models[i])
	}

	return results, nextCursor, int32(total), nil
}
