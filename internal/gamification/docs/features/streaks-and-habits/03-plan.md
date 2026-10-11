# Kế Hoạch Triển Khai Kỹ Thuật (Implementation Plan): Chuỗi Ngày Kỷ Luật (Streaks Engine)

Tài liệu này xác định giao diện các cổng (Ports), hợp đồng Aggregate và ma trận kiểm thử tự động theo chuẩn TDD cho phân hệ Chuỗi Ngày (Streaks & Habits).

---

## 1. Quyết Định Kỹ Thuật & Kiến Trúc Lõi

* **Đánh Giá Lười $O(1)$ Không Dùng Cron (ADR-0001)**: Tính toán số ngày chuỗi dựa trên khoảng cách ngày địa phương $\Delta = \text{DateDiff}($ `today`, `last_active_date` $)$. Không sử dụng bất kỳ Cron Job quét database lúc nửa đêm nào.
* **Thói Quen Kép Tập Luyện & Dinh Dưỡng (ADR-0002)**: Ngày tập nâng tạ, ngày nghỉ log ăn uống chuẩn Calo/Protein đều được ghi nhận là ngày duy trì chuỗi (`active_day`). Mở ứng dụng không được tính là duy trì chuỗi.
* **Chuẩn Hóa Chuỗi Ngày "YYYY-MM-DD"**: Lưu trữ và so sánh ngày bằng chuỗi định dạng `"2006-01-02"` dựa trên múi giờ địa phương của người dùng (`user_timezone`), triệt tiêu 100% lỗi lệch ngày giữa UTC và local time đối với cột `DATE` của PostgreSQL.
* **Khóa Dòng Bi Quan Trong Transaction**: Mọi tác vụ cập nhật chuỗi được bọc trong `port.TransactionManager.WithTransaction` kết hợp `SELECT ... FOR UPDATE` trên bảng `gamification.user_streak`.
* **Transactional Outbox Integration**: Đẩy các sự kiện `StreakAdvanced`, `StreakReset`, `StreakMilestoneReached` vào bảng `gamification.outbox` trong cùng transaction với `user_streak`. Hạ tầng Outbox tuân thủ tài liệu chuẩn duy nhất (SSOT) tại [architecture.md#4](../../architecture.md#4-chuẩn-transactional-outbox-dùng-chung-module-wide-outbox---ssot).

---

## 2. Hợp Đồng Giao Diện Cốt Lõi (Core Interfaces & Signatures)

### 2.1 Domain Layer
```go
// vo/activity_type.go
type ActivityType string
const (
    ActivityWorkout   ActivityType = "WORKOUT"
    ActivityNutrition ActivityType = "NUTRITION"
)

// entity/streak_history_entry.go
type StreakHistoryEntry struct {
    ID           uuid.UUID
    UserID       uuid.UUID
    ActivityDate string // "YYYY-MM-DD"
    ActivityType ActivityType
    StreakCount  int32
    Metadata     map[string]any
    CreatedAt    time.Time
}

// aggregate/user_streak.go: Aggregate Root
type UserStreak struct { /* unexported fields */ }
func NewUserStreak(userID uuid.UUID) *UserStreak
func (s *UserStreak) MaintainDaily(todayStr string, actType ActivityType) error
func (s *UserStreak) CurrentStreak() int32
func (s *UserStreak) LongestStreak() int32
func (s *UserStreak) LastActiveDate() string
func (s *UserStreak) PopDomainEvents() []any // Trả về slice copy phòng vệ (defensive copy)
```

### 2.2 Output Ports & Use Cases (Application Layer)
```go
// port/tx.go: Quản lý ranh giới giao dịch cơ sở dữ liệu
type TransactionManager interface {
    WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

// port/streak_repository.go: Output Port thao tác DB
type StreakRepository interface {
    GetStreakForUpdate(ctx context.Context, userID uuid.UUID) (*aggregate.UserStreak, error)
    SaveStreakAndHistory(ctx context.Context, streak *aggregate.UserStreak, entries []entity.StreakHistoryEntry) error
    GetStreak(ctx context.Context, userID uuid.UUID) (*aggregate.UserStreak, error)
    GetHistory(ctx context.Context, userID uuid.UUID, year int) ([]entity.StreakHistoryEntry, error)
}

// command/maintain_streak.go: Bọc trong txManager.WithTransaction
type MaintainStreakCommand struct {
    UserID       uuid.UUID
    ActivityType vo.ActivityType
    EventTime    time.Time
    UserTimezone string // Ví dụ: "Asia/Ho_Chi_Minh"
    Metadata     map[string]any
}
type MaintainStreakHandler interface {
    Handle(ctx context.Context, cmd MaintainStreakCommand) error
}

// query/get_streak.go: Truy vấn chỉ đọc
type GetStreakQuery struct {
    UserID       uuid.UUID
    UserTimezone string
}
type StreakDTO struct {
    CurrentStreak  int32
    LongestStreak  int32
    LastActiveDate string
    IsActiveToday  bool
}
type GetStreakHandler interface {
    Handle(ctx context.Context, qry GetStreakQuery) (*StreakDTO, error)
}
```

---

## 3. Ma Trận Kiểm Thử TDD & Concurrency Verification

| Phân Vùng Kiểm Thử | File Test | Kịch Bản Trọng Tâm (The 3 AM Test) |
| :--- | :--- | :--- |
| **Domain Aggregate** | `domain/aggregate/user_streak_test.go` | - Khởi tạo mặc định: 0 streak.<br>- Tập ngày liên tiếp `D`, `D+1`: Chuỗi tăng $1 \rightarrow 2$, sinh event `StreakAdvanced`.<br>- Cùng ngày phát sinh nhiều hoạt động: Chuỗi giữ nguyên không tăng lặp (lũy đẳng).<br>- Nghỉ ngày `D+1`, quay lại ngày `D+2` ($\Delta = 2$): Chuỗi reset về 1, `longest_streak` bảo toàn kỷ lục cũ, sinh event `StreakReset`.<br>- Nghỉ nhiều ngày liên tiếp ($\Delta \ge 2$): Chuỗi reset về 1, bảo toàn kỷ lục cũ, sinh event `StreakReset`.<br>- Đạt các mốc chuỗi 7, 14, 30 ngày: Sinh event `StreakMilestoneReached`. |
| **Application Command** | `application/command/maintain_streak_test.go` | - Tính đúng ngày địa phương theo `user_timezone` (23:30 tại VN là hôm nay, không phải hôm sau theo UTC).<br>- Điều phối transaction qua `txManager.WithTransaction`.<br>- Lưu CloudEvents vào Outbox khi tăng chuỗi hoặc reset chuỗi. |
| **Application Query** | `application/query/get_streak_test.go` | - Tính ảo `is_active_today = true` nếu `last_active_date == todayStr`.<br>- Truy vấn chỉ đọc không phát sinh lock hay transaction ghi. |
| **Concurrency & Integrity** | `infrastructure/persistence/postgres/streak_repository_test.go` | - **The 3 AM Test**: Bắn đồng thời 5 Goroutines cùng ghi nhận buổi tập cho 1 user trong cùng ngày $\rightarrow$ 100% tuần tự hóa qua khóa dòng, chuỗi tăng đúng 1 lần, không trùng lặp bản ghi nhờ `uq_streak_history_user_date`. |

### Lệnh Thực Thi Kiểm Thử
```bash
# 1. Chạy Unit Tests tầng Domain & Application
go test -v -race ./internal/gamification/domain/aggregate/...
go test -v -race ./internal/gamification/application/...

# 2. Chạy Integration Tests PostgreSQL Locking
go test -v -race -tags=integration ./internal/gamification/infrastructure/persistence/...
```
