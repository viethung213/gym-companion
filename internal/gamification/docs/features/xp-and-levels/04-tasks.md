# Tasks: XP & Level Progression (Lean TDD Checklist)

Danh sách công việc kỹ thuật tinh gọn theo chuẩn **Test-Driven Development (TDD)** (`RED -> GREEN -> REFACTOR`). Các bài kiểm thử **chỉ tập trung vào nghiệp vụ cốt lõi (Domain Invariants & Concurrency Safety)**.

---

## Phase 1: Shared Prerequisites (Hợp Đồng & Cơ Sở Dữ Liệu)

- [ ] **T-01 [Contract-First]**: Khai báo các hợp đồng Protobuf cho module Gamification (XP & Level Progression).
  - **Đường dẫn**:
    - `proto/contracts/supporting/gamification/v1/message/gamification_messages.proto`
    - `proto/contracts/supporting/gamification/v1/service/gamification_service.proto`
    - `proto/contracts/supporting/gamification/v1/event/gamification_events.proto`
  - **Xác minh**: Chạy `buf lint` đạt $100\%$ không có cảnh báo.

- [ ] **T-02 [Code Generation]**: Sinh mã nguồn Go stubs tự động qua Buf CLI.
  - **Lệnh thực thi**: `buf generate`
  - **Xác minh**: Mã nguồn sinh ra tại `internal/gen/go/contracts/supporting/gamification/v1/` biên dịch thành công (`go build ./...`).

- [ ] **T-03 [Database Migration, BR-XP-05]**: Viết script migration PostgreSQL khởi tạo schema và các bảng dữ liệu cho Gamification.
  - **Đường dẫn**: `internal/shared/database/migrations/13-create-gamification-tables.sql`
  - **Nội dung**: Schema `gamification`, bảng `user_xp (user_id, xp, level, updated_at)`, `xp_history` (kèm unique index `uq_xp_history_workout_session`), `outbox_events` kèm chỉ mục B-Tree.
  - **Xác minh**: Script SQL thực thi thành công trên PostgreSQL.

---

## Phase 2: Core Domain Logic (Strict TDD: RED -> GREEN -> REFACTOR)

- [ ] **T-04 [TDD, FR-XP-04, BR-XP-04, ADR-0001]**: Thuật toán tính cấp độ $1..100$ và XP cần để lên cấp (Value Object).
  - **RED**: Viết test trước tại `internal/gamification/domain/vo/level_calculator_test.go` kiểm thử:
    - Mốc Level 1 ($0$ XP), Level 2 ($100$ XP), Level 3 ($250$ XP), Level 10 ($4,050$ XP).
    - Kẹp trần Level 100, `xpToNextLevel = 0`.
    - Khẳng định test **FAIL**.
  - **GREEN**: Hiện thực hóa hàm thuần túy tại `internal/gamification/domain/vo/level_calculator.go` để test **PASS**.
  - **REFACTOR**: Tối ưu biểu thức toán học.

- [ ] **T-05 [TDD, FR-XP-02, FR-XP-03, BR-XP-02, BR-XP-03, ADR-0003]**: Thuật toán tính điểm thưởng XP theo hiệu suất và dinh dưỡng (Domain Service).
  - **RED**: Viết table-driven test trước tại `internal/gamification/domain/service/xp_calculator_test.go` kiểm thử:
    - Base XP ($50$), Volume XP ($10..30$), Form Score XP ($0..20$), PR Bonus ($25$).
    - Streak Multiplier: $1.0\times$ (streak $< 3$), $1.1\times$ (streak $3..6$), $1.2\times$ (streak $\ge 7$).
    - Thưởng dinh dưỡng cố định $+30$ XP/ngày.
    - Khẳng định test **FAIL**.
  - **GREEN**: Hiện thực hóa thuật toán tại `internal/gamification/domain/service/xp_calculator.go` để test **PASS**.
  - **REFACTOR**: Tối ưu hằng số cấu hình.

