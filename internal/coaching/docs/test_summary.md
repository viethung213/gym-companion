# Danh mục & Hướng dẫn Kiểm thử (Test Suite Catalog) — Coaching Module

Tài liệu này tổng hợp cấu trúc, mục lục và phạm vi các bài kiểm thử (Unit Tests) thuộc module `coaching`.

---

## 📂 Mục lục các nhóm Test (Test Packages)

### 1. `internal/coaching/infrastructure/ai/adk/` (Agent & ADK Workflow)
Nơi kiểm thử logic của Agent AI, Validator, Mapper và Retry:
- **`validator_test.go`**:
  - `TestPlanValidator_AllValid`: Kiểm tra kế hoạch chuẩn xác trả về 0 issue.
  - `TestPlanValidator_StrictRejectsUnknownID`: Kiểm tra chế độ Strict Mode từ chối `exercise_id` không có trong catalog.
  - `TestPlanValidator_DropModeRemovesInvalidExercise`: Kiểm tra chế độ Salvage Mode loại bỏ bài tập sai.
  - `TestPlanValidator_ReportsEmptyMainExercises`: Kiểm tra báo lỗi khi không có bài tập chính.
  - `TestPlanValidator_ReportsWeeklySessionCap`: Kiểm tra vi phạm giới hạn số buổi tập / tuần.
  - Kiểm tra validation cho `slot_time` và `estimated_duration_minutes`.
- **`mapping_test.go`**:
  - `TestMapToDomainRoadmap_GroupsSessionsByDate`: Gom các buổi tập cùng ngày vào chung 1 `DayPlan`.
  - `TestMapToDomainRoadmap_SetsAllIdentityFields`: Xác minh gán đầy đủ ID, `SlotTime` và `EstimatedDurationMinutes`.
  - `TestMapToDomainRoadmap_EnrichesExerciseNamesFromValidator`: Kiểm tra làm giàu tên bài tập từ catalog.
- **`transient_test.go` & `retry_test.go`**:
  - Kiểm tra phân loại lỗi tạm thời (Transient Errors 429, 503) và cơ chế tự động thử lại (Retry with Backoff).

---

### 2. `internal/coaching/infrastructure/adapters/` (Readers & External Adapters)
- **`postgres_readers_test.go`**:
  - `TestPostgresUserProfileReader`:
    - `returns_fallback_for_non-existent_user_profile`: Giảm thiểu rủi ro khi không tìm thấy Profile người dùng.
    - `reads_real_user_profile_and_body_metrics_from_DB`: Đọc đầy đủ thông tin chỉ số cơ thể và chấn thương.
    - `parses_preferred_workout_times_in_key-value_map_format`: Kiểm tra parse dữ liệu `preferred_workout_times` dạng Key-Value Map (`{"mon":["06:00-07:30"]}`).
  - `TestPostgresWorkoutSessionReader`: Kiểm tra lấy lịch sử buổi tập và tính toán 1RM từ `personal_records`.
  - `TestPostgresExerciseCatalogReader`: Kiểm tra tìm kiếm bài tập theo nhóm cơ và thiết bị.

---

### 3. `internal/coaching/domain/` (Domain Aggregates & Business Rules)
- **`session_plan_test.go`**:
  - Kiểm tra tạo mới `SessionPlan`, chuyển trạng thái `PENDING` → `COMPLETED` / `SKIPPED` / `ABORTED`.
  - Kiểm tra tính bất biến (Immutability) và các ràng buộc toàn vẹn dữ liệu.
- **`overload_validator_test.go` & `scr_calculator_test.go`**:
  - Kiểm tra thuật toán Progressive Overload (+10% đến +30% volume/tải).
  - Kiểm tra tính toán Session Completion Rate (SCR) và Delta RPE.

---

### 4. `internal/coaching/transport/consumer/` & `infrastructure/worker/`
- **`profile_completed_consumer_test.go`**: Kiểm tra việc lắng nghe sự kiện `ProfileCompleted` để kích hoạt tạo lộ trình tập tự động.
- **`upcoming_workout_reminder_worker_test.go`**: Kiểm tra quét các buổi tập sắp tới và đẩy sự kiện nhắc nhở vào Outbox trước 60 phút theo `slot_time`.

---

## 🚀 Cách chạy Test

Chạy tất cả unit tests của module `coaching`:
```bash
go test -v ./internal/coaching/infrastructure/ai/adk/... ./internal/coaching/infrastructure/adapters/... ./internal/coaching/infrastructure/persistence/... ./internal/coaching/domain/... ./internal/coaching/transport/consumer/... ./internal/coaching/infrastructure/worker/...
```
