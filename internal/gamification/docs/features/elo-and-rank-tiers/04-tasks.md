# Tasks: ELO Rating & Rank Tiers (Lean TDD Checklist)

Danh sách công việc kỹ thuật tinh gọn theo chuẩn **Test-Driven Development (TDD)** (`RED -> GREEN -> REFACTOR`). Các bài kiểm thử **chỉ tập trung vào nghiệp vụ cốt lõi (Domain Invariants & Concurrency Safety)**, loại bỏ kiểm thử hình thức cho các tầng trung gian không chứa logic.

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
  - **Xác minh**: Mã nguồn sinh ra tại `internal/gen/go/contracts/supporting/gamification/v1/` biên dịch thành công (`go build ./...`).

- [ ] **T-03 [Database Migration, BR-05]**: Viết script migration PostgreSQL khởi tạo schema và các bảng dữ liệu cho Gamification.
  - **Đường dẫn**: `internal/shared/database/migrations/13-create-gamification-tables.sql`
  - **Nội dung**: Schema `gamification`, bảng `user_elo` (không lưu cột `rank_tier`, check `[1000, 3000]`), `elo_history` (kèm unique index `uq_elo_history_workout_session`), `outbox_events` kèm chỉ mục B-Tree.
  - **Xác minh**: Script SQL thực thi thành công trên PostgreSQL.

---

## Phase 2: Core Domain Logic (Strict TDD: RED -> GREEN -> REFACTOR)

- [ ] **T-04 [TDD, FR-03, BR-01, BR-04]**: Ánh xạ 5 bậc hạng và logic kiểm tra mốc biên (Value Object).
  - **RED**: Viết test trước tại `internal/gamification/domain/vo/rank_tier_test.go` kiểm thử chính xác các mốc biên ($1199 \rightarrow 1200$, $1499 \rightarrow 1500$, $1799 \rightarrow 1800$, $2199 \rightarrow 2200$, sàn $1000$, trần $3000$). Khẳng định test **FAIL**.
  - **GREEN**: Hiện thực hóa hàm thuần túy tại `internal/gamification/domain/vo/rank_tier.go` để test **PASS**.
  - **REFACTOR**: Tinh gọn biểu thức điều kiện.

- [ ] **T-05 [TDD, FR-02, BR-02, BR-03, BR-05]**: Thuật toán tính điểm ELO theo hiệu suất tập luyện (Domain Service).
  - **RED**: Viết table-driven test trước tại `internal/gamification/domain/service/elo_calculator_test.go` kiểm thử:
    - Công thức hiệu suất từ volume ratio & form score ratio (nhận từ payload sự kiện của `workout_execution`).
    - Biến động kẹp cứng trong đoạn $[-25, +40]$ ELO.
    - Kẹp trần cứng $3000$ ELO và sàn tối thiểu $1000$ ELO (ADR-0005).
    - Hệ số $K$ suy giảm theo Tier ($32 \rightarrow 24 \rightarrow 16 \rightarrow 10$).
    - Thưởng dinh dưỡng cố định $+3$ ELO/ngày (ADR-0001).
    - Khẳng định test **FAIL**.
  - **GREEN**: Hiện thực hóa thuật toán tại `internal/gamification/domain/service/elo_calculator.go` để test **PASS**.
  - **REFACTOR**: Tối ưu tính toán số thực và hằng số cấu hình.

