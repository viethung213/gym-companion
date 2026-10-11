# Implementation Blueprint & LLD: XP & Level Progression

## 1. Coding Philosophy & Design Patterns
- **Hexagonal Architecture (Ports & Adapters)**: Tầng Domain (`internal/gamification/domain/`) hoàn toàn cô lập, không chứa dependency bên ngoài (không import GORM, Gin, json tag, db tag).
- **Domain-Driven Design (DDD)**:
  - Aggregate Root `UserXp` đóng gói toàn bộ trạng thái điểm `xp` và `level`, kiểm soát các biến động qua các method nghiệp vụ rõ ràng (`ApplyWorkoutResult`, `ApplyNutritionBonus`).
  - Value Object `LevelCalculator` tính toán cấp độ $1..100$ thuần túy từ `xp`.
- **Domain Service**: `XpCalculator` là service tính toán thuần túy (Stateless / Pure Functions), tách biệt thuật toán tính điểm thưởng khỏi Aggregate.
- **Explicit Domain Errors**: Sử dụng các sentinel errors định danh rõ ràng (`ErrNutritionRewardAlreadyClaimedToday`, `ErrInvalidWorkoutSession`, `ErrDuplicateWorkoutSession`).
- **Pessimistic Locking & Idempotent Ledger**: Bảo đảm tính toàn vẹn vật lý (The 3 AM Test) bằng khóa dòng `SELECT ... FOR UPDATE` và ràng buộc duy nhất `uq_xp_history_workout_session` trên bảng `xp_history` (ADR-0002).

---

## 2. Low-Level Code Design & Signatures

### 2.1 Domain Layer (`internal/gamification/domain/`)

#### Value Object: `vo/level_calculator.go`
```go
package vo

import "math"

// CalculateLevel tính level (1..100) và số XP cần để lên cấp kế tiếp
func CalculateLevel(xp int64) (level int32, xpNeeded int64) {
    if xp <= 0 {
        return 1, 100
    }
    
    // level = floor(sqrt(xp / 50)) + 1
    lvl := int32(math.Floor(math.Sqrt(float64(xp)/50.0))) + 1
    if lvl >= 100 {
        return 100, 0
    }
    
    // Ngưỡng XP của level tiếp theo: nextThreshold = 50 * lvl^2
    nextThreshold := int64(50) * int64(lvl) * int64(lvl)
    xpNeeded = nextThreshold - xp
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
)

type UserXp struct {
    userID                  uuid.UUID
    xp                      int64
    level                   int32
    lastWorkoutAt           *time.Time
    lastNutritionRewardDate *time.Time
    createdAt               time.Time
    updatedAt               time.Time
    domainEvents            []any
}

func NewUserXp(userID uuid.UUID) *UserXp
func ReconstituteUserXp(userID uuid.UUID, xp int64, level int32, lastWorkoutAt, lastNutritionRewardDate *time.Time, createdAt, updatedAt time.Time) (*UserXp, error)

// ApplyWorkoutResult cộng XP buổi tập và đánh giá Level Up
func (u *UserXp) ApplyWorkoutResult(deltaXp int32, workoutTime time.Time) error

// ApplyNutritionBonus cộng 30 XP dinh dưỡng hàng ngày
func (u *UserXp) ApplyNutritionBonus(bonus int32, localDate time.Time) error

func (u *UserXp) UserID() uuid.UUID
func (u *UserXp) Xp() int64
func (u *UserXp) Level() int32
func (u *UserXp) LastWorkoutAt() *time.Time
func (u *UserXp) LastNutritionRewardDate() *time.Time
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

---

### 2.2 Infrastructure Layer (`internal/gamification/infrastructure/`)

#### GORM Persistence Model: `persistence/postgres/model.go`
```go
package postgres

import (
    "time"
    "github.com/google/uuid"
)

type UserXpModel struct {
    UserID                  uuid.UUID  `gorm:"type:uuid;primaryKey;column:user_id"`
    Xp                      int64      `gorm:"type:bigint;not null;default:0;column:xp"`
    Level                   int32      `gorm:"type:integer;not null;default:1;column:level"`
    LastWorkoutAt           *time.Time `gorm:"column:last_workout_at"`
    LastNutritionRewardDate *time.Time `gorm:"type:date;column:last_nutrition_reward_date"`
    CreatedAt               time.Time  `gorm:"column:created_at"`
    UpdatedAt               time.Time  `gorm:"column:updated_at"`
}

func (UserXpModel) TableName() string {
    return "gamification.user_xp"
}

type XpHistoryModel struct {
    ID             uuid.UUID `gorm:"type:uuid;primaryKey;column:id"`
    UserID         uuid.UUID `gorm:"type:uuid;not null;column:user_id;index"`
    OldXp          int64     `gorm:"type:bigint;not null;column:old_xp"`
    NewXp          int64     `gorm:"type:bigint;not null;column:new_xp"`
    DeltaXp        int32     `gorm:"type:integer;not null;column:delta_xp"`
    Reason         string    `gorm:"type:varchar(50);not null;column:reason"`
    SourceEventID  *string   `gorm:"type:varchar(100);column:source_event_id"`
    Metadata       string    `gorm:"type:jsonb;column:metadata"`
    CreatedAt      time.Time `gorm:"column:created_at"`
}

