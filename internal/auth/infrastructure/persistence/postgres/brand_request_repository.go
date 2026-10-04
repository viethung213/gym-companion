package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"gorm.io/gorm"
)

// BrandRequestRepository implements repository.BrandRequestRepository using GORM.
type BrandRequestRepository struct {
	db *gorm.DB
}

// Compile-time interface verification
var _ repository.BrandRequestRepository = (*BrandRequestRepository)(nil)

func toBrandRequestModel(r *entity.BrandRequest) *BrandRequestModel {
	var rejectionReason *string
	if r.RejectionReason() != "" {
		reason := r.RejectionReason()
		rejectionReason = &reason
	}
	var reviewedBy *string
	if r.ReviewedBy() != "" {
		reviewer := r.ReviewedBy()
		reviewedBy = &reviewer
	}

	return &BrandRequestModel{
		ID:              r.ID(),
		UserID:          r.UserID(),
		BrandName:       r.BrandName(),
		Description:     r.Description(),
		ContactPhone:    r.ContactPhone(),
		Address:         r.Address(),
		Status:          r.Status(),
		RejectionReason: rejectionReason,
		ReviewedBy:      reviewedBy,
		ReviewedAt:      r.ReviewedAt(),
		CreatedAt:       r.CreatedAt(),
		UpdatedAt:       r.UpdatedAt(),
	}
}

func (m *BrandRequestModel) ToDomain() *entity.BrandRequest {
	var rejectionReason string
	if m.RejectionReason != nil {
		rejectionReason = *m.RejectionReason
	}
	var reviewedBy string
	if m.ReviewedBy != nil {
		reviewedBy = *m.ReviewedBy
	}

	return entity.ReconstituteBrandRequest(
		m.ID,
		m.UserID,
		m.BrandName,
		m.Description,
		m.ContactPhone,
		m.Address,
		m.Status,
		rejectionReason,
		reviewedBy,
		m.ReviewedAt,
		m.CreatedAt,
		m.UpdatedAt,
	)
}

// NewBrandRequestRepository creates a new instance of BrandRequestRepository.
func NewBrandRequestRepository(db *gorm.DB) *BrandRequestRepository {
	return &BrandRequestRepository{db: db}
}

func (r *BrandRequestRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

// Create inserts a new brand request record into the database.
func (r *BrandRequestRepository) Create(ctx context.Context, req *entity.BrandRequest) error {
	m := toBrandRequestModel(req)
	if err := r.getDB(ctx).Create(m).Error; err != nil {
		return fmt.Errorf("gorm create brand request: %w", err)
	}
	return nil
}

// Update modifies an existing brand request in the database.
func (r *BrandRequestRepository) Update(ctx context.Context, req *entity.BrandRequest) error {
	m := toBrandRequestModel(req)
	res := r.getDB(ctx).Model(m).
		Select("Status", "RejectionReason", "ReviewedBy", "ReviewedAt", "UpdatedAt").
		Updates(m)
	if res.Error != nil {
		return fmt.Errorf("gorm update brand request: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return derror.ErrBrandRequestNotFound
	}
	return nil
}

// FindByID retrieves a brand request by its ID.
func (r *BrandRequestRepository) FindByID(ctx context.Context, id string) (*entity.BrandRequest, error) {
	var m BrandRequestModel
	if err := r.getDB(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, derror.ErrBrandRequestNotFound
		}
		return nil, fmt.Errorf("gorm find brand request by id: %w", err)
	}
	return m.ToDomain(), nil
}

// FindPendingByUserID retrieves an active pending brand request for a user.
func (r *BrandRequestRepository) FindPendingByUserID(ctx context.Context, userID string) (*entity.BrandRequest, error) {
	var m BrandRequestModel
	err := r.getDB(ctx).
		Where("user_id = ? AND status = ?", userID, entity.BrandRequestStatusPending).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, derror.ErrBrandRequestNotFound
		}
		return nil, fmt.Errorf("gorm find pending brand request by user id: %w", err)
	}
	return m.ToDomain(), nil
}

// List retrieves a paginated list of brand requests optionally filtered by status.
func (r *BrandRequestRepository) List(
	ctx context.Context,
	filter repository.ListBrandRequestsFilter,
) ([]*entity.BrandRequest, int, error) {
	query := r.getDB(ctx).Model(&BrandRequestModel{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("gorm count brand requests: %w", err)
	}

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var models []BrandRequestModel
	if err := query.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("gorm list brand requests: %w", err)
	}

	results := make([]*entity.BrandRequest, len(models))
	for i := range models {
		results[i] = models[i].ToDomain()
	}

	return results, int(total), nil
}
