# Danh Sách Nhiệm Vụ Kỹ Thuật (TDD Tasks): Bảng Xếp Hạng Toàn Hệ Thống (XP Leaderboard)

## Quy Ước Triển Khai
- Áp dụng nghiêm ngặt chu trình **TDD (RED $\rightarrow$ GREEN $\rightarrow$ REFACTOR)**.
- Mọi logic nghiệp vụ tại tầng Application phải đạt độ bao phủ kiểm thử (Test Coverage) $\ge 90\%$.
- Kiểm tra tính toàn vẹn và thực thi lệnh test sau mỗi task.

---

## Danh Sách Task Chi Tiết

- [x] **[T-LB-01] [RED/GREEN]**: Hợp đồng API Protobuf & Sinh Mã Stubs
- [x] **[T-LB-02] [MIGRATION]**: Migration Bảng User Snapshots & Chỉ Mục B-Tree
- [x] **[T-LB-03] [PORTS]**: Định Nghĩa Các Cổng Giao Tiếp (Output Ports)
- [x] **[T-LB-04] [RED]**: Viết Unit Tests Cho GetGlobalLeaderboardHandler
- [x] **[T-LB-05] [GREEN]**: Triển Khai Logic GetGlobalLeaderboardHandler
- [x] **[T-LB-06] [INFRA]**: Hiện Thực Hóa Postgres Repositories & Consumer Đồng Bộ Snapshot
- [x] **[T-LB-07] [TEST]**: Integration Test Với PostgreSQL Thật
- [x] **[T-LB-08] [TRANSPORT]**: Triển Khai ConnectRPC Server Handler & Module Wiring

---

### [x] [T-LB-01] [RED/GREEN] Hợp đồng API Protobuf & Sinh Mã Stubs
- **Mục tiêu**: Bổ sung API ConnectRPC `GetGlobalLeaderboard` vào hợp đồng `GamificationService` theo chuẩn Contract-First (SSOT).
- **Tệp nguồn**:
  - `proto/contracts/supporting/gamification/v1/service/gamification_service.proto`
  - `proto/contracts/supporting/gamification/v1/message/gamification_messages.proto`
- **Các bước thực hiện**:
  1. Soạn thảo RPC `GetGlobalLeaderboard` kèm annotation `google.api.http` (`get: "/api/v1/gamification/leaderboard"`).
  2. Khai báo các message: `GetGlobalLeaderboardRequest`, `GetGlobalLeaderboardResponse`, `LeaderboardEntry`.
  3. Chạy lệnh sinh mã: `buf generate`.
- **Tiêu chí hoàn thành (DoD)**:
  - `buf lint` không có lỗi.
  - File Go stubs sinh ra thành công tại `internal/gen/go/contracts/supporting/gamification/v1/`.

---

### [x] [T-LB-02] [MIGRATION] Migration Bảng User Snapshots & Chỉ Mục B-Tree
- **Mục tiêu**: Bổ sung bảng `gamification.user_snapshots` và xác thực chỉ mục `idx_user_xp_leaderboard (xp DESC, updated_at ASC)`.
- **Tệp nguồn**:
  - `internal/shared/database/migrations/13-create-gamification-tables.sql`
- **Các bước thực hiện**:
  1. Thêm DDL tạo bảng `gamification.user_snapshots` (`user_id UUID PRIMARY KEY`, `display_name VARCHAR(100)`, `avatar_url TEXT`, `updated_at TIMESTAMPTZ`).
  2. Xác thực chỉ mục `idx_user_xp_leaderboard ON gamification.user_xp (xp DESC, updated_at ASC)`.
- **Tiêu chí hoàn thành (DoD)**:
  - Chạy migration SQL thành công, bảng `user_snapshots` được tạo với đầy đủ ràng buộc.

---

### [x] [T-LB-03] [PORTS] Định Nghĩa Các Cổng Giao Tiếp (Output Ports)
- **Mục tiêu**: Khai báo các interface tại tầng Application theo nguyên tắc Đảo ngược phụ thuộc (DIP).
- **Tệp nguồn**:
  - `internal/gamification/application/port/leaderboard_repository.go`
  - `internal/gamification/application/port/user_snapshot_repository.go`
- **Các bước thực hiện**:
  1. Khai báo interface `LeaderboardRepository` với các hàm `GetTopUsers` (trả về danh sách kèm thông tin snapshot), `GetUserRank`.
  2. Khai báo interface `UserSnapshotRepository` với hàm `Upsert` và `UpdateIdentity`.
- **Tiêu chí hoàn thành (DoD)**:
  - Code compile sạch, không import bất kỳ package ORM hay external framework nào.

---

### [x] [T-LB-04] [RED] Viết Unit Tests Cho GetGlobalLeaderboardHandler
- **Mục tiêu**: Thiết lập các kịch bản kiểm thử tự động thất bại trước khi viết code logic (Chu trình RED).
- **Tệp nguồn**:
  - `internal/gamification/application/query/get_global_leaderboard_test.go`
