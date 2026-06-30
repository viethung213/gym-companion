# 7. Thiết Kế Aggregate (Aggregate Design) - FITAI

Tài liệu này xác định các **Aggregate (Cụm thực thể)**, **Aggregate Root (Thực thể gốc)**, **Entities (Thực thể)**, **Value Objects (Đối tượng giá trị)** và các biên giao dịch (transactional boundaries) trong hệ thống **FITAI**.

---

## 7.1 Cấu Trúc Các Aggregate Theo Ngữ Cảnh

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ USER PROFILE CONTEXT                                                         │
│   [Aggregate Root: User] ──► Entities: (None)                                │
│                          ──► Value Objects: BiologicalMetrics, Injury        │
│   [Aggregate Root: BodyMetricsHistory] ──► Entities: MetricsLogEntry         │
└──────────────────────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────────────────────┐
│ AI COACHING CONTEXT                                                          │
│   [Aggregate Root: WorkoutPlan] ──► Entities: WeeklySchedule, ExerciseSchedule│
│                                 ──► Value Objects: ExerciseSpec              │
│   [Aggregate Root: CoachConfig] ──► Value Objects: CoachPersonality          │
└──────────────────────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────────────────────┐
│ WORKOUT EXECUTION CONTEXT                                                    │
│   [Aggregate Root: WorkoutSession] ──► Entities: WorkoutSetLog               │
│                                    ──► Value Objects: SessionSummary, RepLog │
└──────────────────────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────────────────────┐
│ AI NUTRITION CONTEXT                                                         │
│   [Aggregate Root: NutritionPlan] ──► Entities: DailyMealOption              │
│                                   ──► Value Objects: CalorieAllocation       │
│   [Aggregate Root: MealHistory]   ──► Entities: MealLog                      │
│                                   ──► Value Objects: LockoutRegistry         │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 7.2 Chi Tiết Thiết Kế Từng Aggregate

### 1. Ngữ cảnh User Profile & Health
#### Aggregate Root: `User`
* **Nhiệm vụ**: Quản lý định danh người dùng và kiểm soát hồ sơ sức khỏe.
* **Entities liên kết**: Không có (để giữ mô hình phẳng và tối ưu hiệu năng).
* **Value Objects**:
  * `BiologicalMetrics`: Chiều cao, cân nặng, tỷ lệ mỡ, tuổi, giới tính. (Bất biến, thay đổi bằng cách thay thế toàn bộ đối tượng).
  * `Injury`: Vùng cơ bị thương (ví dụ: `Shoulder`, `Knee`), ngày báo chấn thương, trạng thái (`Active`, `Healed`).
* **Invariants (Quy tắc bất biến tại ranh giới)**:
  * Trạng thái `User` chỉ có thể chuyển sang `ActiveCoachEnabled = true` khi `ProfileCompletionRate` (tính từ các trường bắt buộc của `BiologicalMetrics`) $\ge 80\%$.

#### Aggregate Root: `BodyMetricsHistory`
* **Nhiệm vụ**: Theo dõi tiến trình thay đổi hình thể bằng cách lưu trữ lịch sử các chỉ số sinh học của người dùng theo thời gian.
* **Entities liên kết**:
  * `MetricsLogEntry`: Ghi nhận cân nặng, tỷ lệ mỡ (body fat %), số đo các vòng, ảnh chụp tiến trình tại một thời điểm (ngày ghi nhận).
* **Invariants (Quy tắc bất biến tại ranh giới)**:
  * Khi có bản ghi `MetricsLogEntry` mới được thêm vào, hệ thống tự động phát đi sự kiện `UserMetricsUpdated` mang giá trị mới nhất để đồng bộ sang Aggregate `User` (cập nhật chỉ số hiện tại) và kích hoạt việc tính toán lại kế hoạch tập luyện (AI Coaching) cũng như calo nạp vào (AI Nutrition).

---

