package persistence

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormUserWalletRepository implements repository.UserWalletRepository.
type GormUserWalletRepository struct {
	db *gorm.DB
}

// Compile-time interface check.
var _ repository.UserWalletRepository = (*GormUserWalletRepository)(nil)

// NewGormUserWalletRepository constructs a new GormUserWalletRepository.
func NewGormUserWalletRepository(db *gorm.DB) *GormUserWalletRepository {
	return &GormUserWalletRepository{db: db}
}

// FindByID retrieves a UserWallet aggregate without locks.
func (r *GormUserWalletRepository) FindByID(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
	db := GetDB(ctx, r.db)
	var model UserWalletModel
	err := db.Where("user_id = ?", userID).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find user_wallet by id: %w", err)
	}
	return walletModelToDomain(&model), nil
}

// FindByIDForUpdate retrieves a UserWallet aggregate with pessimistic row lock (SELECT ... FOR UPDATE).
// Fast-path: queries with SELECT ... FOR UPDATE first since existing records account for ~99.999% of requests.
// Fallback: if record does not exist (gorm.ErrRecordNotFound), seeds a baseline row (ON CONFLICT DO NOTHING)
// to prevent lost updates on brand-new users, then re-acquires the row lock.
func (r *GormUserWalletRepository) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
	db := GetDB(ctx, r.db)

	var model UserWalletModel
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&model).Error
	if err == nil {
		return walletModelToDomain(&model), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find user_wallet for update: %w", err)
	}

	now := time.Now().UTC()
	initModel := &UserWalletModel{
		UserID:    userID,
		Balance:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if seedErr := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoNothing: true,
	}).Create(initModel).Error; seedErr != nil {
		return nil, fmt.Errorf("seed user_wallet baseline for update: %w", seedErr)
	}

	err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&model).Error
	if err != nil {
		return nil, fmt.Errorf("find user_wallet for update after seed: %w", err)
	}
	return walletModelToDomain(&model), nil
}

// Save persists the UserWallet aggregate state using UPSERT.
func (r *GormUserWalletRepository) Save(ctx context.Context, wallet *aggregate.UserWallet) error {
	if wallet == nil {
		return errors.New("nil user_wallet aggregate")
	}

	model := walletDomainToModel(wallet)
	db := GetDB(ctx, r.db)
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		UpdateAll: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("save user_wallet: %w", err)
	}
	return nil
}

// SaveLedgerEntry appends an immutable FitCoins transaction record into gamification.coin_ledger.
func (r *GormUserWalletRepository) SaveLedgerEntry(ctx context.Context, entry *entity.CoinLedgerEntry) error {
	if entry == nil {
		return errors.New("nil coin ledger entry")
	}

	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}

	actDate, err := time.Parse("2006-01-02", entry.ActivityDate)
	if err != nil {
		actDate = time.Now().UTC()
	}

	model := &CoinLedgerModel{
		ID:             entry.ID,
		UserID:         entry.UserID,
		Amount:         entry.Amount,
		BalanceAfter:   entry.BalanceAfter,
		Reason:         string(entry.Reason),
		ReferenceID:    entry.ReferenceID,
		IdempotencyKey: entry.IdempotencyKey,
		ActivityDate:   actDate,
		AdminID:        entry.AdminID,
		Note:           entry.Note,
		CreatedAt:      entry.CreatedAt,
	}

	db := GetDB(ctx, r.db)
	if err := db.Create(model).Error; err != nil {
		return fmt.Errorf("save coin_ledger entry: %w", err)
	}
	return nil
}

// FindLedgerEntryByIdempotencyKey retrieves an existing ledger entry for a (user_id, idempotency_key) pair.
func (r *GormUserWalletRepository) FindLedgerEntryByIdempotencyKey(
	ctx context.Context,
	userID string,
	key string,
) (*entity.CoinLedgerEntry, error) {
	if userID == "" || key == "" {
		return nil, nil
	}

	db := GetDB(ctx, r.db)
	var model CoinLedgerModel
	err := db.Where("user_id = ? AND idempotency_key = ?", userID, key).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find coin_ledger by idempotency key: %w", err)
	}
	return ledgerModelToDomain(&model), nil
}

// ListLedgerEntries retrieves cursor-paginated ledger entries ordered by (created_at DESC, id DESC).
func (r *GormUserWalletRepository) ListLedgerEntries(
	ctx context.Context,
	userID string,
	limit int,
	cursor string,
) ([]*entity.CoinLedgerEntry, string, error) {
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	db := GetDB(ctx, r.db).Where("user_id = ?", userID)

	if cursor != "" {
		cursorTime, cursorID, err := decodeLedgerCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		db = db.Where(
			"(created_at < ?) OR (created_at = ? AND id < ?)",
			cursorTime,
			cursorTime,
			cursorID,
		)
	}

	var models []*CoinLedgerModel
	err := db.Order("created_at DESC, id DESC").
		Limit(limit + 1).
		Find(&models).Error
	if err != nil {
		return nil, "", fmt.Errorf("list coin_ledger entries: %w", err)
	}

	var nextCursor string
	if len(models) > limit {
		lastItem := models[limit-1]
		nextCursor = encodeLedgerCursor(lastItem.CreatedAt, lastItem.ID)
		models = models[:limit]
	}

	entries := make([]*entity.CoinLedgerEntry, 0, len(models))
	for _, m := range models {
		entries = append(entries, ledgerModelToDomain(m))
	}

	return entries, nextCursor, nil
}

func encodeLedgerCursor(createdAt time.Time, id string) string {
	raw := fmt.Sprintf("%s|%s", createdAt.UTC().Format(time.RFC3339Nano), id)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

func decodeLedgerCursor(cursor string) (time.Time, string, error) {
	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", aggregate.ErrInvalidCursor
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", aggregate.ErrInvalidCursor
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", aggregate.ErrInvalidCursor
	}
	if _, err := uuid.Parse(parts[1]); err != nil {
		return time.Time{}, "", aggregate.ErrInvalidCursor
	}
	return ts, parts[1], nil
}

func walletModelToDomain(m *UserWalletModel) *aggregate.UserWallet {
	return aggregate.ReconstituteUserWallet(
		m.UserID,
		m.Balance,
		formatOptionalDate(m.LastWorkoutRewardDate),
		formatOptionalDate(m.LastPRRewardDate),
		formatOptionalDate(m.LastNutritionRewardDate),
		m.UpdatedAt,
	)
}

func walletDomainToModel(w *aggregate.UserWallet) *UserWalletModel {
	createdAt := w.UpdatedAt()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	return &UserWalletModel{
		UserID:                  w.UserID(),
		Balance:                 w.Balance(),
		LastWorkoutRewardDate:   parseOptionalDate(w.LastWorkoutRewardDate()),
		LastPRRewardDate:        parseOptionalDate(w.LastPRRewardDate()),
		LastNutritionRewardDate: parseOptionalDate(w.LastNutritionRewardDate()),
		CreatedAt:               createdAt,
		UpdatedAt:               w.UpdatedAt(),
	}
}

func ledgerModelToDomain(m *CoinLedgerModel) *entity.CoinLedgerEntry {
	return entity.NewCoinLedgerEntry(
		m.ID,
		m.UserID,
		m.Amount,
		m.BalanceAfter,
		vo.CoinTransactionReason(m.Reason),
		m.ReferenceID,
		m.IdempotencyKey,
		m.ActivityDate.Format("2006-01-02"),
		m.AdminID,
		m.Note,
		m.CreatedAt,
	)
}

func formatOptionalDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func parseOptionalDate(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil
	}
	return &t
}
