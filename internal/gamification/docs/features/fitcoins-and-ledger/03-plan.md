# Kế Hoạch Triển Khai Kỹ Thuật (Detailed Implementation Plan): Tiền Tệ Ảo FitCoins & Sổ Cái Bất Biến

## 1. Thiết Kế Mã Nguồn Chi Tiết (Low-Level Code Design)

### 1.1. Tầng Domain (Lõi Nghiệp Vụ Thuần Go)

#### `internal/gamification/domain/vo/coin_transaction_reason.go`
```go
package vo

type CoinTransactionReason string

const (
	ReasonEarnLevelUp         CoinTransactionReason = "EARN_LEVEL_UP"
	ReasonEarnWorkout         CoinTransactionReason = "EARN_WORKOUT"
	ReasonEarnPR              CoinTransactionReason = "EARN_PR"
	ReasonEarnNutrition       CoinTransactionReason = "EARN_NUTRITION"
	ReasonEarnStreakMilestone CoinTransactionReason = "EARN_STREAK_MILESTONE"
	ReasonEarnOnboarding      CoinTransactionReason = "EARN_ONBOARDING"
	ReasonSpendGeneric        CoinTransactionReason = "SPEND_GENERIC"
)
```

#### `internal/gamification/domain/entity/coin_ledger_entry.go`
```go
package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

type CoinLedgerEntry struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Amount         int32 // Dương: nạp, Âm: tiêu (luôn khác 0)
	BalanceAfter   int64 // Số dư sau giao dịch (luôn >= 0)
	Reason         vo.CoinTransactionReason
	SourceEventID  string
	IdempotencyKey string
	Metadata       map[string]any
	CreatedAt      time.Time
}

func NewCoinLedgerEntry(
	userID uuid.UUID,
	amount int32,
	balanceAfter int64,
	reason vo.CoinTransactionReason,
	sourceEventID string,
	idempotencyKey string,
	metadata map[string]any,
) (*CoinLedgerEntry, error) {
	if amount == 0 {
		return nil, ErrZeroTransactionAmount
	}
	if balanceAfter < 0 {
		return nil, ErrNegativeBalanceAfter
	}
	if idempotencyKey == "" {
		return nil, ErrMissingIdempotencyKey
	}

	return &CoinLedgerEntry{
		ID:             uuid.New(),
		UserID:         userID,
		Amount:         amount,
		BalanceAfter:   balanceAfter,
		Reason:         reason,
		SourceEventID:  sourceEventID,
		IdempotencyKey: idempotencyKey,
		Metadata:       metadata,
		CreatedAt:      time.Now().UTC(),
	}, nil
}
```

