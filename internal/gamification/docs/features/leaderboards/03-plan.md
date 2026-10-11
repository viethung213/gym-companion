# Kế Hoạch Triển Khai Kỹ Thuật (Implementation Plan): Bảng Xếp Hạng Toàn Hệ Thống

Tài liệu này xác định giao diện các cổng (Ports), kiến trúc luồng dữ liệu hai chặng và ma trận kiểm thử tự động cho tính năng Bảng Xếp Hạng XP Toàn Hệ Thống.

---

## 1. Quyết Định Kỹ Thuật & Kiến Trúc Lõi

* **PostgreSQL B-Tree Index Scan (ADR-0001)**: Tận dụng chỉ mục tổng hợp `idx_user_xp_leaderboard (xp DESC, updated_at ASC)` trên bảng `gamification.user_xp`. Thời gian truy vấn Top 50 đạt $< 3\text{ ms}$ cho quy mô MVP $< 50,000$ users. Khi vượt quy mô này, kích hoạt chiến lược phân tầng (Materialized View / Redis ZSET) theo ADR-0001 mà không làm thay đổi các Port Interfaces.
* **In-Process Profile Enrichment (ADR-0002)**: Thay vì distributed `JOIN` hoặc gọi HTTP sang module Profile, `GetGlobalLeaderboardHandler` gọi qua Port nội bộ `ProfileReaderPort` (in-process Go interface). Tự động fallback `"Người dùng FITAI"` nếu user chưa cập nhật profile.
* **Tie-Breaking Deterministic**: Trường hợp đồng điểm XP, ai đạt điểm trước (`updated_at ASC`) xếp trên; nếu tiếp tục trùng khớp thì sắp xếp theo `user_id ASC`.

---

## 2. Hợp Đồng Giao Diện Cốt Lõi (Core Interfaces & Signatures)

### 2.1 Output Ports (Application Layer)
```go
// port/leaderboard_repository.go
type LeaderboardUserRecord struct {
    UserID    uuid.UUID
    XP        int64
    Level     int32
    UpdatedAt time.Time
}

type LeaderboardRepository interface {
    GetTopUsers(ctx context.Context, limit int) ([]LeaderboardUserRecord, error)
    GetUserRank(ctx context.Context, userID uuid.UUID, xp int64, updatedAt time.Time) (int64, error)
    GetUserXpRecord(ctx context.Context, userID uuid.UUID) (*LeaderboardUserRecord, error)
}

// port/profile_reader.go: Cổng nạp thông tin người dùng in-process
type ProfileSummary struct {
    DisplayName string
    AvatarURL   string
}

type ProfileReaderPort interface {
    BatchGetSummaries(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]ProfileSummary, error)
}
```

### 2.2 Application Use Case & DTO
```go
// query/get_global_leaderboard.go
type GetGlobalLeaderboardQuery struct {
    CurrentUserID uuid.UUID
    Limit         int // Default 50, Max 100
}

type LeaderboardEntryDTO struct {
    Rank        int64
    UserID      uuid.UUID
    DisplayName string
    AvatarURL   string
    XP          int64
    Level       int32
}

type GetGlobalLeaderboardResult struct {
    Items      []LeaderboardEntryDTO
    MyStanding LeaderboardEntryDTO
}

type GetGlobalLeaderboardHandler interface {
    Handle(ctx context.Context, q GetGlobalLeaderboardQuery) (*GetGlobalLeaderboardResult, error)
}
```

---

## 3. Ma Trận Kiểm Thử TDD & Hiệu Năng (Test Matrix)

| Hạng Mục Kiểm Thử | File Test | Kịch Bản Trọng Tâm |
| :--- | :--- | :--- |
| **Use Case Unit Test** | `application/query/get_global_leaderboard_test.go` | - User nằm trong Top 50 $\rightarrow$ `MyStanding` lấy trực tiếp từ danh sách (không query DB lần 2).<br>- User ngoài Top 50 $\rightarrow$ Tính rank chính xác qua `GetUserRank`.<br>- Profile Reader lỗi/thiếu data $\rightarrow$ Fallback an toàn `"Người dùng FITAI"`. |
| **Database Index & Tie-break** | `infrastructure/persistence/postgres/leaderboard_repo_test.go` | - Đồng điểm XP $\rightarrow$ người có `updated_at` sớm hơn xếp trước.<br>- Phân trang Keyset quét đúng chỉ mục `idx_user_xp_leaderboard`.<br>- Hiệu năng câu lệnh `LIMIT 50` đạt $< 5\text{ ms}$ trên dataset $100,000$ dòng. |
| **In-Process Adapter** | `infrastructure/adapter/profile_reader_test.go` | - Batch resolve đúng tên và avatar theo danh sách `user_ids`. |
| **ConnectRPC Handler** | `transport/grpc/leaderboard_handler_test.go` | - Ánh xạ DTO sang Protobuf `GetGlobalLeaderboardResponse`. |

### Lệnh Thực Thi Kiểm Thử
```bash
# 1. Chạy Unit Tests tầng Application
go test -v ./internal/gamification/application/query/...

# 2. Chạy Integration Tests PostgreSQL Index Scan
go test -v -tags=integration ./internal/gamification/infrastructure/persistence/postgres/...

# 3. Benchmark tốc độ truy vấn Leaderboard
go test -bench=BenchmarkGetTopUsers -benchmem ./internal/gamification/infrastructure/persistence/postgres/...
```
