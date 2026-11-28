package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"gorm.io/gorm"
)

// GormLeaderboardRepository serves the XP leaderboard straight from gamification.user_xp
// and is the source of truth the Redis read model is rebuilt from.
type GormLeaderboardRepository struct {
	db *gorm.DB
}

// Compile-time interface checks.
var (
	_ repository.LeaderboardRepository = (*GormLeaderboardRepository)(nil)
	_ port.LeaderboardScoreSource      = (*GormLeaderboardRepository)(nil)
)

// NewGormLeaderboardRepository constructs a new GormLeaderboardRepository.
func NewGormLeaderboardRepository(db *gorm.DB) *GormLeaderboardRepository {
	return &GormLeaderboardRepository{db: db}
}

type leaderboardRow struct {
	UserID      string    `gorm:"column:user_id"`
	Xp          int64     `gorm:"column:xp"`
	Level       int32     `gorm:"column:level"`
	DisplayName *string   `gorm:"column:display_name"`
	AvatarURL   *string   `gorm:"column:avatar_url"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (row *leaderboardRow) toEntity(rank int64) *entity.LeaderboardUser {
	return &entity.LeaderboardUser{
		Rank:        rank,
		UserID:      row.UserID,
		DisplayName: entity.DisplayNameOrDefault(derefString(row.DisplayName)),
		AvatarURL:   derefString(row.AvatarURL),
		Xp:          row.Xp,
		Level:       row.Level,
		UpdatedAt:   row.UpdatedAt,
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (r *GormLeaderboardRepository) leaderboardQuery(ctx context.Context) *gorm.DB {
	return GetDB(ctx, r.db).Table("gamification.user_xp u").
		Select("u.user_id, u.xp, u.level, u.updated_at, s.display_name, s.avatar_url").
		Joins("LEFT JOIN gamification.user_snapshots s ON u.user_id = s.user_id")
}

// GetGlobalTop queries the Top users ordered by (xp DESC, updated_at ASC, user_id ASC)
// joined with local snapshots to avoid cross-schema joins.
func (r *GormLeaderboardRepository) GetGlobalTop(
	ctx context.Context,
	limit, offset int,
) ([]*entity.LeaderboardUser, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var rows []leaderboardRow
	err := r.leaderboardQuery(ctx).
		Order("u.xp DESC, u.updated_at ASC, u.user_id ASC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("query global top leaderboard: %w", err)
	}

	result := make([]*entity.LeaderboardUser, 0, len(rows))
	for i := range rows {
		result = append(result, rows[i].toEntity(int64(offset+i+1)))
	}

	return result, int64(len(result)), nil
}

// GetUserRank calculates a specific user's rank using the indexed tie-breaking ordering.
func (r *GormLeaderboardRepository) GetUserRank(
	ctx context.Context,
	userID string,
) (int64, *entity.LeaderboardUser, error) {
	var userRow leaderboardRow
	err := r.leaderboardQuery(ctx).
		Where("u.user_id = ?", userID).
		Take(&userRow).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil, nil
		}
		return 0, nil, fmt.Errorf("find user for rank: %w", err)
	}

	var higherCount int64
	rankQuery := `
		SELECT COUNT(*)
		FROM gamification.user_xp
		WHERE (xp > ?)
		   OR (xp = ? AND updated_at < ?)
		   OR (xp = ? AND updated_at = ? AND user_id < ?)
	`
	err = GetDB(ctx, r.db).Raw(
		rankQuery,
		userRow.Xp,
		userRow.Xp, userRow.UpdatedAt,
		userRow.Xp, userRow.UpdatedAt, userRow.UserID,
	).Scan(&higherCount).Error
	if err != nil {
		return 0, nil, fmt.Errorf("calculate user rank: %w", err)
	}

	rank := higherCount + 1
	return rank, userRow.toEntity(rank), nil
}

type leaderboardScoreRow struct {
	UserID    string    `gorm:"column:user_id"`
	Xp        int64     `gorm:"column:xp"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (row leaderboardScoreRow) toEntity() entity.LeaderboardScore {
	return entity.LeaderboardScore{UserID: row.UserID, Xp: row.Xp, UpdatedAt: row.UpdatedAt}
}

// ListScores pages through user_xp by user_id (keyset pagination) for read model rebuilds.
func (r *GormLeaderboardRepository) ListScores(
	ctx context.Context,
	afterUserID string,
	limit int,
) ([]entity.LeaderboardScore, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("list leaderboard scores: limit must be positive, got %d", limit)
	}

	query := GetDB(ctx, r.db).Table("gamification.user_xp").Select("user_id, xp, updated_at")
	if afterUserID != "" {
		// Only filter after the first page: "" is not a valid uuid literal in PostgreSQL.
		query = query.Where("user_id > ?", afterUserID)
	}

	var rows []leaderboardScoreRow
	err := query.Order("user_id ASC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list leaderboard scores: %w", err)
	}

	scores := make([]entity.LeaderboardScore, 0, len(rows))
	for _, row := range rows {
		scores = append(scores, row.toEntity())
	}
	return scores, nil
}

// GetScore returns a single user's leaderboard score, or nil when the user has no XP record.
func (r *GormLeaderboardRepository) GetScore(
	ctx context.Context,
	userID string,
) (*entity.LeaderboardScore, error) {
	var row leaderboardScoreRow
	err := GetDB(ctx, r.db).Table("gamification.user_xp").
		Select("user_id, xp, updated_at").
		Where("user_id = ?", userID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get leaderboard score: %w", err)
	}

	score := row.toEntity()
	return &score, nil
}