func (XpHistoryModel) TableName() string {
    return "gamification.xp_history"
}
```

---

### 2.3 Application Layer (`internal/gamification/application/`)

#### Command: `command/process_workout_xp.go`
```go
package command

import (
    "context"
    "time"
    "github.com/google/uuid"
)

type ProcessWorkoutXpCommand struct {
    UserID       uuid.UUID
    SessionID    string
    ActualVolume float64
    TargetVolume float64
    FormScore    float64
    IsPR         bool
    StreakDays   int32
    CompletedAt  time.Time
}

type ProcessWorkoutXpHandler interface {
    Handle(ctx context.Context, cmd ProcessWorkoutXpCommand) error
}
```

#### Query: `query/get_my_xp.go`
```go
package query

import (
    "context"
    "github.com/google/uuid"
)

type GetMyXpQuery struct {
    UserID uuid.UUID
}

type MyXpDTO struct {
    UserID        uuid.UUID
    Xp            int64
    Level         int32
    XpToNextLevel int64
}

type GetMyXpHandler interface {
    Handle(ctx context.Context, qry GetMyXpQuery) (*MyXpDTO, error)
}
```

---

## 3. Core Algorithms & Formula Implementation

### Thuật toán tính $\Delta XP$ buổi tập
```go
func (s *xpCalculatorImpl) CalculateWorkoutXp(p WorkoutXpParams) int32 {
    baseXp := 50.0

    // Volume bonus [10, 30]
    volumeRatio := 1.0
    if p.TargetVolume > 0 {
        volumeRatio = p.ActualVolume / p.TargetVolume
    }
    volBonus := math.Round(25.0 * volumeRatio)
    if volBonus < 10.0 { volBonus = 10.0 }
    if volBonus > 30.0 { volBonus = 30.0 }

    // Form bonus [0, 20]
    formBonus := math.Round(20.0 * (p.FormScore / 100.0))
    if formBonus < 0.0 { formBonus = 0.0 }
    if formBonus > 20.0 { formBonus = 20.0 }

    // PR bonus: 25
    prBonus := 0.0
    if p.IsPR {
        prBonus = 25.0
    }

    // Streak Multiplier: 1.1x (>=3 days), 1.2x (>=7 days)
    mult := 1.0
    if p.StreakDays >= 7 {
        mult = 1.2
    } else if p.StreakDays >= 3 {
        mult = 1.1
    }

    subTotal := baseXp + volBonus + formBonus + prBonus
    return int32(math.Round(subTotal * mult))
}
```

---

## 4. Test-Driven Development (TDD) Strategy

### Kịch Bản Kiểm Thử Trọng Tâm

| Bộ Test | Đường Dẫn | Kịch Bản Kiểm Thử Trọng Tâm |
| :--- | :--- | :--- |
| **Cấp Độ** | `domain/vo/level_calculator_test.go` | - $0$ XP $\rightarrow$ Level 1, cần 100 XP.<br>- $100$ XP $\rightarrow$ Level 2.<br>- $250$ XP $\rightarrow$ Level 3.<br>- Kẹp trần tối đa Level 100, `xpNeeded = 0`. |
| **Thuật Toán XP** | `domain/service/xp_calculator_test.go` | - Buổi tập chuẩn $\rightarrow$ dao động $70..125$ XP.<br>- PR phá kỷ lục $\rightarrow$ cộng thêm đúng 25 XP.<br>- Chuỗi 7 ngày $\rightarrow$ nhân $1.2\times$.<br>- Dinh dưỡng đạt chuẩn $\rightarrow$ đúng 30 XP. |
| **Vòng Đời Aggregate** | `domain/aggregate/user_xp_test.go` | - Khởi tạo mặc định: 0 XP, Level 1.<br>- Tăng XP qua mốc $\rightarrow$ Tự tăng `level` + phát `UserLeveledUp`.<br>- Dinh dưỡng nhận lần 1 thành công; nhận lần 2 cùng ngày trả lỗi `ErrNutritionRewardAlreadyClaimedToday`. |
| **Khóa & Toàn Vẹn** | `infrastructure/persistence/postgres/repository_test.go` | - **The 3 AM Test**: 10 Goroutines đồng thời gọi `GetForUpdate` cộng điểm cho 1 user $\rightarrow 0\%$ Lost Update, tổng XP đúng 100% (ADR-0002).<br>- **Idempotency**: Gửi 2 lần cùng một `session_id` $\rightarrow$ lần 2 rollback vi phạm `uq_xp_history_workout_session`, không cộng trùng. |

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
