# Implementation Blueprint & LLD: XP & Weekly Leagues

## 1. Coding Philosophy & Design Patterns
- **Hexagonal Architecture (Ports & Adapters)**: Tầng Domain (`internal/gamification/domain/`) hoàn toàn cô lập, không chứa dependency bên ngoài (không import GORM, Gin, json tag, db tag).
- **Domain-Driven Design (DDD)**:
  - Aggregate Root `UserXp` đóng gói toàn bộ trạng thái điểm và kiểm soát các biến động. Mọi thay đổi trạng thái phải qua các method nghiệp vụ rõ ràng (`ApplyWorkoutResult`, `ApplyNutritionBonus`).
  - Value Object `LevelCalculator` tính toán cấp độ $1..100$ thuần túy từ `total_xp`.
  - Value Object `RankTier` bất biến định nghĩa 5 bậc giải đấu tuần (Bronze $\rightarrow$ Diamond).
- **Domain Service**: `XpCalculator` là service tính toán thuần túy (Stateless / Pure Functions), tách biệt thuật toán tính điểm thưởng khỏi Aggregate.
- **Explicit Domain Errors**: Sử dụng các sentinel errors định danh rõ ràng (`ErrNutritionRewardAlreadyClaimedToday`, `ErrInvalidWorkoutSession`, `ErrDuplicateWorkoutSession`).
- **Pessimistic Locking & Idempotent Ledger**: Bảo đảm tính toàn vẹn vật lý (The 3 AM Test) bằng khóa dòng `SELECT ... FOR UPDATE` và ràng buộc duy nhất `uq_xp_history_workout_session` trên bảng `xp_history` (ADR-0003).

---

## 2. Low-Level Code Design & Signatures

### 2.1 Domain Layer (`internal/gamification/domain/`)

#### Value Object: `vo/rank_tier.go`
```go
package vo

type RankTier string

const (
    RankTierUnspecified RankTier = "UNSPECIFIED"
    RankTierBronze      RankTier = "BRONZE"
    RankTierSilver      RankTier = "SILVER"
    RankTierGold        RankTier = "GOLD"
    RankTierPlatinum    RankTier = "PLATINUM"
    RankTierDiamond     RankTier = "DIAMOND"
)
```

#### Value Object: `vo/level_calculator.go`
```go
package vo

import "math"

// DetermineLevel tính cấp độ (1..100) và số XP cần để lên cấp tiếp theo dựa trên totalXp
func DetermineLevel(totalXp int64) (level int32, xpToNextLevel int64) {
    if totalXp <= 0 {
        return 1, 100
    }
    // Formula: level = floor(sqrt(totalXp / 50)) + 1, kẹp trần 100
    lvl := int32(math.Floor(math.Sqrt(float64(totalXp)/50.0))) + 1
    if lvl > 100 {
        return 100, 0
    }
    
    // Ngưỡng XP của level tiếp theo: nextThreshold = 50 * lvl^2
    nextThreshold := int64(50) * int64(lvl) * int64(lvl)
    xpNeeded := nextThreshold - totalXp
    if xpNeeded < 0 {
        xpNeeded = 0
    }
    return lvl, xpNeeded
}
```

#### Domain Service: `service/xp_calculator.go`
```go
package service

type WorkoutXpParams struct {
    ActualVolume float64
    TargetVolume float64
    FormScore    float64 // 0..100 (nhận từ payload workout_execution)
    IsPR         bool
    StreakDays   int32   // Số ngày chuỗi liên tục
}

type XpCalculator interface {
    CalculateWorkoutXp(params WorkoutXpParams) int32
    CalculateNutritionBonus() int32 // Cố định 30 XP
}

type xpCalculatorImpl struct{}

func NewXpCalculator() XpCalculator
```

#### Aggregate Root: `aggregate/user_xp.go`
```go
package aggregate

import (
    "time"
    "github.com/google/uuid"
    "github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

type UserXp struct {
    userID                  uuid.UUID
    totalXp                 int64
    currentLevel            int32
    weeklyXp                int32
    currentWeekNumber       string // Ví dụ: "2026-W41"
    rankTier                vo.RankTier
    lastWorkoutAt           *time.Time
    lastNutritionRewardDate *time.Time
    createdAt               time.Time
    updatedAt               time.Time
    domainEvents            []any
}

func NewUserXp(userID uuid.UUID, initialWeek string) *UserXp
func ReconstituteUserXp(userID uuid.UUID, totalXp int64, currentLevel int32, weeklyXp int32, currentWeekNumber string, rankTier vo.RankTier, lastWorkoutAt, lastNutritionRewardDate *time.Time, createdAt, updatedAt time.Time) (*UserXp, error)

// ApplyWorkoutResult cộng XP buổi tập, tự động xử lý Lazy Reset tuần và đánh giá Level Up
func (u *UserXp) ApplyWorkoutResult(deltaXp int32, workoutTime time.Time, currentWeek string) error

// ApplyNutritionBonus cộng 30 XP dinh dưỡng hàng ngày
func (u *UserXp) ApplyNutritionBonus(bonus int32, localDate time.Time, currentWeek string) error

func (u *UserXp) UserID() uuid.UUID
func (u *UserXp) TotalXp() int64
func (u *UserXp) CurrentLevel() int32
func (u *UserXp) WeeklyXp() int32
func (u *UserXp) RankTier() vo.RankTier
func (u *UserXp) CurrentWeekNumber() string
func (u *UserXp) PopDomainEvents() []any
```

