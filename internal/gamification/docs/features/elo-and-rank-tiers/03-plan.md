# Implementation Blueprint & LLD: ELO Rating & Rank Tiers

## 1. Coding Philosophy & Design Patterns
- **Hexagonal Architecture (Ports & Adapters)**: Tầng Domain (`internal/gamification/domain/`) hoàn toàn cô lập, không chứa dependency bên ngoài (không import GORM, Gin, json tag, db tag).
- **Domain-Driven Design (DDD)**:
  - Aggregate Root `UserElo` đóng gói toàn bộ trạng thái và kiểm soát các biến động điểm. Mọi thay đổi trạng thái phải qua các method nghiệp vụ rõ ràng (`ApplyWorkoutResult`, `ApplyNutritionBonus`, `ApplyInactivityDecay`).
  - Value Object `RankTier` bất biến, cung cấp hàm ánh xạ thuần túy `DetermineRankTier(elo int32) RankTier`. Bậc hạng không lưu trạng thái độc lập mà được tính toán động (Stateless Mapping - ADR-0003).
- **Domain Service**: `EloCalculator` là service tính toán thuần túy (Stateless / Pure Functions), tách biệt thuật toán tính điểm khỏi Aggregate.
- **Explicit Domain Errors**: Sử dụng các sentinel errors định danh rõ ràng (`ErrEloOutOfRange`, `ErrNutritionRewardAlreadyClaimedToday`, `ErrInvalidWorkoutSession`, `ErrDuplicateWorkoutSession`) thay vì generic errors.
- **Pessimistic Locking & Idempotent Ledger**: Bảo đảm tính toàn vẹn vật lý (The 3 AM Test) bằng khóa dòng `SELECT ... FOR UPDATE` và ràng buộc duy nhất `uq_elo_history_workout_session` trên bảng `elo_history` (ADR-0004).

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

// DetermineRankTier ánh xạ điểm ELO sang Bậc Hạng tương ứng theo ADR-0003
func DetermineRankTier(elo int32) RankTier

// GetTierThreshold trả về ngưỡng điểm tối thiểu để đạt bậc hạng
func GetTierThreshold(tier RankTier) int32

// GetNextTierThreshold trả về ngưỡng điểm cần đạt để lên bậc tiếp theo (trả về 3000 nếu là Diamond)
func GetNextTierThreshold(elo int32) (nextTier RankTier, threshold int32, pointsNeeded int32)
```

#### Domain Service: `service/elo_calculator.go`
```go
package service

import "github.com/viethung213/gym-companion/internal/gamification/domain/vo"

type WorkoutPerformanceParams struct {
    CurrentTier  vo.RankTier
    ActualVolume float64
    TargetVolume float64
    FormScore    float64 // 0..100 (nhận từ payload của workout_execution)
    IsPR         bool
}

type EloCalculator interface {
    CalculateWorkoutDelta(params WorkoutPerformanceParams) int32
    CalculateNutritionBonus() int32 // Cố định +3 ELO/ngày
    CalculateInactivityDecay() int32
    GetKFactor(tier vo.RankTier) float64
}

type eloCalculatorImpl struct{}

func NewEloCalculator() EloCalculator
```

#### Aggregate Root: `aggregate/user_elo.go`
```go
package aggregate