#### `internal/gamification/domain/aggregate/user_wallet.go`
```go
package aggregate

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInsufficientBalance    = errors.New("wallet: insufficient balance for spending")
	ErrNegativeSpendAmount    = errors.New("wallet: spend amount must be strictly positive")
	ErrDailyCapExceeded       = errors.New("wallet: daily earning cap exceeded for today")
	ErrZeroTransactionAmount = errors.New("ledger: transaction amount cannot be zero")
	ErrNegativeBalanceAfter   = errors.New("ledger: balance after transaction cannot be negative")
	ErrMissingIdempotencyKey  = errors.New("ledger: idempotency key is required")
)

type UserWallet struct {
	userID                  uuid.UUID
	balance                 int64
	lastWorkoutRewardDate   *time.Time
	lastPRRewardDate        *time.Time
	lastNutritionRewardDate *time.Time
	createdAt               time.Time
	updatedAt               time.Time
}

func NewUserWallet(userID uuid.UUID) *UserWallet {
	now := time.Now().UTC()
	return &UserWallet{
		userID:    userID,
		balance:   0,
		createdAt: now,
		updatedAt: now,
	}
}

// UserID getter
func (w *UserWallet) UserID() uuid.UUID { return w.userID }

// Balance getter
func (w *UserWallet) Balance() int64 { return w.balance }

// Getters cho ngày trần
func (w *UserWallet) LastWorkoutRewardDate() *time.Time   { return w.lastWorkoutRewardDate }
func (w *UserWallet) LastPRRewardDate() *time.Time        { return w.lastPRRewardDate }
func (w *UserWallet) LastNutritionRewardDate() *time.Time { return w.lastNutritionRewardDate }

func isSameDate(t1 *time.Time, t2 time.Time) bool {
	if t1 == nil {
		return false
	}
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// Case 1: Thăng cấp độ
func (w *UserWallet) EarnFromLevelUp(newLevel int32) int32 {
	var reward int32 = 10
	if newLevel >= 100 {
		reward = 200
	} else if newLevel%10 == 0 {
		reward = 50
	}

	w.balance += int64(reward)
	w.updatedAt = time.Now().UTC()
	return reward
}

// Case 2: Buổi tập hợp lệ (Daily Cap = 1 lần/ngày)
func (w *UserWallet) EarnFromWorkout(workoutDate time.Time) (int32, error) {
	if isSameDate(w.lastWorkoutRewardDate, workoutDate) {
		return 0, ErrDailyCapExceeded
	}

	reward := int32(5)
	w.balance += int64(reward)
	w.lastWorkoutRewardDate = &workoutDate
	w.updatedAt = time.Now().UTC()
	return reward, nil
}

// Case 3: Kỷ lục cá nhân (Daily Cap = 1 lần/ngày)
func (w *UserWallet) EarnFromPR(prDate time.Time) (int32, error) {
	if isSameDate(w.lastPRRewardDate, prDate) {
		return 0, ErrDailyCapExceeded
	}

	reward := int32(5)
	w.balance += int64(reward)
	w.lastPRRewardDate = &prDate
	w.updatedAt = time.Now().UTC()
	return reward, nil
}

// Case 4: Kỷ luật dinh dưỡng (Daily Cap = 1 lần/ngày)
func (w *UserWallet) EarnFromNutrition(nutritionDate time.Time) (int32, error) {
	if isSameDate(w.lastNutritionRewardDate, nutritionDate) {
		return 0, ErrDailyCapExceeded
	}

	reward := int32(3)
	w.balance += int64(reward)
	w.lastNutritionRewardDate = &nutritionDate
	w.updatedAt = time.Now().UTC()
	return reward, nil
}

// Case 5: Cột mốc Streak
func (w *UserWallet) EarnFromStreakMilestone(days int32) int32 {
	var reward int32
	switch days {
	case 7:
		reward = 15
	case 30:
		reward = 50
	case 100:
		reward = 150
	case 365:
		reward = 500
	default:
		reward = 10
	}

	w.balance += int64(reward)
	w.updatedAt = time.Now().UTC()
	return reward
}

// Case 6: Onboarding
func (w *UserWallet) EarnFromOnboarding() int32 {
	reward := int32(20)
	w.balance += int64(reward)
	w.updatedAt = time.Now().UTC()
	return reward
}

// Chi tiêu coin an toàn (Domain Invariant: Balance >= Amount)
func (w *UserWallet) Spend(amount int64) error {
	if amount <= 0 {
		return ErrNegativeSpendAmount
	}
	if w.balance < amount {
		return ErrInsufficientBalance
	}

	w.balance -= amount
	w.updatedAt = time.Now().UTC()
	return nil
}
```

---

### 1.2. Tầng Application (Commands, Queries & Ports)

#### `internal/gamification/application/port/wallet_repository.go`
```go
package port

import (
	"context"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

type WalletRepository interface {
	// GetWalletForUpdate khóa dòng ví bằng SELECT FOR UPDATE
	GetWalletForUpdate(ctx context.Context, userID uuid.UUID) (*aggregate.UserWallet, error)

	// GetWallet đọc số dư ví thông thường
	GetWallet(ctx context.Context, userID uuid.UUID) (*aggregate.UserWallet, error)

	// SaveWalletAndLedger lưu cập nhật ví và chèn dòng ledger trong 1 transaction ACID
	SaveWalletAndLedger(ctx context.Context, wallet *aggregate.UserWallet, entry *entity.CoinLedgerEntry) error

	// CheckIdempotency kiểm tra xem idempotency_key đã tồn tại trong coin_ledger chưa
	CheckIdempotency(ctx context.Context, idempotencyKey string) (bool, error)

	// ListLedgerEntries lấy danh sách giao dịch phân trang
	ListLedgerEntries(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]entity.CoinLedgerEntry, int64, error)
}
```

