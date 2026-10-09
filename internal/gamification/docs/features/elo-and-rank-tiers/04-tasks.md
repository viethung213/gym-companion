# Tasks: ELO Rating & Rank Tiers (TDD Approach)

Tài liệu phân rã công việc kỹ thuật tinh gọn theo chuẩn **Test-Driven Development (TDD)** (`RED -> GREEN -> REFACTOR`). Các bài kiểm thử **chỉ tập trung vào nghiệp vụ cốt lõi (Core Domain Logic & Concurrency Invariants)**, loại bỏ hoàn toàn các tầng test giả lập (Mocking boilerplate) hoặc test râu ria không chứa logic.

---

## Phase 1: Shared Prerequisites (Hợp Đồng & Cơ Sở Dữ Liệu)

- [ ] **T-01 [Contract-First]**: Khai báo các hợp đồng Protobuf cho module Gamification.
  - **Đường dẫn**:
    - `proto/contracts/supporting/gamification/v1/message/gamification_messages.proto`
    - `proto/contracts/supporting/gamification/v1/service/gamification_service.proto`
    - `proto/contracts/supporting/gamification/v1/event/elo_events.proto`
  - **Xác minh**: Chạy `buf lint` đạt $100\%$ không có cảnh báo.

- [ ] **T-02 [Code Generation]**: Sinh mã nguồn Go stubs tự động qua Buf CLI.
  - **Lệnh thực thi**: `buf generate`
  - **Xác minh**: Mã nguồn sinh ra tại `internal/gen/go/contracts/supporting/gamification/v1/` biên dịch không lỗi (`go build ./...`).

- [ ] **T-03 [Database Migration]**: Viết script migration PostgreSQL khởi tạo schema và các bảng dữ liệu cho Gamification.
  - **Đường dẫn**: `internal/shared/database/migrations/13-create-gamification-tables.sql`
  - **Nội dung**: Bảng `user_elo` (ràng buộc check `[1000, 3000]`), `elo_history`, `processed_events`, `outbox_events`.
  - **Xác minh**: Script SQL thực thi thành công trên PostgreSQL.

---

## Phase 2: Core Domain Logic (Strict TDD: RED -> GREEN -> REFACTOR)

- [ ] **T-04 [TDD: RankTier Boundary Logic]**: Ánh xạ 5 bậc hạng và logic kiểm tra mốc biên.
  - **RED**: Viết test trước tại `internal/gamification/domain/vo/rank_tier_test.go` kiểm thử chính xác các mốc biên:
    - Sàn $1000$ (Bronze), Trần $3000$ (Diamond).
    - Các mốc thăng bậc: $1199 \rightarrow 1200$ (Silver), $1499 \rightarrow 1500$ (Gold), $1799 \rightarrow 1800$ (Platinum), $2199 \rightarrow 2200$ (Diamond).
    - Khẳng định test **FAIL** (chưa có code production).
  - **GREEN**: Viết mã nguồn tối thiểu tại `internal/gamification/domain/vo/rank_tier.go` để test **PASS**.
  - **REFACTOR**: Tinh gọn biểu thức điều kiện.

- [ ] **T-05 [TDD: EloCalculator Mathematical Algorithm]**: Thuật toán tính điểm ELO theo hiệu suất tập luyện.
  - **RED**: Viết table-driven test trước tại `internal/gamification/domain/service/elo_calculator_test.go` kiểm thử:
    - Công thức hiệu suất từ volume ratio & form score ratio.
    - Fallback buổi tập không có AI (form = 0) về form mặc định $75\%$.
    - Phá kỷ lục (PR) cộng thưởng, kẹp trần biến động $+40$.
    - Tập hỏng form/bỏ bài, kẹp sàn biến động $-25$.
    - Kẹp trần cứng tối đa $3000$ ELO và sàn tối thiểu $1000$ ELO (ADR-0005).
    - Hệ số $K$ suy giảm theo Tier ($32 \rightarrow 24 \rightarrow 16 \rightarrow 10$).
    - Khẳng định test **FAIL**.
  - **GREEN**: Hiện thực hóa thuật toán tại `internal/gamification/domain/service/elo_calculator.go` để test **PASS**.
  - **REFACTOR**: Tối ưu các phép tính số thực và hằng số cấu hình.

- [ ] **T-06 [TDD: UserElo Aggregate Invariants]**: Bất biến trạng thái, thăng/giáng bậc và thưởng dinh dưỡng.
  - **RED**: Viết test trước tại `internal/gamification/domain/aggregate/user_elo_test.go` kiểm thử:
    - Khởi tạo `NewUserElo(userID)` mặc định $1000$ ELO, Bronze Tier.
    - `ApplyWorkoutResult`: Điểm vượt mốc $\rightarrow$ Tự thăng hạng và phát sinh sự kiện `RankTierPromoted`.
    - `ApplyWorkoutResult`: Điểm tụt mốc $\rightarrow$ Tự giáng hạng và phát sinh sự kiện `RankTierDemoted` (ADR-0003).
    - `ApplyNutritionBonus`: Nhận lần 1 thành công $+3$ ELO; nhận lần 2 cùng ngày trả lỗi `ErrNutritionRewardAlreadyClaimedToday` (ADR-0001).
    - `ApplyInactivityDecay`: Quá 14 ngày không tập trừ $15$ ELO (không rớt dưới sàn $1000$).
    - Khẳng định test **FAIL**.
  - **GREEN**: Hiện thực hóa Aggregate tại `internal/gamification/domain/aggregate/user_elo.go` và domain events tại `internal/gamification/domain/event/elo_events.go` để test **PASS**.
  - **REFACTOR**: Đảm bảo đóng gói bất biến tuyệt đối (zero ORM tags, zero external imports).

