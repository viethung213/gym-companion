# Danh Sách Nhiệm Vụ Kỹ Thuật (TDD Tasks): Chuỗi Ngày Kỷ Luật (Streaks Engine)

## Quy Ước Triển Khai
- Áp dụng nghiêm ngặt chu trình **TDD (RED $\rightarrow$ GREEN $\rightarrow$ REFACTOR)**.
- Mọi logic nghiệp vụ tại tầng Domain & Application phải đạt độ bao phủ kiểm thử (Test Coverage) $\ge 90\%$.
- Kiểm tra tính toàn vẹn và thực thi lệnh test sau mỗi task.

---

## Danh Sách Task Chi Tiết

### [T-ST-01] [CONTRACT] Hợp Đồng API Protobuf & Sinh Mã Go Stubs
- **Mục tiêu**: Bổ sung API ConnectRPC quản lý chuỗi ngày vào hợp đồng `GamificationService` theo chuẩn Contract-First (SSOT).
- **Tệp nguồn**:
  - `proto/contracts/supporting/gamification/v1/service/gamification_service.proto` (kèm `google.api.http` và `BearerAuth`)
  - `proto/contracts/supporting/gamification/v1/message/gamification_messages.proto`
  - `proto/contracts/supporting/gamification/v1/event/gamification_events.proto`
- **Các bước thực hiện**:
  1. Khai báo các RPCs trong `GamificationService`: `GetStreak`, `GetStreakHistory`.
  2. Khai báo các message: `GetStreakRequest`, `GetStreakResponse`, `GetStreakHistoryRequest`, `GetStreakHistoryResponse`, `StreakHistoryDay`.
  3. Khai báo các CloudEvents message: `StreakAdvanced`, `StreakReset`, `StreakMilestoneReached`.
  4. Chạy lệnh sinh mã: `buf generate`.
- **Tiêu chí hoàn thành (DoD)**:
  - `buf lint` không có lỗi.
  - File Go stubs sinh ra thành công tại `internal/gen/go/contracts/supporting/gamification/v1/`.

---

### [T-ST-02] [MIGRATION] Migration Cơ Sở Dữ Liệu Chuỗi Ngày
- **Mục tiêu**: Tạo các bảng `user_streak`, `streak_history` với đầy đủ ràng buộc và khóa duy nhất trong migration chuẩn.
- **Tệp nguồn**:
  - `internal/shared/database/migrations/13-create-gamification-tables.sql`
- **Các bước thực hiện**:
  1. Viết migration SQL tạo bảng `gamification.user_streak` (lưu trữ `user_id`, `current_streak`, `longest_streak`, `last_active_date`, `created_at`, `updated_at`).
  2. Viết migration SQL tạo bảng `gamification.streak_history` kèm:
     - `CONSTRAINT uq_streak_history_user_date UNIQUE (user_id, activity_date)`.
  3. Thêm chỉ mục `idx_streak_history_user_date ON gamification.streak_history (user_id, activity_date DESC)`.
- **Tiêu chí hoàn thành (DoD)**:
  - Chạy migration SQL thành công vào cơ sở dữ liệu.

---

### [T-ST-03] [DOMAIN TDD] Triển Khai UserStreak Aggregate (RED -> GREEN -> REFACTOR)
- **Mục tiêu**: Xây dựng nghiệp vụ chuỗi ngày, cơ chế Lazy Evaluation duy trì kỷ luật và bảo toàn kỷ lục chuỗi dài nhất (`longest_streak`).
- **Tệp nguồn**:
  - `internal/gamification/domain/aggregate/user_streak.go`
  - `internal/gamification/domain/aggregate/user_streak_test.go`
  - `internal/gamification/domain/entity/streak_history_entry.go`
  - `internal/gamification/domain/vo/activity_type.go`