#### Repository Port: `repository/user_xp_repository.go`
```go
package repository

import (
    "context"
    "github.com/google/uuid"
    "github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
)

type UserXpRepository interface {
    FindByID(ctx context.Context, userID uuid.UUID) (*aggregate.UserXp, error)
    GetForUpdate(ctx context.Context, userID uuid.UUID) (*aggregate.UserXp, error)
    Save(ctx context.Context, userXp *aggregate.UserXp) error
}
```

### 2.2 Persistence & Infrastructure Layer (`internal/gamification/infrastructure/persistence/postgres/`)

#### Model & Data Mapper: `model.go` & `mapper.go`
```go
type UserXpModel struct {
    UserID                  uuid.UUID  `gorm:"primaryKey;type:uuid"`
    TotalXp                 int64      `gorm:"not null;default:0"`
    CurrentLevel            int32      `gorm:"not null;default:1"`
    WeeklyXp                int32      `gorm:"not null;default:0"`
    CurrentWeekNumber       string     `gorm:"type:varchar(10);not null;default:''"`
    RankTier                string     `gorm:"type:varchar(20);not null;default:'BRONZE'"`
    LastWorkoutAt           *time.Time
    LastNutritionRewardDate *time.Time `gorm:"type:date"`
    CreatedAt               time.Time
    UpdatedAt               time.Time
}

func (m *UserXpModel) ToDomain() (*aggregate.UserXp, error)
func ToPersistence(agg *aggregate.UserXp) *UserXpModel
```

#### Repositories:
- `user_xp_repository.go`: Thực thi `SELECT ... FOR UPDATE` trong transaction context.
- `xp_history_repository.go`: `AppendHistory(ctx context.Context, record XpHistoryRecord) error` (bảo vệ tính lũy đẳng qua unique index `uq_xp_history_workout_session`).
- `outbox_repository.go`: `StoreOutboxEvent(ctx context.Context, event OutboxEvent) error`.

### 2.3 Application Layer (`internal/gamification/application/`)

```go
// Commands
type ProcessWorkoutXpCommand struct {
    EventID      string
    SessionID    string
    UserID       uuid.UUID
    ActualVolume float64
    TargetVolume float64
    FormScore    float64
    IsPR         bool
    CompletedAt  time.Time
}

type ProcessNutritionXpCommand struct {
    EventID       string
    UserID        uuid.UUID
    UserLocalDate time.Time
    CaloriesHit   bool
    ProteinHit    bool
}

// Queries
type GetMyXpQuery struct {
    UserID uuid.UUID
}

type GetMyXpDTO struct {
    UserID            uuid.UUID
    TotalXp           int64
    CurrentLevel      int32
    XpToNextLevel     int64
    WeeklyXp          int32
    RankTier          string
    CurrentWeekNumber string
    UpdatedAt         time.Time
}
```

---

## 3. Algorithmic Logic & Edge Nuances

### 3.1 Quy Cách Thuật Toán XP Buổi Tập
1. **Base XP**: Cố định $50$ XP.
2. **Volume XP**:
   $$\text{VolumeRatio} = \frac{\text{ActualVolume}}{\text{TargetVolume}}$$
   $$\text{VolumeXP} = \text{clamp}(\text{round}(25 \times \text{VolumeRatio}), 10, 30)$$
3. **Form Score XP**:
   $$\text{FormXP} = \text{round}\left(20 \times \frac{\text{FormScore}}{100}\right) \quad (\text{clamp } [0, 20])$$
4. **PR Bonus**: $+25$ XP nếu `isPR == true`, ngược lại $0$.
5. **Streak Multiplier**:
   - Streak $< 3$ ngày: $\times 1.0$.
   - Streak $3..6$ ngày: $\times 1.1$.
   - Streak $\ge 7$ ngày: $\times 1.2$.
6. **Tổng XP buổi tập**:
   $$\Delta XP = \text{round}( (50 + \text{VolumeXP} + \text{FormXP} + \text{PRBonus}) \times \text{Multiplier} )$$

### 3.2 Quy Cách Lazy Reset Tuần
- Định danh tuần theo chuẩn ISO 8601: `year, week := time.Now().ISOWeek()` $\rightarrow$ `fmt.Sprintf("%04d-W%02d", year, week)`.
- Nếu `user.currentWeekNumber != currentWeek`:
  1. `user.weeklyXp = 0`
  2. `user.currentWeekNumber = currentWeek`
- Sau đó mới cộng điểm `weeklyXp += deltaXp`.

---

## 4. Lean Test Design (TDD Alignment)