- [ ] **T-07 [Domain Repository Port]**: Định nghĩa Interface Port cho tầng dữ liệu.
  - **Đường dẫn**: `internal/gamification/domain/repository/user_elo_repository.go`
  - **Yêu cầu**: Khai báo `FindByID`, `GetForUpdate`, `Save`.

---

## Phase 3: PostgreSQL Concurrency & Idempotency (Physical Invariants)

- [ ] **T-08 [TDD Integration: Row Lock Concurrency & Inbox Guard]**: Kiểm tra khóa bi quan và chặn trùng lặp sự kiện.
  - **RED**: Viết integration test tại `internal/gamification/infrastructure/persistence/postgres/repository_test.go`:
    - **The 3 AM Test**: Khởi tạo user $1200$ ELO. Bắn đồng thời **10 Goroutines** cùng gọi `GetForUpdate` và cộng $+10$ ELO trong 10 transactions riêng biệt $\rightarrow$ Khẳng định **0% Lost Update**, điểm cuối cùng trong DB phải đúng tuyệt đối $1300$ (ADR-0004).
    - **Idempotency Guard**: Ghi 2 lần cùng một `event_id` $\rightarrow$ Lần 2 bị từ chối vi phạm khóa chính, transaction rollback an toàn.
    - Khẳng định test **FAIL**.
  - **GREEN**: Hiện thực hóa Data Model/Mapper (`model.go`), `PostgresUserEloRepository` với `SELECT ... FOR UPDATE`, `PostgresInboxRepository`, và `PostgresOutboxRepository` để test **PASS**.
  - **REFACTOR**: Tối ưu câu lệnh SQL và quản lý transaction context.

---

## Phase 4: Delivery, Wiring & Background Workers (Ráp Nối Hệ Thống)

*Giai đoạn này tập trung ráp nối (wiring) các Use Case và Transport layer, không viết unit test giả lập (mock theater) rườm rà.*

- [ ] **T-09 [Application Commands]**: Điều phối nghiệp vụ và transaction boundary.
  - **Đường dẫn**:
    - `internal/gamification/application/command/process_workout_elo.go` (Check Inbox $\rightarrow$ Khóa bi quan `user_elo` $\rightarrow$ Tính ELO $\rightarrow$ Ghi log $\rightarrow$ Ghi Outbox).
    - `internal/gamification/application/command/process_nutrition_elo.go` (Cộng thưởng dinh dưỡng $+3$ ELO hàng ngày).
    - `internal/gamification/application/command/process_inactivity_decay.go` (Trừ điểm bất hoạt).

- [ ] **T-10 [Application Queries]**: Đọc thông tin ELO và lịch sử.
  - **Đường dẫn**:
    - `internal/gamification/application/query/get_my_elo.go` (Lấy ELO và tính `points_to_next_tier`).
    - `internal/gamification/application/query/get_elo_history.go` (Lấy lịch sử phân trang).

- [ ] **T-11 [Transport Consumers & ConnectRPC API]**: Cổng giao tiếp ngoại vi.
  - **Đường dẫn**:
    - `internal/gamification/transport/consumer/workout_event_consumer.go` (Nhận CloudEvent `WorkoutSessionCompleted`).
    - `internal/gamification/transport/consumer/nutrition_event_consumer.go` (Nhận CloudEvent `MealLogged`).
    - `internal/gamification/transport/grpc/gamification_handler.go` (Phục vụ gRPC API theo Protobuf stubs).

- [ ] **T-12 [Transactional Outbox Worker]**: Quét và bắn sự kiện ra Kafka.
  - **Đường dẫn**: `internal/gamification/infrastructure/worker/outbox_worker.go`
  - **Yêu cầu**: Background worker định kỳ đọc `outbox_events` bằng `SELECT ... FOR UPDATE SKIP LOCKED` và publish tới Kafka topic `gamification.events`.

---

## Phase 5: Verification, Race Check & Coverage Audit

- [ ] **T-13 [TDD Audit & Race Verification]**:
  - Chạy toàn bộ test suite kèm cờ phát hiện race condition:
    ```bash
    go test -v -race -coverprofile=coverage.out ./internal/gamification/...
    ```
  - Kiểm tra độ bao phủ statement coverage:
    - Domain Layer (`internal/gamification/domain/...`): $> 90\%$
    - Toàn module Gamification: $> 80\%$
  - Chạy linter: `golangci-lint run ./internal/gamification/...`.
  - Đối chiếu tiêu chí nghiệm thu [01-spec.md](./01-spec.md) và báo cáo hoàn thành.