import (
    "time"
    "github.com/google/uuid"
    "github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

type UserElo struct {
    userID                   uuid.UUID
    currentElo               int32
    peakElo                  int32
    lastWorkoutAt            *time.Time
    lastDecayAt              *time.Time
    lastNutritionRewardDate  *time.Time // Ngày địa phương nhận thưởng dinh dưỡng
    createdAt                time.Time
    updatedAt                time.Time
    domainEvents             []any
}

func NewUserElo(userID uuid.UUID) *UserElo
func ReconstituteUserElo(userID uuid.UUID, currentElo, peakElo int32, lastWorkoutAt, lastDecayAt, lastNutritionRewardDate *time.Time, createdAt, updatedAt time.Time) (*UserElo, error)

func (u *UserElo) ApplyWorkoutResult(deltaElo int32, workoutTime time.Time) error
func (u *UserElo) ApplyNutritionBonus(bonus int32, localDate time.Time) error
func (u *UserElo) ApplyInactivityDecay(decayPoints int32, decayTime time.Time) error

func (u *UserElo) UserID() uuid.UUID
func (u *UserElo) CurrentElo() int32
func (u *UserElo) RankTier() vo.RankTier // Pure mapping qua vo.DetermineRankTier(u.currentElo)
func (u *UserElo) PeakElo() int32
func (u *UserElo) LastNutritionRewardDate() *time.Time
func (u *UserElo) PopDomainEvents() []any
```

#### Repository Port: `repository/user_elo_repository.go`
```go
package repository

import (
    "context"
    "github.com/google/uuid"
    "github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
)

type UserEloRepository interface {
    FindByID(ctx context.Context, userID uuid.UUID) (*aggregate.UserElo, error)
    GetForUpdate(ctx context.Context, userID uuid.UUID) (*aggregate.UserElo, error)
    Save(ctx context.Context, userElo *aggregate.UserElo) error
}
```

### 2.2 Persistence & Infrastructure Layer (`internal/gamification/infrastructure/persistence/postgres/`)

#### Model & Data Mapper: `model.go` & `mapper.go`
```go
type UserEloModel struct {
    UserID                  uuid.UUID  `gorm:"primaryKey;type:uuid"`
    CurrentElo              int32      `gorm:"not null;default:1000"`
    PeakElo                 int32      `gorm:"not null;default:1000"`
    LastWorkoutAt           *time.Time
    LastDecayAt             *time.Time
    LastNutritionRewardDate *time.Time `gorm:"type:date"`
    CreatedAt               time.Time
    UpdatedAt               time.Time
}

func (m *UserEloModel) ToDomain() (*aggregate.UserElo, error)
func ToPersistence(agg *aggregate.UserElo) *UserEloModel
```

#### Repositories & Transaction Control:
- `user_elo_repository.go`: Thực thi `SELECT ... FOR UPDATE` trong context giao dịch.
- `elo_history_repository.go`: `AppendHistory(ctx context.Context, record EloHistoryRecord) error` (chặn trùng `session_id` qua unique index `uq_elo_history_workout_session`).
- `outbox_repository.go`: `StoreOutboxEvent(ctx context.Context, event OutboxEvent) error`.

### 2.3 Application Layer (`internal/gamification/application/`)

```go
// Commands
type ProcessWorkoutEloCommand struct {
    EventID      string
    SessionID    string
    UserID       uuid.UUID
    ActualVolume float64
    TargetVolume float64
    FormScore    float64
    IsPR         bool
    CompletedAt  time.Time
}

type ProcessNutritionEloCommand struct {
    EventID       string
    UserID        uuid.UUID
    UserLocalDate time.Time
    CaloriesHit   bool
    ProteinHit    bool
}

// Queries
type GetMyEloQuery struct {
    UserID uuid.UUID
}

type GetMyEloDTO struct {
    UserID            uuid.UUID
    CurrentElo        int32
    RankTier          string
    PeakElo           int32
    NextTierThreshold int32
    PointsToNextTier  int32
    UpdatedAt         time.Time
}
```

---

## 3. Algorithmic Logic & Edge Nuances (Small Details)

### 3.1 Quy Cách Thuật Toán ELO Buổi Tập
1. **$K$-Factor Scaling Theo Bậc (Anti-Inflation)**:
   - Bronze, Silver: $K = 32$ (Tăng trưởng nhanh, tạo hưng phấn ban đầu).
   - Gold: $K = 24$ (Ổn định dần).
   - Platinum: $K = 16$ (Yêu cầu phong độ cao).
   - Diamond: $K = 10$ (Bậc tinh anh; chặn leo rank thiếu kiểm soát).

2. **Công Thức Tính Hiệu Suất (Performance Score)**:
   $$\text{VolumeRatio} = \min\left(\frac{V_{\text{actual}}}{V_{\text{target}}}, 1.2\right)$$
   $$\text{FormScoreRatio} = \frac{\text{FormScore}}{100.0} \quad (\text{Nhận từ payload sự kiện của } \text{workout\_execution})$$
   $$\text{PRBonus} = 1.0 \text{ nếu } \text{isPR} = \text{true}, \text{ ngược lại } 0.0$$
   $$\text{PerformanceScore} = 0.5 \cdot \text{VolumeRatio} + 0.3 \cdot \text{FormScoreRatio} + 0.2 \cdot \text{PRBonus} - 0.5$$

3. **Biên Độ Biến Động Kẹp Cứng & Trần/Sàn (ADR-0005)**:
   $$\Delta ELO = \text{clamp}\left(\text{round}(K \cdot \text{PerformanceScore}), -25, +40\right)$$
   $$\text{new\_elo} = \min\left(\max\left(\text{current\_elo} + \Delta ELO, 1000\right), 3000\right)$$
   $$\text{new\_peak} = \max\left(\text{current\_peak}, \text{new\_elo}\right)$$

### 3.2 Quy Cách Thưởng Dinh Dưỡng Hàng Ngày (ADR-0001)
- **Điều kiện**: Calo đạt $\pm 10\%$ mục tiêu VÀ Protein đạt $\ge 90\%$ mục tiêu.
- **Mức thưởng**: Cố định $+3$ ELO/ngày.
- **Chặn trùng lặp trong ngày**: So khớp `last_nutrition_reward_date == user_local_date`. Nếu đã nhận trong ngày, bỏ qua an toàn.
- **Không phạt**: Ăn lệch mục tiêu hoặc quên ghi log tuyệt đối không bị trừ điểm.

### 3.3 Quy Cách Suy Giảm Điểm Do Bất Hoạt (Inactivity Decay)
- **Điều kiện**: `current_elo > 1000` VÀ `last_workout_at < NOW() - INTERVAL '14 days'` VÀ (`last_decay_at IS NULL` HOẶC `last_decay_at < NOW() - INTERVAL '7 days'`).
- **Mức phạt**: Trừ $15$ ELO mỗi 7 ngày (`peak_elo` không đổi).
- **Ngưỡng sàn**: $\text{new\_elo} = \max(\text{current\_elo} - 15, 1000)$ (Bậc Bronze không bao giờ bị trừ dưới 1000).

### 3.4 Bảng Ánh Xạ Mã Lỗi (Error Mapping Table)
| Domain Error | ConnectRPC Code | HTTP Status | Mô Tả |
| :--- | :--- | :---: | :--- |
| `ErrEloOutOfRange` | `CodeInvalidArgument` | 400 | Điểm ELO nằm ngoài khoảng $[1000, 3000]$. |
| `ErrNutritionAlreadyClaimed` | `CodeAlreadyExists` | 409 | Thưởng dinh dưỡng đã được nhận trong ngày. |
| `ErrUserEloNotFound` | `CodeNotFound` | 404 | Không tìm thấy hồ sơ người dùng. |
| `ErrDuplicateWorkoutSession` | `CodeAlreadyExists` | 409 | Buổi tập session_id đã được xử lý (Lũy đẳng). |

---

## 4. Lean Test Design (TDD Alignment)

Áp dụng phương pháp TDD tinh gọn: Tập trung $100\%$ vào kiểm thử bất biến nghiệp vụ, thuật toán và ràng buộc đồng thời vật lý. **Không viết test cho các hàm getter/setter, mapper thụ động, hoặc mock tràn lan**.

### 4 Bộ Test Cốt Lõi (Core Test Suites)
```text
┌────────────────────────────────────────────────────────────────────────┐
│ 1. rank_tier_test.go (Domain VO)                                       │
│    -> Kiểm tra logic chuyển bậc hạng tại các mốc biên (Boundary Logic) │
├────────────────────────────────────────────────────────────────────────┤
│ 2. elo_calculator_test.go (Domain Service)                             │
│    -> Kiểm tra công thức toán, K-factor, clamp [-25, +40], trần/sàn    │
├────────────────────────────────────────────────────────────────────────┤
│ 3. user_elo_test.go (Domain Aggregate)                                 │
│    -> Kiểm tra bất biến Aggregate, thăng/giáng hạng, peak_elo, chặn lặp │
├────────────────────────────────────────────────────────────────────────┤
│ 4. repository_test.go (PostgreSQL Integration)                         │
│    -> Kiểm tra khóa bi quan FOR UPDATE chống Lost Update & Idempotency │
└────────────────────────────────────────────────────────────────────────┘
```

### Kịch Bản Kiểm Thử Bắt Buộc

| Bộ Test | Đường Dẫn | Kịch Bản Kiểm Thử Trọng Tâm |
| :--- | :--- | :--- |
| **Bậc Hạng** | `domain/vo/rank_tier_test.go` | - $1000..1199 \rightarrow$ Bronze; $1200 \rightarrow$ Silver; $1500 \rightarrow$ Gold; $1800 \rightarrow$ Platinum; $2200 \rightarrow$ Diamond.<br>- Sàn cứng $1000$, trần cứng $3000$. |
| **Thuật Toán ELO** | `domain/service/elo_calculator_test.go` | - Volume & form chuẩn $\rightarrow \Delta$ chính xác theo công thức.<br>- Biến động kẹp cứng trong đoạn $[-25, +40]$.<br>- Kẹp trần 3000 ELO và sàn 1000 ELO (ADR-0005).<br>- $K$-factor suy giảm đúng bậc ($32 \rightarrow 24 \rightarrow 16 \rightarrow 10$).<br>- Thưởng dinh dưỡng cố định $+3$ ELO. |
| **Vòng Đời Aggregate** | `domain/aggregate/user_elo_test.go` | - Khởi tạo mặc định $1000$ ELO, 1000 Peak ELO, Bronze Tier.<br>- Tăng điểm qua mốc $\rightarrow$ Tự thăng hạng + phát `RankTierPromoted`, `peak_elo = max(peak_elo, new_elo)`.<br>- Giảm điểm rớt mốc $\rightarrow$ Tự giáng hạng + phát `RankTierDemoted` (ADR-0003), `peak_elo` không bị giảm.<br>- Thưởng dinh dưỡng $+3$ ELO: nhận lần 1 thành công; nhận lần 2 cùng ngày trả lỗi `ErrNutritionAlreadyClaimed` (ADR-0001).<br>- Bất hoạt $>14$ ngày $\rightarrow$ trừ $15$ ELO (không rớt dưới sàn 1000, `peak_elo` giữ nguyên). |
| **Khóa & Toàn Vẹn** | `infrastructure/persistence/postgres/repository_test.go` | - **The 3 AM Test**: 10 Goroutines đồng thời gọi `GetForUpdate` cộng điểm cho 1 user $\rightarrow 0\%$ Lost Update, điểm cuối cùng đúng $1300$ ELO (ADR-0004).<br>- **Idempotency**: Gửi 2 lần cùng một `session_id` $\rightarrow$ lần 2 rollback vi phạm `uq_elo_history_workout_session`, không cộng trùng. |

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
  - Khai báo Protobuf (`gamification_service.proto`, `gamification_messages.proto`, `elo_events.proto`).
  - Chạy `buf generate` sinh Go stubs tại `internal/gen/go/contracts/supporting/gamification/v1/`.
- **Phase 2: Database Schema & Migration (PostgreSQL DDL)**:
  - Migration script tạo schema `gamification`, bảng `user_elo` (check `[1000, 3000]`), `elo_history` (kèm index unique `session_id`), `outbox_events` kèm chỉ mục và check constraints.
- **Phase 3: Domain Core & Unit Tests (Pure Go, TDD)**:
  - Viết test trước (RED) $\rightarrow$ Hiện thực hóa Value Object, Domain Service, Aggregate Root, Repository Port (GREEN) $\rightarrow$ Tối ưu (REFACTOR).
- **Phase 4: Persistence Layer & Concurrency Integration Tests**:
  - Triển khai PostgreSQL Repository với `SELECT ... FOR UPDATE`, EloHistoryRepository (Idempotency Guard), Outbox Writer.
  - Chạy kiểm thử race condition 10 Goroutines đồng thời.
- **Phase 5: Application CQRS & Transport Wiring**:
  - Hiện thực hóa Command Handlers, Query Handlers, Kafka Consumers, và ConnectRPC Handler.
  - Background Outbox Worker quét và publish sự kiện sang Kafka.

---

## 6. Migration, Compatibility & Rollout

- **Schema Cô Lập**: Schema `gamification.*` hoàn toàn độc lập, không ảnh hưởng đến các module hiện hành.
- **Lazy Onboarding**: Người dùng mới được khởi tạo mặc định $1000$ ELO (Bronze) ngay trong lần đầu phát sinh buổi tập hoặc truy vấn điểm, không cần chạy data backfill cho người dùng cũ.
- **Rollback Safety**: Xóa bảng và schema `gamification` mà không gây ảnh hưởng đến dữ liệu tập luyện của module `workout_execution`.