### 4 Bộ Test Cốt Lõi
```text
┌────────────────────────────────────────────────────────────────────────┐
│ 1. level_calculator_test.go (Domain VO)                                │
│    -> Kiểm tra công thức mốc level, kẹp trần 100, tính xpToNextLevel   │
├────────────────────────────────────────────────────────────────────────┤
│ 2. xp_calculator_test.go (Domain Service)                             │
│    -> Kiểm tra Base, Volume, Form, PR, Streak Multiplier, Dinh dưỡng   │
├────────────────────────────────────────────────────────────────────────┤
│ 3. user_xp_test.go (Domain Aggregate)                                 │
│    -> Kiểm tra bất biến Aggregate, Lazy Reset tuần, sự kiện Level Up   │
├────────────────────────────────────────────────────────────────────────┤
│ 4. repository_test.go (PostgreSQL Integration)                         │
│    -> Kiểm tra khóa bi quan FOR UPDATE chống Lost Update & Idempotency │
└────────────────────────────────────────────────────────────────────────┘
```

### Kịch Bản Kiểm Thử Trọng Tâm

| Bộ Test | Đường Dẫn | Kịch Bản Kiểm Thử Trọng Tâm |
| :--- | :--- | :--- |
| **Cấp Độ** | `domain/vo/level_calculator_test.go` | - $0$ XP $\rightarrow$ Level 1, cần 100 XP.<br>- $100$ XP $\rightarrow$ Level 2.<br>- $250$ XP $\rightarrow$ Level 3.<br>- Kẹp trần tối đa Level 100, `xpNeeded = 0`. |
| **Thuật Toán XP** | `domain/service/xp_calculator_test.go` | - Buổi tập chuẩn $\rightarrow$ dao động $70..125$ XP.<br>- PR phá kỷ lục $\rightarrow$ cộng thêm đúng 25 XP.<br>- Chuỗi 7 ngày $\rightarrow$ nhân $1.2\times$.<br>- Dinh dưỡng đạt chuẩn $\rightarrow$ đúng 30 XP. |
| **Vòng Đời Aggregate** | `domain/aggregate/user_xp_test.go` | - Khởi tạo mặc định: 0 XP, Level 1, 0 Weekly XP.<br>- Tăng XP qua mốc $\rightarrow$ Tự tăng `currentLevel` + phát `UserLeveledUp`.<br>- Sang tuần mới (khác `currentWeekNumber`) $\rightarrow$ `weeklyXp` tự động reset về 0 và cộng dồn điểm mới.<br>- Dinh dưỡng nhận lần 1 thành công; nhận lần 2 cùng ngày trả lỗi `ErrNutritionRewardAlreadyClaimedToday`. |
| **Khóa & Toàn Vẹn** | `infrastructure/persistence/postgres/repository_test.go` | - **The 3 AM Test**: 10 Goroutines đồng thời gọi `GetForUpdate` cộng điểm cho 1 user $\rightarrow 0\%$ Lost Update, tổng XP đúng 100% (ADR-0003).<br>- **Idempotency**: Gửi 2 lần cùng một `session_id` $\rightarrow$ lần 2 rollback vi phạm `uq_xp_history_workout_session`, không cộng trùng. |

### Lệnh Chạy Kiểm Thử
```bash
# 1. Chạy Unit Tests Domain (nhanh, zero I/O)
go test -v -race ./internal/gamification/domain/...

# 2. Chạy Integration Tests khóa bi quan PostgreSQL
go test -v -race -tags=integration ./internal/gamification/infrastructure/persistence/postgres/...

# 3. Kiểm tra Statement Coverage
go test -coverprofile=coverage.out ./internal/gamification/domain/...
go tool cover -func=coverage.out
```

---

## 5. Execution Phases & Sequencing

- **Phase 1: API & Event Contracts (Protobuf)**:
  - Khai báo Protobuf (`gamification_service.proto`, `gamification_messages.proto`, `xp_events.proto`).
  - Chạy `buf generate` sinh Go stubs tại `internal/gen/go/contracts/supporting/gamification/v1/`.
- **Phase 2: Database Schema & Migration (PostgreSQL DDL)**:
  - Migration script tạo schema `gamification`, bảng `user_xp`, `xp_history` (kèm index unique `session_id`), `outbox_events` kèm chỉ mục B-Tree.
- **Phase 3: Domain Core & Unit Tests (Pure Go, TDD)**:
  - Viết test trước (RED) $\rightarrow$ Hiện thực hóa Value Object, Domain Service, Aggregate Root, Repository Port (GREEN) $\rightarrow$ Tối ưu (REFACTOR).
- **Phase 4: Persistence Layer & Concurrency Integration Tests**:
  - Triển khai PostgreSQL Repository với `SELECT ... FOR UPDATE`, `XpHistoryRepository` (Idempotency Guard), Outbox Writer.
  - Chạy kiểm thử race condition 10 Goroutines đồng thời.
- **Phase 5: Application CQRS & Transport Wiring**:
  - Hiện thực hóa Command Handlers, Query Handlers, Kafka Consumers, và ConnectRPC Handler.
  - Background Outbox Worker quét và publish CloudEvents sang Kafka.