### 2. Ngữ cảnh AI Coaching & Planning
#### Aggregate Root: `WorkoutPlan`
* **Nhiệm vụ**: Kiểm soát kế hoạch luyện tập 4 tuần và tiến trình tăng tải.
* **Entities liên kết**:
  * `WeeklySchedule`: Lịch các ngày tập/nghỉ trong tuần.
  * `ExerciseSchedule`: Danh sách bài tập phân bổ cho một ngày tập cụ thể.
* **Value Objects**:
  * `ExerciseSpec`: Chỉ số bài tập gồm `targetSets`, `targetReps`, `targetWeight`, `targetRPE`.
* **Invariants (Quy tắc bất biến tại ranh giới)**:
  * Tổng volume tạ dự kiến của một `WeeklySchedule` tiếp theo không được vượt quá **10%** volume của tuần hiện tại (`Progressive Overload Rule`).
  * Một `WeeklySchedule` bắt buộc phải chứa tối thiểu **1 ngày nghỉ** hoàn toàn (`Rest Day Invariant`) và tối đa **6 ngày tập**.

#### Aggregate Root: `CoachConfig`
* **Nhiệm vụ**: Quản lý tương tác và phong cách huấn luyện viên ảo.
* **Value Objects**:
  * `CoachPersonality`: Phong cách (`DrillSergeant`, `BestFriend`, `DataAnalyst`).

---

### 3. Ngữ cảnh Workout Execution
#### Aggregate Root: `WorkoutSession`
* **Nhiệm vụ**: Kiểm soát một buổi tập đang diễn ra thực tế dưới camera và chống gian lận.
* **Entities liên kết**:
  * `WorkoutSetLog`: Kết quả thực nâng của từng Set (số rep đếm được, cân nặng thực tế, điểm Form trung bình của Set, RPE).
* **Value Objects**:
  * `SessionSummary`: Tổng số Set hoàn thành, tổng volume nâng thực tế, điểm kỹ thuật trung bình toàn bộ buổi tập.
  * `RepLog`: Ghi nhận tọa độ skeleton thô, ROM% và trạng thái lỗi của từng rep cụ thể phục vụ chấm điểm.
* **Invariants (Quy tắc bất biến tại ranh giới)**:
  * Một rep chỉ được ghi nhận tăng số đếm (`repCount++`) nếu `ROM% >= 70%` (`Valid Rep Invariant`).
  * Khi lưu `WorkoutSession`, nếu tổng tỷ lệ khung hình có skeleton hợp lệ / tổng thời gian quay hình `< 50%`, buổi tập phải bị gắn cờ cảnh cáo gian lận (`AntiCheat Invariant`).

---

### 4. Ngữ cảnh AI Nutrition
#### Aggregate Root: `NutritionPlan`
* **Nhiệm vụ**: Quản lý kế hoạch calo và đề xuất thực đơn cho người dùng.
* **Entities liên kết**:
  * `DailyMealOption`: Các gợi ý món ăn cho Sáng, Trưa, Tối, Phụ.
* **Value Objects**:
  * `CalorieAllocation`: Calo tiêu chuẩn nạp vào cơ thể, chỉ số dinh dưỡng đa lượng (Protein/Carb/Fat).
* **Invariants (Quy tắc bất biến tại ranh giới)**:
  * Năng lượng mục tiêu của `CalorieAllocation` tuyệt đối không được nhỏ hơn **1200 kcal/ngày** (`BioSafety Invariant`).

#### Aggregate Root: `MealHistory`
* **Nhiệm vụ**: Theo dõi lịch sử ăn uống thực tế và áp dụng luật chống lặp món.
* **Entities liên kết**:
  * `MealLog`: Các món ăn thực tế đã ghi chép.
* **Value Objects**:
  * `LockoutRegistry`: Danh sách các nguyên liệu (Protein, Carb, Chủ đề món) đang bị khóa và ngày mở khóa tương ứng.
* **Invariants (Quy tắc bất biến tại ranh giới)**:
  * Khi thêm một món ăn mới vào `MealLog`, hệ thống tự động cập nhật `LockoutRegistry` để khóa nguyên liệu Protein chính trong 7 ngày, Carb trong 5 ngày và Chủ đề món trong 3 ngày (`Anti-Repetition Invariant`).
