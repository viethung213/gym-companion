# Kế Hoạch Triển Khai Kỹ Thuật (Implementation Plan): XP & Level Progression

Tài liệu này xác định chiến lược triển khai mã nguồn, hợp đồng giao diện các tầng (Ports & Signatures) và ma trận kiểm thử tương tranh theo chuẩn TDD.

---

## 1. Nguyên Tắc Thiết Kế Mã Nguồn (Design Principles)

* **Hexagonal Isolation**: Tầng Domain (`internal/gamification/domain/`) là Pure Go — tuyệt đối không import ORM, framework HTTP hay JSON/DB tags.
* **Aggregate Invariants**: `UserXp` kiểm soát toàn bộ biến động điểm. Mọi thay đổi trạng thái phải qua method tường minh (`ApplyWorkoutResult`, `ApplyNutritionBonus`).
* **Stateless Domain Service**: Thuật toán tính XP buổi tập tách riêng vào `XpCalculator` thuần túy (Pure Function), không giữ trạng thái.
* **Pessimistic Concurrency**: Bảo đảm tính toàn vẹn vật lý (The 3 AM Test) bằng khóa dòng `SELECT ... FOR UPDATE` và chặn duplicate event bằng partial unique index `uq_xp_history_workout_session` (ADR-0002).

---

## 2. Hợp Đồng Giao Diện Cốt Lõi (Core Interfaces & Signatures)

### 2.1 Domain Layer
```go
// vo/level_calculator.go: Hàm tính cấp độ thuần túy 1..100
// Level(X) = min(floor(sqrt(X / 50)) + 1, 100). Khi xp <= 0: level = 1, xpNeeded = 50.
func CalculateLevel(xp int64) (level int32, xpNeeded int64)

// service/xp_calculator.go: Service tính toán delta XP
type XpCalculator interface {
    CalculateWorkoutXp(params WorkoutXpParams) int32
    CalculateNutritionBonus() int32 // Cố định +30 XP
}

// aggregate/user_xp.go: Aggregate Root
type UserXp struct { /* unexported fields */ }
func NewUserXp(userID uuid.UUID) *UserXp
func (u *UserXp) ApplyWorkoutResult(deltaXp int32, workoutTime time.Time) error
func (u *UserXp) ApplyNutritionBonus(bonus int32, rewardDate string) error // Format "2006-01-02"
func (u *UserXp) PopDomainEvents() []any // Trả về bản sao phòng vệ (defensive copy slice)

// repository/user_xp_repository.go: Output Port lưu trữ
type UserXpRepository interface {
    FindByID(ctx context.Context, userID uuid.UUID) (*aggregate.UserXp, error)
    GetForUpdate(ctx context.Context, userID uuid.UUID) (*aggregate.UserXp, error)
    Save(ctx context.Context, userXp *aggregate.UserXp) error
}
```

### 2.2 Application Layer
```go
// port/tx.go: Quản lý ranh giới giao dịch cơ sở dữ liệu
type TransactionManager interface {
    WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

// command/process_workout_xp.go: Thực thi trong WithTransaction để giữ khóa bi quan
type ProcessWorkoutXpHandler interface {
    Handle(ctx context.Context, cmd ProcessWorkoutXpCommand) error
}

// command/process_nutrition_xp.go: Thực thi trong WithTransaction
type ProcessNutritionXpHandler interface {
    Handle(ctx context.Context, cmd ProcessNutritionXpCommand) error
}

// query/get_my_xp.go
type GetMyXpHandler interface {
    Handle(ctx context.Context, qry GetMyXpQuery) (*MyXpDTO, error)
}
```

---

## 3. Ma Trận Kiểm Thử TDD & Concurrency Verification

| Phân Vùng Kiểm Thử | File Test | Kịch Bản Trọng Tâm (The 3 AM Test) |
| :--- | :--- | :--- |
| **Value Object** | `domain/vo/level_calculator_test.go` | - Level 1 ($0$ XP, `xpNeeded = 50`), Level 2 ($50$ XP, `xpNeeded = 150`), Level 3 ($200$ XP, `xpNeeded = 250`), Level 10 ($4,050$ XP).<br>- Kẹp trần Level 100 tối đa, `xpNeeded = 0`. |
| **Domain Service** | `domain/service/xp_calculator_test.go` | - Base check-in ($50$ XP) + Volume ($10..30$) + Form ($0..20$) + PR ($25$).<br>- Streak Multiplier ($1.1\times$ khi $\ge 3$ ngày, $1.2\times$ khi $\ge 7$ ngày).<br>- Dinh dưỡng cố định $+30$ XP. |
| **Aggregate Root** | `domain/aggregate/user_xp_test.go` | - Khởi tạo mặc định 0 XP, Level 1.<br>- Vượt ngưỡng XP tự động tăng `level` + sinh event `UserLeveledUp`.<br>- Nhận thưởng dinh dưỡng 2 lần cùng ngày trả lỗi `ErrNutritionRewardAlreadyClaimedToday`. |
| **Concurrency & Integrity** | `infrastructure/persistence/postgres/repository_test.go` | - **Zero Lost Update**: 10 Goroutines đồng thời gọi `GetForUpdate` cộng $+50$ XP $\rightarrow$ kết quả cuối cùng đúng $500$ XP (ADR-0002).<br>- **Idempotency Guard**: Bắn 2 lần cùng `session_id` $\rightarrow$ lần 2 rollback vi phạm `uq_xp_history_workout_session`. |

### Lệnh Thực Thi Kiểm Thử
```bash
# 1. Chạy Unit Tests Domain (nhanh, isolated)
go test -v -race ./internal/gamification/domain/...

# 2. Chạy Integration Tests PostgreSQL (tương tranh & locking)
go test -v -race -tags=integration ./internal/gamification/infrastructure/persistence/postgres/...

# 3. Kiểm tra Test Coverage (Mục tiêu >= 90%)
go test -coverprofile=coverage.out ./internal/gamification/domain/...
go tool cover -func=coverage.out
```