- [ ] **T-06 [TDD, FR-01, FR-04, FR-05, FR-07, FR-08, BR-04, BR-05, BR-07]**: Bất biến trạng thái, thăng/giáng bậc, kỷ lục peak_elo và thưởng dinh dưỡng (Aggregate Root).
  - **RED**: Viết test trước tại `internal/gamification/domain/aggregate/user_elo_test.go` kiểm thử:
    - Khởi tạo `NewUserElo(userID)` mặc định $1000$ ELO, $1000$ Peak ELO, Bậc Bronze (`RankTier()` tính động).
    - `ApplyWorkoutResult`: Tự động thăng hạng / giáng hạng tức thì, phát sinh Domain Events (ADR-0003), cập nhật `peakElo = max(peakElo, currentElo)`.
    - `ApplyNutritionBonus`: Nhận lần 1 thành công $+3$ ELO; nhận lần 2 cùng ngày trả lỗi `ErrNutritionAlreadyClaimed` (ADR-0001).
    - `ApplyInactivityDecay`: Quá 14 ngày không tập trừ $15$ ELO (không rớt dưới sàn 1000, `peakElo` không bị giảm).
    - Khẳng định test **FAIL**.
  - **GREEN**: Hiện thực hóa Aggregate tại `internal/gamification/domain/aggregate/user_elo.go` và domain events tại `internal/gamification/domain/event/elo_events.go` để test **PASS**.
  - **REFACTOR**: Đảm bảo đóng gói bất biến tuyệt đối (zero ORM tags, zero external imports).

- [ ] **T-07 [Domain Repository Port]**: Định nghĩa Interface Port cho tầng lưu trữ dữ liệu.
  - **Đường dẫn**: `internal/gamification/domain/repository/user_elo_repository.go`
  - **Yêu cầu**: Khai báo `FindByID`, `GetForUpdate`, `Save`.

---

## Phase 3: Persistence Layer & Concurrency Integration Tests

- [ ] **T-08 [TDD Integration, BR-06, ADR-0004]**: Khóa dòng bi quan và chặn trùng lặp buổi tập (PostgreSQL).
  - **RED**: Viết integration test tại `internal/gamification/infrastructure/persistence/postgres/repository_test.go`:
    - **The 3 AM Test**: Khởi tạo user $1200$ ELO. Bắn đồng thời **10 Goroutines** cùng gọi `GetForUpdate` và cộng $+10$ ELO $\rightarrow$ Khẳng định **0% Lost Update**, điểm cuối cùng đúng $1300$ ELO.
    - **Idempotency Guard**: Ghi 2 lần cùng một `session_id` $\rightarrow$ Lần 2 bị từ chối vi phạm unique constraint `uq_elo_history_workout_session`, transaction rollback an toàn.
    - Khẳng định test **FAIL**.
  - **GREEN**: Hiện thực hóa Data Model/Mapper (`model.go`), `PostgresUserEloRepository` với `SELECT ... FOR UPDATE`, `PostgresEloHistoryRepository` và `PostgresOutboxRepository` để test **PASS**.
  - **REFACTOR**: Tối ưu câu lệnh SQL và quản lý transaction context.

---

## Phase 4: Application CQRS & Transport Wiring (Ráp Nối Hệ Thống)

- [ ] **T-09 [Application Commands, UC-ELO-01, UC-ELO-02, UC-ELO-03]**: Điều phối nghiệp vụ và ranh giới giao dịch.
  - **Đường dẫn**:
    - `internal/gamification/application/command/process_workout_elo.go` (Khóa bi quan `user_elo` $\rightarrow$ Tính ELO $\rightarrow$ Ghi `elo_history` kèm `session_id` để chặn lặp $\rightarrow$ Ghi Outbox).
    - `internal/gamification/application/command/process_nutrition_elo.go` (Cộng thưởng dinh dưỡng $+3$ ELO hàng ngày).
    - `internal/gamification/application/command/process_inactivity_decay.go` (Trừ điểm bất hoạt).

- [ ] **T-10 [Application Queries, UC-ELO-04, FR-06]**: Đọc thông tin ELO và lịch sử.
  - **Đường dẫn**:
    - `internal/gamification/application/query/get_my_elo.go` (Lấy ELO và tính `points_to_next_tier`).
    - `internal/gamification/application/query/get_elo_history.go` (Lấy lịch sử phân trang).

- [ ] **T-11 [Transport Consumers & ConnectRPC API, UC-ELO-01, UC-ELO-02, UC-ELO-04]**: Cổng giao tiếp ngoại vi.
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