- [ ] **T-06 [TDD, FR-XP-01, FR-XP-04, BR-XP-01, ADR-0001]**: Vòng đời Aggregate và sự kiện thăng cấp (Aggregate Root).
  - **RED**: Viết test trước tại `internal/gamification/domain/aggregate/user_xp_test.go` kiểm thử:
    - Khởi tạo `NewUserXp(userID)`: 0 XP, Level 1.
    - `ApplyWorkoutResult`: Cộng dồn XP, tự động thăng cấp và phát `UserLeveledUp` khi vượt ngưỡng.
    - `ApplyNutritionBonus`: Nhận lần 1 thành công $+30$ XP; nhận lần 2 cùng ngày trả lỗi `ErrNutritionRewardAlreadyClaimedToday`.
    - Khẳng định test **FAIL**.
  - **GREEN**: Hiện thực hóa Aggregate tại `internal/gamification/domain/aggregate/user_xp.go` và domain events tại `internal/gamification/domain/event/xp_events.go` để test **PASS**.
  - **REFACTOR**: Đảm bảo đóng gói bất biến tuyệt đối (zero ORM tags, zero external imports).

- [ ] **T-07 [Domain Repository Port]**: Định nghĩa Interface Port cho tầng lưu trữ dữ liệu.
  - **Đường dẫn**: `internal/gamification/domain/repository/user_xp_repository.go`
  - **Yêu cầu**: Khai báo `FindByID`, `GetForUpdate`, `Save`.

---

## Phase 3: Persistence Layer & Concurrency Integration Tests

- [ ] **T-08 [TDD Integration, BR-XP-05, ADR-0002]**: Khóa dòng bi quan và chặn trùng lặp buổi tập (PostgreSQL).
  - **RED**: Viết integration test tại `internal/gamification/infrastructure/persistence/postgres/repository_test.go`:
    - **The 3 AM Test**: Khởi tạo user. Bắn đồng thời **10 Goroutines** cùng gọi `GetForUpdate` và cộng $+50$ XP $\rightarrow$ Khẳng định **0% Lost Update**, điểm cuối cùng đúng $500$ XP.
    - **Idempotency Guard**: Ghi 2 lần cùng một `session_id` $\rightarrow$ Lần 2 bị từ chối vi phạm unique constraint `uq_xp_history_workout_session`, transaction rollback an toàn.
    - Khẳng định test **FAIL**.
  - **GREEN**: Hiện thực hóa Data Model/Mapper (`model.go`), `PostgresUserXpRepository` với `SELECT ... FOR UPDATE`, `PostgresXpHistoryRepository` và `PostgresOutboxRepository` để test **PASS**.
  - **REFACTOR**: Tối ưu câu lệnh SQL và quản lý transaction context.

---

## Phase 4: Application CQRS & Transport Wiring (Ráp Nối Hệ Thống)

- [ ] **T-09 [Application Commands, UC-XP-01, UC-XP-02]**: Điều phối nghiệp vụ và ranh giới giao dịch.
  - **Đường dẫn**:
    - `internal/gamification/application/command/process_workout_xp.go` (Khóa bi quan `user_xp` $\rightarrow$ Tính XP $\rightarrow$ Ghi `xp_history` kèm `session_id` $\rightarrow$ Ghi Outbox).
    - `internal/gamification/application/command/process_nutrition_xp.go` (Cộng thưởng dinh dưỡng $+30$ XP hàng ngày).

- [ ] **T-10 [Application Queries, UC-XP-03]**: Đọc thông tin XP và lịch sử.
  - **Đường dẫn**:
    - `internal/gamification/application/query/get_my_xp.go` (Lấy XP và tính `xp_to_next_level`).
    - `internal/gamification/application/query/get_xp_history.go` (Lấy lịch sử phân trang).

- [ ] **T-11 [Transport Consumers & ConnectRPC API, UC-XP-01, UC-XP-02, UC-XP-03]**: Cổng giao tiếp ngoại vi.
  - **Đường dẫn**:
    - `internal/gamification/transport/consumer/workout_event_consumer.go` (Lắng nghe `WorkoutSessionCompleted`).
    - `internal/gamification/transport/consumer/nutrition_event_consumer.go` (Lắng nghe `MealLogged`).
    - `internal/gamification/transport/grpc/gamification_handler.go` (Phục vụ gRPC API theo Protobuf stubs).

- [ ] **T-12 [Transactional Outbox Worker]**: Quét và xuất bản CloudEvents sang Kafka.
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
  - Đối chiếu tiêu chí nghiệm thu [01-spec.md](./01-spec.md) và báo cáo kết quả.
