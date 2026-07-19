# MỤC LỤC KỊCH BẢN KIỂM THỬ — MODULE COACHING & PLANNING

Tài liệu này mô tả danh mục các test suite, phạm vi kiểm thử và cách thức thực thi cho module **Coaching & Planning** (Bounded Context `coaching`).

---

## 1. Danh Mục Test Suite

| Test Suite | Đường Dẫn File | Mục Tiêu Kiểm Thử |
| :--- | :--- | :--- |
| **WorkoutRoadmap Unit Test** | `internal/coaching/domain/roadmap_test.go` | Kiểm tra Aggregate Root `WorkoutRoadmap`: Khởi tạo lộ trình 4 tuần, kiểm tra validation bắt buộc (ID, UserID), và chuyển đổi trạng thái vòng đời (`Active` → `Paused` → `Resumed` → `Completed`). |
| **WeeklySchedule Unit Test** | `internal/coaching/domain/schedule_test.go` | Kiểm tra Aggregate Root `WeeklySchedule`: Phân bổ ngày tập theo kinh nghiệm (`beginner`: 3 ngày, `intermediate`: 4 ngày, `advanced`: 5 ngày), đảm bảo invariant `BR-AC-01` (tối thiểu 1 ngày nghỉ), và gán ID giáo án ngày. |
| **DailyWorkoutPlan Unit Test** | `internal/coaching/domain/plan_test.go` | Kiểm tra Aggregate Root `DailyWorkoutPlan`: Khởi tạo giáo án ngày ở trạng thái `DRAFT`, lưu đơn bài tập (`WorkoutPrescription`), và các chuyển đổi trạng thái (`Draft` → `Active` → `Completed`). |
| **InitiateRoadmap Command Test** | `internal/coaching/application/command/initiate_roadmap_test.go` | Kiểm tra Command Handler `InitiateRoadmapHandler`: Luồng khởi tạo lộ trình 4 tuần + lịch tuần 1 + 3 giáo án ngày cho beginner. Kiểm tra tính Idempotent (nếu user đã có roadmap `ACTIVE` thì bỏ qua không tạo đè). |
| **ProfileCompleted Event Handler Test** | `internal/coaching/application/event/profile_completed_handler_test.go` | Kiểm tra Event Handler `ProfileCompletedHandler`: Nhận event `ProfileCompleted`, map thông tin (UserID, Goals, Injuries, ExperienceLevel) sang `InitiateRoadmapCommand` và dispatch tới command handler. |

---

## 2. Hướng Dẫn Chạy Test

### Chạy toàn bộ Unit Test của module Coaching
```bash
go test ./internal/coaching/... -v -cover
```

### Chạy riêng Domain Unit Tests
```bash
go test ./internal/coaching/domain/... -v -cover
```

### Chạy riêng Application Command & Event Unit Tests
```bash
go test ./internal/coaching/application/... -v -cover
```
