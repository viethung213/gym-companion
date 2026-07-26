# MỤC LỤC VÀ TÀI LIỆU HƯỚNG DẪN KIỂM THỬ MODULE COACHING & PLANNING

Tài liệu này tổng hợp toàn bộ danh mục Unit Tests và Integration Tests cho **Module Coaching & Planning** trong kiến trúc Hexagonal (Ports & Adapters).

---

## 1. DANH MỤC UNIT TESTS (TEST SUITES INDEX)

### 🧩 A. Domain Layer Unit Tests (`internal/coaching/domain/`)
* **`roadmap_test.go`**:
  - `TestNewWorkoutRoadmap/successful_creation`: Kiểm thử khởi tạo thành công Lộ trình 4 tuần (`WorkoutRoadmap`).
  - `TestNewWorkoutRoadmap/empty_user_id_returns_error`: Kiểm thử bắt lỗi khi thiếu `user_id`.
  - `TestNewWorkoutRoadmap/invalid_date_range_returns_error`: Kiểm thử bắt lỗi khi ngày bắt đầu sau ngày kết thúc.
  - `TestNewWorkoutRoadmap/complete_active_roadmap`: Kiểm thử chuyển trạng thái lộ trình thành `Completed`.
* **`schedule_test.go`**:
  - `TestNewWeeklySchedule/successful_creation_with_1_rest_day`: Kiểm thử tạo Lịch tuần có 1 ngày nghỉ chuẩn BR-AC-01.
  - `TestNewWeeklySchedule/violation_of_BR-AC-01_(7_training_days,_0_rest_days)`: Kiểm thử vi phạm luật ngày nghỉ (0 ngày nghỉ) $\rightarrow$ Trả lỗi `ErrViolationBRAC01`.
  - `TestNewWeeklySchedule/invalid_week_number`: Kiểm thử số tuần không thuộc 1..4.
  - `TestNewWeeklySchedule/mark_day_skipped_per_BR-AC-03_decision_1.1`: Kiểm thử tự động đánh dấu `Skipped` cho buổi tập bị bỏ quên theo Quyết định 1.1 (BR-AC-03).
* **`daily_plan_test.go`**:
  - `TestNewDailyWorkoutPlan/successful_creation`: Kiểm thử sinh giáo án chi tiết hàng ngày (`DailyWorkoutPlan`).
  - `TestNewDailyWorkoutPlan/empty_main_exercises_returns_error`: Kiểm thử bắt lỗi khi danh sách bài tập chính bị rỗng.
  - `TestNewDailyWorkoutPlan/activate_and_complete_plan`: Kiểm thử chuyển trạng thái `Active` $\rightarrow$ `Completed`.
* **`safety_envelope_test.go` (Upper Safety Envelope Domain Service)**:
  - `TestUpperSafetyEnvelopeValidator/BR-AC-02_Load_Adjustment_Ceiling_(+30%)`: Kiểm thử trần tăng tạ vượt $+30\%$ $\rightarrow$ Tự động hạ về $+30\%$ (Max Load Ceiling).
  - `TestUpperSafetyEnvelopeValidator/BR-AC-02_Load_Adjustment_Ceiling_(-30%)`: Kiểm thử trần giảm tạ vượt $-30\%$ $\rightarrow$ Tự động nâng về $-30\%$ (Min Load Ceiling).
  - `TestUpperSafetyEnvelopeValidator/BR-AC-02_Load_Adjustment_Ceiling_within_range`: Kiểm thử mức tạ trong ngưỡng an toàn $\rightarrow$ Giữ nguyên.
  - `TestUpperSafetyEnvelopeValidator/Decision_1.4_Deload_Week_RPE_<=_6_lock`: Kiểm thử Tuần 4 (Deload) $\rightarrow$ Tự động khóa trần RPE $\le 6.0$ theo Quyết định 1.4.
  - `TestUpperSafetyEnvelopeValidator/Active_Injury_Lock_pruning`: Kiểm thử loại bỏ 100% bài tập vi phạm chấn thương active.
  - `TestUpperSafetyEnvelopeValidator/BR-AC-01_Rest_Days_validation`: Kiểm thử hợp lệ lịch tuần có ngày nghỉ.

---

### ⚙️ B. Application Layer Unit Tests (`internal/coaching/application/command/`)
* **`initiate_roadmap_test.go`**:
  - `TestInitiateRoadmapHandler`: Kiểm thử luồng 2A Onboarding Sequential Workflow (Sinh Roadmap $\rightarrow$ Sinh Weekly Schedule Tuần 1 qua Safety Envelope $\rightarrow$ Lưu DB và phát Event).
* **`generate_daily_plan_test.go`**:
  - `TestGenerateDailyPlanHandler`: Kiểm thử luồng 2B Daily Check-in ReAct Agent (Tự động skip buổi cũ, gọi AI Agent, kiểm duyệt qua Load Ceiling $+30\%$).
* **`process_post_workout_test.go`**:
  - `TestProcessPostWorkoutHandler`: Kiểm thử luồng 2C Post-Workout Review & Critique (Đóng buổi $N$, Pre-cache ngầm giáo án cho buổi $N+1$ ở trạng thái `DraftCached`).

---

### 🔌 C. Infrastructure Layer Unit Tests (`internal/coaching/infrastructure/transport/grpc/`)
* **`handler_test.go`**:
  - `TestCoachingGRPCHandler_InitiateRoadmap`: Kiểm thử gRPC Server Handler chuyển đổi Request/Response với Protobuf contract (`InitiateRoadmapRequest`).

---

## 2. CÂU LỆNH CHẠY TOÀN BỘ KIỂM THỬ (EXECUTION COMMANDS)

Chạy tất cả các unit tests của module Coaching:
```bash
go test -v ./internal/coaching/...
```

Chạy kiểm thử đo tỷ lệ bao phủ mã nguồn (Coverage Target > 80%):
```bash
go test -coverprofile=coverage.out ./internal/coaching/...
go tool cover -func=coverage.out
```
