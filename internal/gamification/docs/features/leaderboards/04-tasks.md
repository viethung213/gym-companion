# Danh Sách Nhiệm Vụ Kỹ Thuật (TDD Tasks): Bảng Xếp Hạng Toàn Hệ Thống (XP Leaderboard)

## Quy Ước Triển Khai
- Áp dụng nghiêm ngặt chu trình **TDD (RED $\rightarrow$ GREEN $\rightarrow$ REFACTOR)**.
- Mọi logic nghiệp vụ tại tầng Application phải đạt độ bao phủ kiểm thử (Test Coverage) $\ge 90\%$.
- Kiểm tra tính toàn vẹn và thực thi lệnh test sau mỗi task.

---

## Danh Sách Task Chi Tiết

### [T-LB-01] [RED/GREEN] Hợp đồng API Protobuf & Sinh Mã Stubs
- **Mục tiêu**: Định nghĩa API ConnectRPC `LeaderboardService` theo chuẩn Contract-First.
- **Tệp nguồn**:
  - `contracts/gamification/leaderboard/v1/leaderboard.proto`
- **Các bước thực hiện**:
  1. Soạn thảo định nghĩa dịch vụ `LeaderboardService` với RPC `GetGlobalLeaderboard`.
  2. Khai báo các message: `GetGlobalLeaderboardRequest`, `GetGlobalLeaderboardResponse`, `LeaderboardEntry`.
  3. Chạy lệnh sinh mã: `buf generate`.
- **Tiêu chí hoàn thành (DoD)**:
  - `buf lint` không có lỗi.
  - File Go stubs sinh ra thành công tại `internal/gen/go/contracts/gamification/leaderboard/v1/`.

---

### [T-LB-02] [MIGRATION] Migration Chỉ Mục B-Tree Composite Index
- **Mục tiêu**: Tạo index tối ưu hóa cho câu lệnh `ORDER BY xp DESC, updated_at ASC`.
- **Tệp nguồn**:
  - `internal/gamification/infrastructure/persistence/migrations/`
- **Các bước thực hiện**:
  1. Viết migration SQL thêm chỉ mục:
     ```sql
     CREATE INDEX IF NOT EXISTS idx_user_xp_leaderboard 
     ON gamification.user_xp (xp DESC, updated_at ASC);
     ```
- **Tiêu chí hoàn thành (DoD)**:
  - Migration chạy thành công không lỗi cú pháp.
  - Kiểm tra `EXPLAIN ANALYZE` xác nhận câu lệnh `LIMIT 50` sử dụng `Index Scan`.

---

### [T-LB-03] [PORTS] Định Nghĩa Các Cổng Giao Tiếp (Output Ports)
- **Mục tiêu**: Khai báo các interface tại tầng Application theo nguyên tắc Đảo ngược phụ thuộc (DIP).
- **Tệp nguồn**:
  - `internal/gamification/application/port/leaderboard_repository.go`
  - `internal/gamification/application/port/profile_reader.go`
- **Các bước thực hiện**:
  1. Khai báo interface `LeaderboardRepository` với các hàm `GetTopUsers`, `GetUserRank`, `GetUserXpRecord`.
  2. Khai báo interface `ProfileReaderPort` với hàm `BatchGetSummaries`.
- **Tiêu chí hoàn thành (DoD)**:
  - Code compile sạch, không import bất kỳ package ORM hay external framework nào.

---

### [T-LB-04] [RED] Viết Unit Tests Cho GetGlobalLeaderboardHandler
- **Mục tiêu**: Thiết lập các kịch bản kiểm thử tự động thất bại trước khi viết code logic (Chu trình RED).
- **Tệp nguồn**:
  - `internal/gamification/application/query/get_global_leaderboard_test.go`
