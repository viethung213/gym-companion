package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserRepository implements repository.UserRepository port using GORM over PostgreSQL.
type UserRepository struct {
	db *gorm.DB
}

// Compile-time interface verification
var _ repository.UserRepository = (*UserRepository)(nil)

// NewUserRepository creates a new GORM implementation of UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

// Create inserts a new user record and all associated identities.
func (r *UserRepository) Create(ctx context.Context, u *aggregate.User) error {
	dbUser := toUserModel(u)
	if err := r.getDB(ctx).Create(dbUser).Error; err != nil {
		return fmt.Errorf("gorm create user: %w", err)
	}
	return nil
}

// Update modifies an existing user record and upserts its identities.
func (r *UserRepository) Update(ctx context.Context, u *aggregate.User) error {
	dbUser := toUserModel(u)
	tx := r.getDB(ctx).Model(dbUser).Select("FullName", "RoleID", "Status", "UpdatedAt").Updates(dbUser)
	if tx.Error != nil {
		return fmt.Errorf("gorm update user: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return derror.ErrUserNotFound
	}

	if dbUser.Identity != nil && dbUser.Identity.ID != "" {
		err := r.getDB(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"identity_type",
				"identifier",
				"credential_data",
				"metadata",
				"updated_at",
			}),
		}).Create(dbUser.Identity).Error
		if err != nil {
			return fmt.Errorf("gorm upsert user identity: %w", err)
		}
	}

	return nil
}

// FindByID retrieves a user and their identity by ID.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*aggregate.User, error) {
	var dbUser UserModel
	if err := r.getDB(ctx).Preload("Identity").First(&dbUser, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, derror.ErrUserNotFound
		}
		return nil, fmt.Errorf("gorm find user by id: %w", err)
	}
	return dbUser.ToDomain()
}

// FindByIdentity retrieves a user associated with a specific identity type and identifier.
func (r *UserRepository) FindByIdentity(
	ctx context.Context,
	identityType string,
	identifier string,
) (*aggregate.User, error) {
	var ident UserIdentityModel
	err := r.getDB(ctx).First(
		&ident,
		"identity_type = ? AND identifier = ?",
		identityType,
		identifier,
	).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, derror.ErrUserNotFound
		}
		return nil, fmt.Errorf("gorm find identity: %w", err)
	}
	return r.FindByID(ctx, ident.UserID)
}

// List retrieves users with filtering by role, status, search keyword, and pagination.
func (r *UserRepository) List(
	ctx context.Context,
	filter repository.ListUsersFilter,
) ([]*aggregate.User, int, error) {
	query := r.getDB(ctx).Model(&UserModel{})
	if filter.Role != "" {
		query = query.Where("role_id = ?", filter.Role)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Search != "" {
		term := "%" + strings.TrimSpace(filter.Search) + "%"
		query = query.Where("full_name ILIKE ? OR id IN (SELECT user_id FROM auth.user_identities WHERE identifier ILIKE ?)", term, term)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("gorm count users: %w", err)
	}

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var dbUsers []UserModel
	if err := query.Preload("Identity").Order("created_at DESC").Find(&dbUsers).Error; err != nil {
		return nil, 0, fmt.Errorf("gorm list users: %w", err)
	}

	results := make([]*aggregate.User, 0, len(dbUsers))
	for i := range dbUsers {
		domainUser, err := dbUsers[i].ToDomain()
		if err != nil {
			return nil, 0, fmt.Errorf("map user to domain: %w", err)
		}
		results = append(results, domainUser)
	}

	return results, int(total), nil
}
