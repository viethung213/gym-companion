package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GORMSoloMatchRepository struct {
	db *gorm.DB
}

var _ repository.SoloMatchRepository = (*GORMSoloMatchRepository)(nil)

func NewGORMSoloMatchRepository(db *gorm.DB) *GORMSoloMatchRepository {
	return &GORMSoloMatchRepository{db: db}
}

func (r *GORMSoloMatchRepository) Save(ctx context.Context, match *aggregate.SoloMatch) error {
	db := GetDB(ctx, r.db)
	model := FromDomainSoloMatch(match)

	// Lưu bản ghi SoloMatch
	err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status", "winner_id", "is_draw", "ended_at", "updated_at",
		}),
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("save solo match model: %w", err)
	}

	// Lưu các bản ghi Participants
	for i := range model.Participants {
		pCopy := model.Participants[i]
		err = db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"score_value", "is_completed", "completed_at", "result", "updated_at",
			}),
		}).Create(&pCopy).Error
		if err != nil {
			return fmt.Errorf("save match participant model %s: %w", pCopy.ID, err)
		}
	}

	return nil
}

func (r *GORMSoloMatchRepository) FindByID(ctx context.Context, id string) (*aggregate.SoloMatch, error) {
	db := GetDB(ctx, r.db)
	var model SoloMatchModel

	err := db.Preload("Participants").First(&model, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find solo match by id %s: %w", id, err)
	}

	return model.ToDomain()
}

func (r *GORMSoloMatchRepository) FindActiveByUserID(ctx context.Context, userID string) (*aggregate.SoloMatch, error) {
	db := GetDB(ctx, r.db)
	var model SoloMatchModel

	err := db.Preload("Participants").
		Joins("JOIN solo_match.match_participants ON solo_match.match_participants.match_id = solo_match.solo_matches.id").
		Where("solo_match.solo_matches.status = ? AND solo_match.match_participants.user_id = ?",
			valueobject.MatchStatusInProgress.String(), userID).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find active solo match for user %s: %w", userID, err)
	}

	return model.ToDomain()
}

func (r *GORMSoloMatchRepository) FindExpiredMatches(ctx context.Context, now time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	db := GetDB(ctx, r.db)
	var models []SoloMatchModel

	err := db.Preload("Participants").
		Where("status = ? AND timeout_at <= ?", valueobject.MatchStatusInProgress.String(), now).
		Order("timeout_at ASC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("find expired matches: %w", err)
	}

	result := make([]*aggregate.SoloMatch, 0, len(models))
	for i := range models {
		m, err := models[i].ToDomain()
		if err != nil {
			return nil, fmt.Errorf("convert match %s to domain: %w", models[i].ID, err)
		}
		result = append(result, m)
	}

	return result, nil
}

func (r *GORMSoloMatchRepository) ListByUserID(ctx context.Context, userID string, offset, limit int) ([]*aggregate.SoloMatch, error) {
	db := GetDB(ctx, r.db)
	var models []SoloMatchModel

	err := db.Preload("Participants").
		Joins("JOIN solo_match.match_participants ON solo_match.match_participants.match_id = solo_match.solo_matches.id").
		Where("solo_match.match_participants.user_id = ?", userID).
		Order("solo_match.solo_matches.created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("list matches by user id %s: %w", userID, err)
	}

	result := make([]*aggregate.SoloMatch, 0, len(models))
	for i := range models {
		m, err := models[i].ToDomain()
		if err != nil {
			return nil, fmt.Errorf("convert match %s to domain: %w", models[i].ID, err)
		}
		result = append(result, m)
	}

	return result, nil
}
