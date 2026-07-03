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
│   [Aggregate Root: WorkoutRoadmap] ──► Entities: WeeklySchedule              │
│                                    ──► Value Objects: MuscleSplit            │
│   [Aggregate Root: CoachConfig]    ──► Value Objects: CoachPersonality       │
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
#### Aggregate Root: `WorkoutRoadmap`
* **Nhiệm vụ**: Kiểm soát lộ trình định hướng tập luyện 4 tuần và lịch tuần phân bổ nhóm cơ tập luyện (WeeklySplit). Lưu ý giáo án chi tiết từng bài tập/set/rep/tạ không nằm trong Aggregate này mà được sinh JIT dưới dạng Specification riêng biệt tại thời điểm tập.
* **Entities liên kết**:
  * `WeeklySchedule`: Lịch các ngày tập/nghỉ trong tuần.
* **Value Objects**:
  * `MuscleSplit`: Nhóm cơ chính được phân bổ cho ngày tập cụ thể (ví dụ: Thứ Hai - Chest/Triceps, Thứ Ba - Nghỉ).
* **Invariants (Quy tắc bất biến tại ranh giới)**:
  * Tổng volume tạ dự kiến của một `WeeklySchedule` tiếp theo do AI sinh JIT không được vượt quá **10%** volume của tuần trước đó (`Progressive Overload Rule` - BR-AC-02).
  * Một `WeeklySchedule` bắt buộc phải chứa tối thiểu **1 ngày nghỉ hoàn toàn** trong tuần (`Rest Day Invariant` - BR-AC-01) và tối đa **6 ngày tập**.
  * Giáo án các buổi bỏ tập đánh dấu là "Bỏ qua", không tự động dồn/bù vào ngày tiếp theo nếu chưa có xác nhận từ người dùng (BR-AC-03).

#### Aggregate Root: `CoachConfig`
* **Nhiệm vụ**: Quản lý tương tác và phong cách huấn luyện viên ảo.
* **Value Objects**:
  * `CoachPersonality`: Phong cách (`DrillSergeant`, `BestFriend`, `DataAnalyst`).

---

### 3. Ngữ cảnh Workout Execution
#### Aggregate Root: `WorkoutSession`
* **Nhiệm vụ**: Kiểm soát một buổi tập đang diễn ra thực tế (cả AI Camera và Phi AI) và áp dụng các quy tắc an toàn/chống gian lận.
* **Entities liên kết**:
  * `WorkoutSetLog`: Kết quả thực nâng của từng Set (số rep thực tế, cân nặng thực tế, điểm Form trung bình của Set, RPE).
* **Value Objects**:
  * `SessionSummary`: Tổng số Set hoàn thành, tổng volume nâng thực tế, điểm kỹ thuật trung bình toàn bộ buổi tập (Form Score trung bình = N/A nếu tập Non-AI).
  * `RepLog` (chỉ dùng cho tập AI): Ghi nhận tọa độ skeleton thô, ROM% và trạng thái lỗi của từng rep cụ thể phục vụ chấm điểm.
* **Invariants (Quy tắc bất biến tại ranh giới)**:
  * [Nhánh AI] Một rep chỉ được ghi nhận tăng số đếm (`repCount++`) nếu `ROM% >= 70%` (`Valid Rep Invariant` - BR-CC-01).
  * [Nhánh AI] Khi lưu `WorkoutSession`, nếu tổng tỷ lệ khung hình có skeleton hợp lệ / tổng thời gian quay hình `< 50%`, buổi tập phải bị gắn cờ cảnh cáo gian lận (`AntiCheat Invariant` - BR-CC-02).
  * **Giới hạn thời gian (BR-WL-01)**: Nếu buổi tập vượt quá 240 phút không tương tác, hệ thống tự động đóng buổi tập, lưu nhãn `Anomalous Session` và loại bỏ khỏi việc tính toán Overload tuần sau.
  * **Tải lượng bất thường (BR-WL-02)**: Nếu tải lượng (volume) buổi tập vượt 250% trung bình 5 buổi gần nhất của cùng nhóm cơ, bắt buộc phải yêu cầu người dùng xác nhận và tự động chèn ít nhất 1 ngày nghỉ hoàn toàn cho nhóm cơ đó.

---

### 4. Ngữ cảnh AI Nutrition
#### Aggregate Root: `NutritionPlan`
* **Nhiệm vụ**: Quản lý kế hoạch calo nạp vào và đề xuất thực đơn linh hoạt cho người dùng.
* **Entities liên kết**:
  * `DailyMealOption`: Các gợi ý món ăn cho Sáng, Trưa, Tối, Phụ. Hỗ trợ cả cờ tự nấu (Self-cooked) và ăn ngoài tiệm (Dining-out).
* **Value Objects**:
  * `CalorieAllocation`: Calo tiêu chuẩn nạp vào cơ thể, chỉ số dinh dưỡng đa lượng (Protein/Carb/Fat).
* **Invariants (Quy tắc bất biến tại ranh giới)**:
  * Năng lượng mục tiêu của `CalorieAllocation` tuyệt đối không được nhỏ hơn **1200 kcal/ngày** (`BioSafety Invariant` - BR-NU-01).

#### Aggregate Root: `MealHistory`
* **Nhiệm vụ**: Theo dõi lịch sử ăn uống thực tế và áp dụng luật chống lặp món.
* **Entities liên kết**:
  * `MealLog`: Các món ăn thực tế đã ghi chép.
* **Value Objects**:
  * `LockoutRegistry`: Danh sách các nguyên liệu (Protein, Carb, Chủ đề món) đang bị khóa và ngày mở khóa tương ứng.
* **Invariants (Quy tắc bất biến tại ranh giới)**:
  * Khi thêm một món ăn mới vào `MealLog`, hệ thống tự động cập nhật `LockoutRegistry` để khóa nguyên liệu Protein chính trong 7 ngày, Carb trong 5 ngày và Chủ đề món trong 3 ngày (`Anti-Repetition Invariant` - BR-NU-02).
