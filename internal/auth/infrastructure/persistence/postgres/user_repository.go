package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"gorm.io/gorm"
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
		return tx
	}
	return r.db.WithContext(ctx)
}

// Create inserts a new user record.
func (r *UserRepository) Create(ctx context.Context, u *aggregate.User) error {
	dbUser := toUserModel(u)
	if err := r.getDB(ctx).Create(dbUser).Error; err != nil {
		return fmt.Errorf("gorm create user: %w", err)
	}
	return nil
}

// Update modifies an existing user record.
func (r *UserRepository) Update(ctx context.Context, u *aggregate.User) error {
	dbUser := toUserModel(u)
	tx := r.getDB(ctx).Model(&UserModel{}).Where("id = ?", dbUser.ID).Updates(dbUser)
	if tx.Error != nil {
		return fmt.Errorf("gorm update user: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return derror.ErrUserNotFound
	}
	return nil
}

// FindByID retrieves a user by ID.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*aggregate.User, error) {
	var dbUser UserModel
	if err := r.getDB(ctx).First(&dbUser, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, derror.ErrUserNotFound
		}
		return nil, fmt.Errorf("gorm find user by id: %w", err)
	}
	return dbUser.ToDomain()
}

// FindByEmail retrieves a user by email.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*aggregate.User, error) {
	var dbUser UserModel
	if err := r.getDB(ctx).First(&dbUser, "email = ?", email).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, derror.ErrUserNotFound
		}
		return nil, fmt.Errorf("gorm find user by email: %w", err)
	}
	return dbUser.ToDomain()
}

// FindByGoogleID retrieves a user by Google Social ID.
func (r *UserRepository) FindByGoogleID(
	ctx context.Context,
	googleID string,
) (*aggregate.User, error) {
	var dbUser UserModel
	if err := r.getDB(ctx).First(&dbUser, "google_id = ?", googleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, derror.ErrUserNotFound
		}
		return nil, fmt.Errorf("gorm find user by google id: %w", err)
	}
	return dbUser.ToDomain()
}

// FindByFacebookID retrieves a user by Facebook Social ID.
func (r *UserRepository) FindByFacebookID(
	ctx context.Context,
	facebookID string,
) (*aggregate.User, error) {
	var dbUser UserModel
	if err := r.getDB(ctx).First(&dbUser, "facebook_id = ?", facebookID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, derror.ErrUserNotFound
		}
		return nil, fmt.Errorf("gorm find user by facebook id: %w", err)
	}
	return dbUser.ToDomain()
}