#### `internal/gamification/application/command/spend_coins.go`
```go
package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

type SpendCoinsCommand struct {
	UserID         uuid.UUID
	Amount         int64
	Reason         vo.CoinTransactionReason
	IdempotencyKey string
	Metadata       map[string]any
}

type SpendCoinsResult struct {
	TransactionID uuid.UUID
	BalanceAfter  int64
}

type SpendCoinsHandler struct {
	repo port.WalletRepository
}

func NewSpendCoinsHandler(repo port.WalletRepository) *SpendCoinsHandler {
	return &SpendCoinsHandler{repo: repo}
}

func (h *SpendCoinsHandler) Handle(ctx context.Context, cmd SpendCoinsCommand) (*SpendCoinsResult, error) {
	// 1. Kiểm tra Idempotency
	exists, err := h.repo.CheckIdempotency(ctx, cmd.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}
	if exists {
		// Đã xử lý trước đó, lấy số dư hiện tại trả về
		wallet, err := h.repo.GetWallet(ctx, cmd.UserID)
		if err != nil {
			return nil, err
		}
		return &SpendCoinsResult{
			TransactionID: uuid.Nil,
			BalanceAfter:  wallet.Balance(),
		}, nil
	}

	// 2. Khóa dòng ví người dùng (SELECT FOR UPDATE)
	wallet, err := h.repo.GetWalletForUpdate(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("lock wallet: %w", err)
	}

	// 3. Thực thi nghiệp vụ trừ tiền tại Domain Aggregate
	if err := wallet.Spend(cmd.Amount); err != nil {
		return nil, err
	}

	// 4. Khởi tạo bản ghi sổ cái kiểm toán bất biến
	entry, err := entity.NewCoinLedgerEntry(
		cmd.UserID,
		-int32(cmd.Amount),
		wallet.Balance(),
		cmd.Reason,
		"",
		cmd.IdempotencyKey,
		cmd.Metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("create ledger entry: %w", err)
	}

	// 5. Lưu nguyên tử trong Database Transaction
	if err := h.repo.SaveWalletAndLedger(ctx, wallet, entry); err != nil {
		return nil, fmt.Errorf("persist wallet and ledger: %w", err)
	}

	return &SpendCoinsResult{
		TransactionID: entry.ID,
		BalanceAfter:  wallet.Balance(),
	}, nil
}
```

---

### 1.3. Tầng Infrastructure (Persistence Adapter)

#### `internal/gamification/infrastructure/persistence/wallet_repository.go`
```go
package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresWalletRepository struct {
	db *gorm.DB
}

func NewPostgresWalletRepository(db *gorm.DB) *PostgresWalletRepository {
	return &PostgresWalletRepository{db: db}
}

type walletPO struct {
	UserID                  uuid.UUID  `gorm:"column:user_id;primaryKey"`
	Balance                 int64      `gorm:"column:balance"`
	LastWorkoutRewardDate   *time.Time `gorm:"column:last_workout_reward_date"`
	LastPRRewardDate        *time.Time `gorm:"column:last_pr_reward_date"`
	LastNutritionRewardDate *time.Time `gorm:"column:last_nutrition_reward_date"`
	CreatedAt               time.Time  `gorm:"column:created_at"`
	UpdatedAt               time.Time  `gorm:"column:updated_at"`
}

func (walletPO) TableName() string { return "gamification.user_wallet" }

type ledgerPO struct {
	ID             uuid.UUID `gorm:"column:id;primaryKey"`
	UserID         uuid.UUID `gorm:"column:user_id"`
	Amount         int32     `gorm:"column:amount"`
	BalanceAfter   int64     `gorm:"column:balance_after"`
	Reason         string    `gorm:"column:reason"`
	SourceEventID  string    `gorm:"column:source_event_id"`
	IdempotencyKey string    `gorm:"column:idempotency_key"`
	Metadata       string    `gorm:"column:metadata"` // JSON string
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (ledgerPO) TableName() string { return "gamification.coin_ledger" }

func (r *PostgresWalletRepository) GetWalletForUpdate(ctx context.Context, userID uuid.UUID) (*aggregate.UserWallet, error) {
	var po walletPO
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		Take(&po).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Tự động khởi tạo ví nếu người dùng chưa có ví
		newWallet := aggregate.NewUserWallet(userID)
		insertPO := walletPO{
			UserID:    userID,
			Balance:   0,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if createErr := r.db.WithContext(ctx).Create(&insertPO).Error; createErr != nil {
			return nil, createErr
		}
		return newWallet, nil
	}
	if err != nil {
		return nil, err
	}

	return toDomainWallet(&po), nil
}

func (r *PostgresWalletRepository) SaveWalletAndLedger(
	ctx context.Context,
	wallet *aggregate.UserWallet,
	entry *entity.CoinLedgerEntry,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Cập nhật ví
		wPO := walletPO{
			UserID:                  wallet.UserID(),
			Balance:                 wallet.Balance(),
			LastWorkoutRewardDate:   wallet.LastWorkoutRewardDate(),
			LastPRRewardDate:        wallet.LastPRRewardDate(),
			LastNutritionRewardDate: wallet.LastNutritionRewardDate(),
			UpdatedAt:               time.Now().UTC(),
		}
		if err := tx.Model(&walletPO{}).Where("user_id = ?", wPO.UserID).Updates(map[string]any{
			"balance":                    wPO.Balance,
			"last_workout_reward_date":   wPO.LastWorkoutRewardDate,
			"last_pr_reward_date":        wPO.LastPRRewardDate,
			"last_nutrition_reward_date": wPO.LastNutritionRewardDate,
			"updated_at":                 wPO.UpdatedAt,
		}).Error; err != nil {
			return err
		}

		// 2. Chèn dòng sổ cái append-only
		lPO := ledgerPO{
			ID:             entry.ID,
			UserID:         entry.UserID,
			Amount:         entry.Amount,
			BalanceAfter:   entry.BalanceAfter,
			Reason:         string(entry.Reason),
			SourceEventID:  entry.SourceEventID,
			IdempotencyKey: entry.IdempotencyKey,
			CreatedAt:      entry.CreatedAt,
		}
		return tx.Create(&lPO).Error
	})
}
```

