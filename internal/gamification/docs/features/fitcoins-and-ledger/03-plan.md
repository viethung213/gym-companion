# Kế Hoạch Triển Khai Kỹ Thuật (Implementation Plan): Tiền Tệ Ảo FitCoins & Sổ Cái Bất Biến

Tài liệu này xác định giao diện các cổng (Ports), hợp đồng thực thể và ma trận kiểm thử tương tranh theo chuẩn TDD cho phân hệ FitCoins và Sổ Cái Bất Biến.

---

## 1. Quyết Định Kỹ Thuật & Kiến Trúc Lõi

* **Sổ Cái Bất Biến (ADR-0001)**: Mọi biến động số dư ví (`user_wallet`) bắt buộc phải ghi kèm 1 dòng bản ghi append-only vào `coin_ledger` trong cùng một Database Transaction. Nghiêm cấm `UPDATE` hoặc `DELETE` trên `coin_ledger`.
* **Trần Kiểm Soát Daily Caps $O(1)$ (ADR-0002)**: Lưu trữ ngày nhận thưởng gần nhất (`last_workout_reward_date`, `last_pr_reward_date`, `last_nutrition_reward_date`) trực tiếp trên bảng `user_wallet`. Chặn lạm phát tại tầng Domain $O(1)$ mà không cần cron job quét DB lúc nửa đêm.
* **Cổng Chi Tiêu Tách Rời Shop (ADR-0003)**: `SpendCoins` là một generic seam tiếp nhận `(amount, reason, idempotency_key)`. Module Gamification đóng vai trò Ngân hàng/Ví, không hardcode danh mục vật phẩm hay bảng giá Shop.
* **Cổng Thưởng Nội Bộ Đồng Bộ Kết Hợp Consumer Bất Đồng Bộ (ADR-0004)**: Cung cấp Input Port `WalletRewarder` cho phép các use case nội bộ (`ActivityIngestionCoordinator`, mở khóa thành tựu) gọi cộng coin tức thì ($0\text{ ms}$ lag) trong cùng Database Transaction, đồng thời hỗ trợ Kafka Consumers cho các sự kiện từ các Bounded Context bên ngoài.
* **Transactional Outbox Atomic Integration**: Khi nạp hoặc trừ coin thành công, ghi nhận CloudEvents `CoinsEarned` hoặc `CoinsSpent` vào bảng `gamification.outbox` trong cùng transaction với ví và sổ cái. Hạ tầng Outbox tuân thủ tài liệu chuẩn duy nhất (SSOT) tại [architecture.md#4](../../architecture.md#4-chuẩn-transactional-outbox-dùng-chung-module-wide-outbox---ssot).

---

## 2. Hợp Đồng Giao Diện Cốt Lõi (Core Interfaces & Signatures)

### 2.1 Domain Layer
```go
// vo/coin_transaction_reason.go
type CoinTransactionReason string
const (
    ReasonEarnLevelUp         CoinTransactionReason = "EARN_LEVEL_UP"
    ReasonEarnWorkout         CoinTransactionReason = "EARN_WORKOUT"
    ReasonEarnPR              CoinTransactionReason = "EARN_PR"
    ReasonEarnNutrition       CoinTransactionReason = "EARN_NUTRITION"
    ReasonEarnStreakMilestone CoinTransactionReason = "EARN_STREAK_MILESTONE"
    ReasonEarnOnboarding      CoinTransactionReason = "EARN_ONBOARDING"
    ReasonEarnAchievement     CoinTransactionReason = "EARN_ACHIEVEMENT"
    ReasonSpendGeneric        CoinTransactionReason = "SPEND_GENERIC"
)

// entity/coin_ledger_entry.go
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

// aggregate/user_wallet.go: Quản lý số dư và Daily Caps
type UserWallet struct { /* unexported fields */ }
func NewUserWallet(userID uuid.UUID) *UserWallet
func (w *UserWallet) EarnFromLevelUp(level int32) (int32, error)
func (w *UserWallet) EarnFromWorkout(rewardDate string) (int32, error)     // Format "2006-01-02"
func (w *UserWallet) EarnFromPR(rewardDate string) (int32, error)          // Format "2006-01-02"
func (w *UserWallet) EarnFromNutrition(rewardDate string) (int32, error)   // Format "2006-01-02"
func (w *UserWallet) EarnFromStreakMilestone(milestoneDays int32) (int32, error)
func (w *UserWallet) EarnFromAchievement(tier string, customAmount int32) (int32, error)
func (w *UserWallet) Spend(amount int64, reason string) error
```

### 2.2 Output Ports & Use Cases (Application Layer)
```go
// port/tx.go: Quản lý ranh giới giao dịch cơ sở dữ liệu
type TransactionManager interface {
    WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

// port/wallet_repository.go: Output Port thao tác DB
type WalletRepository interface {
    GetWalletForUpdate(ctx context.Context, userID uuid.UUID) (*aggregate.UserWallet, error)
    SaveWalletAndLedger(ctx context.Context, wallet *aggregate.UserWallet, entry *entity.CoinLedgerEntry) error
    CheckIdempotency(ctx context.Context, idempotencyKey string) (bool, error)
    GetBalance(ctx context.Context, userID uuid.UUID) (int64, error)
    ListTransactions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]entity.CoinLedgerEntry, int64, error)
}

// port/wallet_rewarder.go: Input Port thưởng coin nội bộ đồng bộ (ADR-0004)
type AwardCoinsCommand struct {
    UserID         uuid.UUID
    Amount         int32
    Reason         vo.CoinTransactionReason
    SourceEventID  string
    IdempotencyKey string
    Metadata       map[string]any
}

type AwardCoinsResult struct {
    NewBalance     int64
    LedgerEntryID  uuid.UUID
    AlreadyAwarded bool
}

type WalletRewarder interface {
    AwardCoins(ctx context.Context, cmd AwardCoinsCommand) (*AwardCoinsResult, error)
}

// command/spend_coins.go: Bọc trong txManager.WithTransaction để chống race condition
type SpendCoinsCommand struct {
    UserID         uuid.UUID
    Amount         int64
    Reason         string
    IdempotencyKey string
}
type SpendCoinsHandler interface {
    Handle(ctx context.Context, cmd SpendCoinsCommand) (*SpendCoinsResult, error)
}
```

---

## 3. Ma Trận Kiểm Thử TDD & Concurrency Verification (The 3 AM Test)

| Hạng Mục Kiểm Thử | File Test | Kịch Bản Trọng Tâm |
| :--- | :--- | :--- |
| **Domain Invariants** | `domain/aggregate/user_wallet_test.go` | - Thưởng thăng cấp: Thường $+10$, mốc tròn chục $+50$, mốc 100 $+200$.<br>- Thưởng thành tựu: Bậc Đồng $+10$, Bạc $+25$, Vàng $+50$, Bạch Kim $+100$, Kim Cương $+250$.<br>- Daily Cap: Buổi tập lần 1 trong ngày $+5$; lần 2 cùng ngày trả `ErrDailyCapExceeded`; sang hôm sau $+5$ thành công.<br>- Chi tiêu: Số dư $100$, tiêu $60 \rightarrow$ còn $40$; tiêu $50$ khi còn $40 \rightarrow$ lỗi `ErrInsufficientBalance`; tiêu số âm $\rightarrow$ lỗi `ErrNegativeSpendAmount`. |
| **Ledger Balancing** | `domain/entity/coin_ledger_entry_test.go` | - Khẳng định phương trình cân bằng: `BalanceAfter = BalanceBefore + Amount`.<br>- Báo lỗi nếu `amount == 0` hoặc `idempotency_key == ""`. |
| **In-Process Rewarder & Idempotency** | `application/command/award_coins_test.go` | - Gọi `AwardCoins` lần 1 $\rightarrow$ cộng coin, ghi ledger, trả `AlreadyAwarded = false`.<br>- Gọi `AwardCoins` lần 2 cùng `idempotency_key` $\rightarrow$ giữ nguyên số dư, không ghi đúp ledger, trả `AlreadyAwarded = true`.<br>- Chạy trong transaction cha (`txCtx`): Dùng chung transaction, không mở transaction lồng nhau. |
| **Idempotency & Rollback** | `application/command/spend_coins_test.go` | - Trùng `idempotency_key` $\rightarrow$ trả về kết quả cũ, tuyệt đối không trừ coin lần 2.<br>- Ghi Outbox `CoinsSpent` khi chi tiêu thành công. |
| **Anti-Double-Spending** | `infrastructure/persistence/postgres/wallet_repository_test.go` | - **The 3 AM Test**: Khởi tạo ví 100 coin. Bắn đồng thời 2 Goroutines cùng gọi `Spend(60)` $\rightarrow$ đúng 1 bên thành công, bên kia thất bại do không đủ tiền; số dư cuối cùng đúng 40 coin (Zero Double Spending).<br>- **DB Constraint**: Lệnh SQL trừ âm số dư bị PostgreSQL chặn đứng qua `CHECK (balance >= 0)`. |

### Lệnh Thực Thi Kiểm Thử
```bash
# 1. Chạy Unit Tests Domain & Application
go test -v -race ./internal/gamification/domain/... ./internal/gamification/application/...

# 2. Chạy Integration Tests tương tranh PostgreSQL (Khóa bi quan)
go test -v -race -tags=integration ./internal/gamification/infrastructure/persistence/postgres/...

# 3. Kiểm tra Test Coverage (Mục tiêu >= 90%)
go test -coverprofile=coverage.out ./internal/gamification/domain/...
go tool cover -func=coverage.out
```