- **Các kịch bản kiểm thử**:
  1. `TestGetGlobalLeaderboard_CurrentUserInTop50`: Kiểm tra user nằm trong Top 50 lấy đúng vị trí không cần gọi `GetUserRank`.
  2. `TestGetGlobalLeaderboard_CurrentUserOutsideTop50`: Kiểm tra user ngoài Top 50 tính đúng thứ hạng qua `GetUserRank`.
  3. `TestGetGlobalLeaderboard_UserWithZeroXp`: Kiểm tra user chưa có điểm XP hiển thị mặc định Level 1 và thứ hạng cuối.
  4. `TestGetGlobalLeaderboard_ProfileServiceFallback`: Kiểm tra khi module Profile lỗi/timeout thì tên hiển thị fallback về `"Gymer"`.
  5. `TestGetGlobalLeaderboard_LimitClamping`: Kiểm tra giới hạn limit từ 1 đến 100 (mặc định 50).
- **Tiêu chí hoàn thành (DoD)**:
  - Chạy `go test ./internal/gamification/application/query/...` báo lỗi biên dịch/failing test như dự kiến.

---

### [T-LB-05] [GREEN] Triển Khai Logic GetGlobalLeaderboardHandler
- **Mục tiêu**: Viết code tối giản vừa đủ để toàn bộ test trong `T-LB-04` chuyển sang màu xanh (Chu trình GREEN).
- **Tệp nguồn**:
  - `internal/gamification/application/query/get_global_leaderboard.go`
- **Các bước thực hiện**:
  1. Hiện thực hóa `GetGlobalLeaderboardHandler.Handle`.
  2. Tích hợp gọi repo, gom danh sách `user_ids`, gọi `profileReader` có context timeout $500\text{ ms}$.
  3. Ghép DTO và trả về `GetGlobalLeaderboardResult`.
- **Tiêu chí hoàn thành (DoD)**:
  - Chạy `go test ./internal/gamification/application/query/...` pass $100\%$.
  - Test coverage đạt $\ge 90\%$.

---

### [T-LB-06] [INFRA] Hiện Thực Hóa PostgresLeaderboardRepository & ProfileReaderAdapter
- **Mục tiêu**: Triển khai các Driven Adapters tại tầng Infrastructure.
- **Tệp nguồn**:
  - `internal/gamification/infrastructure/persistence/leaderboard_repository.go`
  - `internal/gamification/infrastructure/adapter/profile_adapter.go`
- **Các bước thực hiện**:
  1. Viết `PostgresLeaderboardRepository` sử dụng GORM/SQL truy vấn bảng `gamification.user_xp`.
  2. Viết `InProcessProfileReaderAdapter` kết nối sang module `profile`.
- **Tiêu chí hoàn thành (DoD)**:
  - Các adapter hiện thực đầy đủ các methods của Ports tương ứng.

---

### [T-LB-07] [TEST] Integration Test Với PostgreSQL Thật
- **Mục tiêu**: Xác thực hiệu năng truy vấn và thuật toán hòa điểm trên database thực tế.
- **Tệp nguồn**:
  - `internal/gamification/infrastructure/persistence/leaderboard_repository_test.go`
- **Các kịch bản kiểm thử**:
  1. Kiểm tra thứ tự sắp xếp giảm dần theo XP.
  2. Kiểm tra Tie-breaking: cùng XP thì ai có `updated_at` nhỏ hơn đứng trước; cùng cả `updated_at` thì so sánh `user_id`.
  3. Kiểm tra tính toán `GetUserRank` chính xác trên tập dữ liệu 100 dòng.
- **Tiêu chí hoàn thành (DoD)**:
  - Toàn bộ integration test chạy thành công.
  - Thời gian truy vấn Top 50 $< 5\text{ ms}$.

---

### [T-LB-08] [TRANSPORT] Triển Khai ConnectRPC Server Handler & Wire DI
- **Mục tiêu**: Đấu nối API endpoint ra ngoài tầng giao diện và hoàn tất dependency injection.
- **Tệp nguồn**:
  - `internal/gamification/transport/grpc/leaderboard_handler.go`
  - `cmd/server/wire.go`
- **Các bước thực hiện**:
  1. Triển khai `LeaderboardGrpcHandler` ánh xạ từ Protobuf request sang Application query và ngược lại.
  2. Đăng ký service handler vào gRPC / Connect server.
  3. Cấu hình Dependency Injection trong `wire.go`.
- **Tiêu chí hoàn thành (DoD)**:
  - Gọi API ConnectRPC qua `curl` / `buf curl` trả về đúng danh sách Top 50 và `my_standing` kèm `display_name`, `avatar_url`.