- **Các kịch bản kiểm thử**:
  1. `TestUserStreak_ConsecutiveDays`: Tập các ngày liên tiếp $\rightarrow$ Chuỗi tăng $1 \rightarrow 2 \rightarrow 3$, cập nhật `longest_streak`.
  2. `TestUserStreak_SameDayMultipleActivities`: Cùng ngày có nhiều buổi tập / bữa ăn $\rightarrow$ Chuỗi giữ nguyên không tăng lặp (lũy đẳng).
  3. `TestUserStreak_MissOneOrMoreDays`: Nghỉ từ 1 ngày trở lên ($\Delta \ge 2$) $\rightarrow$ Chuỗi reset về 1, `longest_streak` giữ nguyên kỷ lục cũ, phát sự kiện `StreakReset`.
  4. `TestUserStreak_Milestones`: Chạm các mốc 7, 14, 30 ngày $\rightarrow$ Phát sự kiện `StreakMilestoneReached`.
  5. `TestUserStreak_PopDomainEvents`: Trả về bản sao phòng vệ (defensive copy slice) và reset danh sách event.
- **Tiêu chí hoàn thành (DoD)**:
  - Unit tests cho Aggregate pass $100\%$.
  - Test coverage đạt $\ge 95\%$.

---

### [T-ST-04] [APP TDD] Triển Khai Use Cases & Command Handlers
- **Mục tiêu**: Xây dựng Use Cases duy trì chuỗi và tra cứu trạng thái chuỗi qua `TransactionManager`.
- **Tệp nguồn**:
  - `internal/gamification/application/port/streak_repository.go`
  - `internal/gamification/application/port/tx.go`
  - `internal/gamification/application/command/maintain_streak.go`
  - `internal/gamification/application/command/maintain_streak_test.go`
  - `internal/gamification/application/query/get_streak.go`
  - `internal/gamification/application/query/get_streak_history.go`
- **Các kịch bản kiểm thử**:
  - `maintain_streak_test.go`: Xác định đúng ngày địa phương theo `user_timezone`; bọc chu trình trong `txManager.WithTransaction`; lưu CloudEvents vào Outbox.
  - `get_streak_test.go`: Truy vấn chỉ đọc tính ảo `is_active_today = true` khi `last_active_date == todayStr`.
- **Tiêu chí hoàn thành (DoD)**:
  - Unit tests cho Use Cases pass $100\%$.

---

### [T-ST-05] [INFRA] Hiện Thực Hóa PostgresStreakRepository & Integration Tests
- **Mục tiêu**: Triển khai tầng lưu trữ với khóa dòng bi quan `SELECT ... FOR UPDATE` và kiểm thử tương tranh trên PostgreSQL thật.
- **Tệp nguồn**:
  - `internal/gamification/infrastructure/persistence/streak_repository.go`
  - `internal/gamification/infrastructure/persistence/streak_repository_test.go`
- **Các kịch bản kiểm thử tương tranh (The 3 AM Test)**:
  1. Bắn đồng thời 5 Goroutines cùng ghi nhận hoạt động cho 1 user trong cùng một ngày qua `txManager.WithTransaction` $\rightarrow$ Chuỗi tăng đúng 1 lần, không trùng lặp bản ghi, 0% race condition.
- **Tiêu chí hoàn thành (DoD)**:
  - Toàn bộ integration test chạy thành công trên PostgreSQL.

---

### [T-ST-06] [TRANSPORT] Triển Khai ConnectRPC Server, Kafka Consumers & Module Wiring
- **Mục tiêu**: Đấu nối API và các sự kiện hệ thống vào module.
- **Tệp nguồn**:
  - `internal/gamification/transport/grpc/gamification_handler.go`
  - `internal/gamification/transport/consumer/nutrition_streak_consumer.go`
  - `internal/gamification/module.go`
  - `cmd/api/main.go`
- **Các bước thực hiện**:
  1. Hiện thực các phương thức RPC `GetStreak`, `GetStreakHistory` ánh xạ từ Protobuf sang Use Cases.
  2. Viết Kafka Consumer lắng nghe `DailyNutritionGoalAchieved` gọi sang `MaintainStreakCommandHandler`. Đối với hoạt động tập luyện (`WorkoutSessionCompleted`), việc duy trì streak được điều phối bởi `ActivityIngestionCoordinator` (xem [architecture.md#2](../../architecture.md#2-cấu-trúc-thư-mục-và-tổ-chức-mã-nguồn---hexagonal-modular-monolith)).
  3. Khởi tạo các handlers trong `internal/gamification/module.go` và đăng ký trong `cmd/api/main.go` (không sử dụng Google Wire).
- **Tiêu chí hoàn thành (DoD)**:
  - Gọi API ConnectRPC qua `buf curl` trả về đúng trạng thái chuỗi và bản đồ nhiệt lịch sử.