---

## 2. Kế Hoạch Kiểm Thử TDD Tinh Gọn (Lean Testing Strategy)

### 2.1. Unit Test Domain Aggregate (`user_wallet_test.go`)
1. **Kiểm tra Thăng cấp độ (Level Up)**:
   - Level 2 $\rightarrow$ Thưởng $+10$.
   - Level 10 (Mốc tròn chục) $\rightarrow$ Thưởng $+50$.
   - Level 100 $\rightarrow$ Thưởng $+200$.
2. **Kiểm tra Buổi tập hợp lệ & Daily Cap**:
   - Buổi tập 1 cùng ngày `2026-10-11` $\rightarrow$ Thưởng $+5$, `LastWorkoutRewardDate` cập nhật.
   - Buổi tập 2 cùng ngày `2026-10-11` $\rightarrow$ Trả về `ErrDailyCapExceeded`, số dư không đổi.
   - Buổi tập 3 sang ngày hôm sau `2026-10-12` $\rightarrow$ Thưởng $+5$ thành công.
3. **Kiểm tra Chi tiêu (Spending & Invariants)**:
   - Số dư 100 coin, chi tiêu 60 coin $\rightarrow$ Thành công, số dư còn 40.
   - Số dư 40 coin, chi tiêu 50 coin $\rightarrow$ Trả về `ErrInsufficientBalance`, số dư giữ nguyên 40.
   - Chi tiêu số tiền âm hoặc bằng 0 $\rightarrow$ Trả về lỗi `ErrNegativeSpendAmount`.

### 2.2. Unit Test Command Handlers (`spend_coins_test.go`, `earn_coins_test.go`)
1. **Chống trùng lặp Idempotency Key**:
   - Mock repo báo `CheckIdempotency == true` $\rightarrow$ Handler trả về ngay kết quả trước đó mà không trừ coin lần 2.
2. **Xử lý Transaction Rollback**:
   - Mock repo thất bại khi lưu $\rightarrow$ Handler trả về error, không để dữ liệu bị treo.

### 2.3. Integration Test PostgreSQL Thật (`wallet_repository_test.go`)
1. **Chống Chi Tiêu Kép (Anti-Double-Spending)**:
   - Tạo ví có số dư $100$ coin.
   - Chạy 2 goroutines đồng thời cùng gửi lệnh trừ $60$ coin (`SELECT FOR UPDATE`).
   - Kỳ vọng: Đúng 1 goroutine thành công, goroutine thứ 2 thất bại với lỗi `ErrInsufficientBalance`. Số dư cuối cùng phải là $40$ coin (không bao giờ bị âm $-20$ coin).
2. **Ràng buộc Database Constraint `CHECK (balance >= 0)`**:
   - Cố tình chạy lệnh SQL trừ số dư vượt quá giới hạn $\rightarrow$ PostgreSQL engine ném lỗi vi phạm check constraint.