- **Các kịch bản kiểm thử**:
  1. `TestGetGlobalLeaderboard_CurrentUserInTop50`: Kiểm tra user nằm trong Top 50 lấy đúng vị trí không cần gọi `GetUserRank`.
  2. `TestGetGlobalLeaderboard_CurrentUserOutsideTop50`: Kiểm tra user ngoài Top 50 tính đúng thứ hạng qua `GetUserRank`.
  3. `TestGetGlobalLeaderboard_UserWithZeroXp`: Kiểm tra user mới có 0 XP hiển thị mặc định Level 1 và thứ hạng cuối.
  4. `TestGetGlobalLeaderboard_SnapshotDefaultFallback`: Kiểm tra khi user chưa có snapshot thì tên hiển thị fallback về `"Gymer"`.
  5. `TestGetGlobalLeaderboard_LimitClamping`: Kiểm tra giới hạn limit từ 1 đến 100 (mặc định 50).
- **Tiêu chí hoàn thành (DoD)**:
  - Chạy `go test ./internal/gamification/application/query/...` báo lỗi biên dịch/failing test như dự kiến.

---

### [x] [T-LB-05] [GREEN] Triển Khai Logic GetGlobalLeaderboardHandler
- **Mục tiêu**: Viết code tối giản vừa đủ để toàn bộ test trong `T-LB-04` chuyển sang màu xanh (Chu trình GREEN).
- **Tệp nguồn**:
  - `internal/gamification/application/query/get_global_leaderboard.go`
- **Các bước thực hiện**:
  1. Hiện thực hóa `GetGlobalLeaderboardHandler.Handle`.
  2. Gọi `LeaderboardRepository.GetTopUsers` nhận danh sách đã JOIN sẵn tên/avatar từ snapshot cục bộ.
  3. Ghép DTO và trả về `GetGlobalLeaderboardResult`.
- **Tiêu chí hoàn thành (DoD)**:
  - Chạy `go test ./internal/gamification/application/query/...` pass $100\%$.
  - Test coverage đạt $\ge 90\%$.

---

### [x] [T-LB-06] [INFRA] Hiện Thực Hóa Postgres Repositories & Consumer Đồng Bộ Snapshot
- **Mục tiêu**: Triển khai tầng lưu trữ PostgreSQL và Consumer đồng bộ tên/avatar qua Kafka.
- **Tệp nguồn**:
  - `internal/gamification/infrastructure/persistence/leaderboard_repository.go`
  - `internal/gamification/infrastructure/persistence/user_snapshot_repository.go`
  - `internal/gamification/transport/consumer/user_identity_updated_consumer.go`
- **Các bước thực hiện**:
  1. Viết `PostgresLeaderboardRepository` thực thi `user_xp LEFT JOIN user_snapshots`.
  2. Viết `PostgresUserSnapshotRepository` thực hiện UPSERT snapshot.
  3. Viết `UserIdentityUpdatedConsumer` lắng nghe `contracts.supporting.profile.v1.event.UserIdentityUpdated` từ topic `profile.events` để cập nhật `user_snapshots`.
- **Tiêu chí hoàn thành (DoD)**:
  - Toàn bộ repo và consumer compile sạch, pass unit tests.

---

### [x] [T-LB-07] [TEST] Integration Test Với PostgreSQL Thật
- **Mục tiêu**: Xác thực hiệu năng truy vấn `user_xp LEFT JOIN user_snapshots` trên database thực tế.
- **Tệp nguồn**:
  - `internal/gamification/infrastructure/persistence/leaderboard_repository_test.go`
- **Các kịch bản kiểm thử**:
  1. Kiểm tra thứ tự sắp xếp giảm dần theo XP và hiển thị đúng tên/avatar từ snapshot.
  2. Kiểm tra Tie-breaking: cùng XP thì ai có `updated_at` nhỏ hơn đứng trước; cùng cả `updated_at` thì so sánh `user_id`.
  3. Kiểm tra tính toán `GetUserRank` chính xác trên tập dữ liệu 100 dòng.
- **Tiêu chí hoàn thành (DoD)**:
  - Toàn bộ integration test chạy thành công.
  - Thời gian truy vấn Top 50 $< 2\text{ ms}$.

---

### [x] [T-LB-08] [TRANSPORT] Triển Khai ConnectRPC Server Handler & Module Wiring
- **Mục tiêu**: Đấu nối API endpoint ra ngoài tầng giao diện và hoàn tất dependency injection.
- **Tệp nguồn**:
  - `internal/gamification/transport/grpc/gamification_handler.go`
  - `internal/gamification/module.go`
  - `cmd/api/main.go`
- **Các bước thực hiện**:
  1. Triển khai phương thức RPC `GetGlobalLeaderboard` ánh xạ từ Protobuf request sang Application query và ngược lại.
  2. Đăng ký service handler vào Connect server trong `internal/gamification/module.go`.
  3. Khởi tạo module trong `cmd/api/main.go` (không sử dụng Google Wire).
- **Tiêu chí hoàn thành (DoD)**:
  - Gọi API ConnectRPC qua `curl` / `buf curl` trả về đúng danh sách Top 50 và `my_standing` kèm `display_name`, `avatar_url`.
