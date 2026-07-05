# FITAI — Thiết Kế Aggregate & DDD Tactical

> Nguồn: [Đặc tả Yêu cầu Nghiệp vụ Cốt lõi BABOK](./NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](./02_bounded_context.md)

---

## 1. Context User Profile & Health

#### Aggregate Root: `User`
- **Nhiệm vụ**: Quản lý định danh, hồ sơ sức khỏe và cấu hình tập luyện.
- **Entities**:
  - `Injury`: Vùng cơ bị thương, ngày báo, trạng thái (`Active` | `Recovered`).
- **Value Objects**:
  - `BiologicalMetrics`: Tuổi, giới tính, chiều cao, cân nặng hiện tại, tỷ lệ mỡ.
  - `TrainingScheduleSlot`: Khung giờ tập cố định.
  - `ChatbotContext`: Thiết bị có sẵn, dị ứng thức ăn.
  - `CoachPersonality`: Phong cách Coach (`DrillSergeant` | `BestFriend` | `DataAnalyst`).
- **Repository**: `UserRepository`
- **Domain Events**:
  - `UserProfileCompleted`: Kích hoạt tạo lộ trình khi hồ sơ đạt ≥ 80%.
  - `InjuryReported`: Kích hoạt đổi giáo án.
  - `InjuryRecovered`: Cho phép phục hồi bài tập đã loại bỏ.
- **Invariants**:
  - `ActiveCoachEnabled = true` chỉ khi hồ sơ đạt ≥ 80% độ hoàn thiện (gồm các chỉ số sinh học bắt buộc và mục tiêu).

#### Aggregate Root: `BodyMetricsHistory`
- **Nhiệm vụ**: Lưu lịch sử thay đổi hình thể theo thời gian.
- **Entities**:
  - `MetricsLogEntry`: Cân nặng, tỷ lệ mỡ, số đo các vòng, ảnh tiến trình, ngày ghi nhận.
- **Repository**: `BodyMetricsHistoryRepository`
- **Domain Events**:
  - `UserMetricsUpdated`: Phát đi kèm giá trị mới nhất khi có `MetricsLogEntry` được thêm.
- **Invariants**:
  - `MetricsLogEntry.weight > 0`.
  - `MetricsLogEntry.recordedAt <= today`.

---

## 2. Context AI Coaching & Planning

#### Aggregate Root: `WorkoutRoadmap`
- **Nhiệm vụ**: Kiểm soát lộ trình tập luyện 4 tuần và trạng thái chu kỳ.
- **Value Objects**:
  - `RoadmapPhase`: Giai đoạn hiện tại và Progressive Overload target.
  - `CompletionRate`: Tỷ lệ hoàn thành tính cuối chu kỳ (CR).
- **Repository**: `WorkoutRoadmapRepository`
- **Domain Events**:
  - `RoadmapInitiated`: Khởi tạo lộ trình đầu tiên.
  - `RoadmapAdjusted`: Điều chỉnh lộ trình (Trigger A).
  - `RoadmapPaused`: Tạm dừng lộ trình (Signal B1, tối đa 4 tuần).
  - `RoadmapResumed`: Tiếp tục lộ trình sau khi tạm dừng.
- **Invariants**:
  - Lifecycle: `Active` → `Paused` → `Resumed` → `Completed`.
  - `RoadmapPaused` tối đa 4 tuần, sau đó tự chuyển về `Active` hoặc hỏi lại user.

#### Aggregate Root: `WeeklySchedule`
- **Nhiệm vụ**: Lịch tập/nghỉ của một tuần cụ thể, tham chiếu `WorkoutRoadmapId` bằng ID.
- **Value Objects**:
  - `MuscleSplit`: Nhóm cơ phân bổ cho từng ngày tập.
  - `DailyPlanIds`: Danh sách ID tham chiếu tới các `DailyWorkoutPlan`.
- **Repository**: `WeeklyScheduleRepository`
- **Domain Events**:
  - `WeeklyScheduleGenerated`: Tạo lịch tuần mới.
  - `ScheduleDayRescheduled`: Dời ngày tập (Signal B2).
- **Invariants**:
  - Tối thiểu 1 ngày nghỉ hoàn toàn, tối đa 6 ngày tập trong tuần (BR-AC-01).
  - Buổi bỏ tập đánh dấu "Bỏ qua", không tự dồn bù chưa có xác nhận (BR-AC-03).

#### Aggregate Root: `DailyWorkoutPlan`
- **Nhiệm vụ**: Giáo án chi tiết một buổi tập, sinh JIT để tránh lock `WeeklySchedule`.
- **Value Objects**:
  - `WorkoutPrescription`: Bài tập, set, rep, tạ gợi ý, warm-up/cool-down.
- **Repository**: `DailyWorkoutPlanRepository`
- **Domain Events**:
  - `DailyWorkoutPlanGenerated`: Giáo án đã được sinh.

#### [Domain Service] `AdaptiveCoachEngine`
- **Nhiệm vụ**: Phát hiện và xử lý 4 tín hiệu hành vi (Signal B1–B4) và đánh giá CR cuối chu kỳ (BR-AC-04).
- **Input**: `WorkoutRoadmap`, `WeeklySchedule`, lịch sử `WorkoutSession`.
- **Signal B1** (BR-AC-05): Không hoạt động 7 ngày → Đề xuất 3 phương án (tiếp tục / đặt lại / tạm dừng).
- **Signal B2** (BR-AC-06): Bỏ tập cùng ngày ≥ 3 lần liên tiếp → Đề xuất dời slot.
- **Signal B3** (BR-AC-07): ≥ 2 buổi/ngày hoặc RPE ≥ 8.5 liên tục ≥ 5 buổi → Cảnh báo, chèn nghỉ bắt buộc.
- **Signal B4** (BR-AC-08): 1RM + Form không tăng 3 tuần liên tiếp (CR ≥ 70%) → Đề xuất Deload / đổi bài / tăng set.

#### [Domain Service] `OverloadValidator`
- **Nhiệm vụ**: Kiểm tra volume `WeeklySchedule` mới không vượt 10% volume thực tế tuần trước (BR-AC-02).

---

## 3. Context Workout Execution & Motion

#### Aggregate Root: `WorkoutSession`
- **Nhiệm vụ**: Kiểm soát buổi tập thực tế, áp dụng quy tắc an toàn.
- **Entities**:
  - `WorkoutSetLog`: Kết quả thực nâng của từng Set (rep, tạ, Form trung bình, RPE).
- **Value Objects**:
  - `SessionSummary`: Tổng set, tổng volume, điểm Form trung bình (N/A nếu Phi AI).
  - `RepLog`: Tọa độ skeleton thô, ROM%, trạng thái lỗi của từng rep (chỉ nhánh AI).
  - `SessionTimer`: Giới hạn thời gian.
- **Repository**: `WorkoutSessionRepository`
- **Domain Events**:
  - `WorkoutSessionStarted`: Bắt đầu buổi tập.
  - `WorkoutSessionCompleted`: Kết thúc buổi tập (mang `SessionSummary`).
  - `WorkoutSessionAborted`: Tự động đóng do quá thời gian → lưu `AnomalousSession`.
  - `BodyMetricUpdated`: User log cân nặng trong buổi tập.
- **Invariants**:
  - `repCount++` chỉ tính khi ROM% ≥ 70% (BR-CC-01).
  - Tỷ lệ frame skeleton hợp lệ < 50% → gắn cờ gian lận (BR-CC-02).
  - Lifecycle: `Scheduled` → `InProgress` → `Completed` | `Aborted (Anomalous)`.
  - Quá 240 phút không tương tác → tự đóng, loại khỏi tính Overload tuần sau (BR-WL-01).

#### [Domain Service] `TrainingLoadGuard`
- **Nhiệm vụ**: Kiểm tra volume buổi tập hiện tại > 250% trung bình 5 buổi gần nhất cùng nhóm cơ — nếu vượt thì yêu cầu xác nhận và chèn ngày nghỉ (BR-WL-02).
- **Lý do tách**: Cần đọc lịch sử nhiều `WorkoutSession` — không thể nằm trong một Aggregate.

#### Aggregate Root: `WorkoutPerformance`
- **Nhiệm vụ**: Lưu kỷ lục cá nhân 1RM, cập nhật qua Eventual Consistency.
- **Entities**:
  - `PersonalRecord`: Kỷ lục 1RM theo Epley Formula cho từng bài tập.
- **Repository**: `WorkoutPerformanceRepository`
- **Domain Events**:
  - `NewPersonalRecordAchieved`: Đạt kỷ lục 1RM mới.

#### Aggregate Root: `MotionSpecification`
- **Nhiệm vụ**: Cấu hình AI và Pose mẫu chuẩn cho từng bài tập.
- **Value Objects**:
  - `PoseTemplate`: Tọa độ khớp chuẩn (33 điểm MediaPipe / 17 điểm YOLO).
  - `CalibrationConfig`: Ngưỡng khoảng cách, góc nghiêng thiết bị.
  - `RepCountingRules`: Ngưỡng ROM% tối thiểu ≥ 70%.
  - `FormScoringRules`: Tiêu chí phát hiện lỗi tư thế.
- **Repository**: `MotionSpecificationRepository`

---

## 4. Context AI Nutrition

#### Aggregate Root: `NutritionPlan`
- **Nhiệm vụ**: Quản lý mục tiêu calo và thực đơn gợi ý trong ngày.
- **Value Objects**:
  - `DailyMealOption`: Gợi ý món ăn cho Sáng, Trưa, Tối, Phụ (tự nấu hoặc ăn ngoài).
  - `CalorieAllocation`: Calo target, tỷ lệ đa lượng Protein/Carb/Fat.
  - `BudgetTier`: Phân khúc giá (Tiết kiệm / Phổ thông / Thoải mái).
- **Repository**: `NutritionPlanRepository`
- **Domain Events**:
  - `NutritionPlanGenerated`: Thực đơn ngày đã được sinh.
- **Invariants**:
  - `CalorieAllocation.target >= 1200 kcal/ngày` (BR-NU-01).

#### Aggregate Root: `MealHistory`
- **Nhiệm vụ**: Theo dõi lịch sử ăn uống và kiểm soát chống lặp món.
- **Entities**:
  - `MealLog`: Món ăn thực tế đã ghi, có thể sửa/xóa (có lifecycle riêng).
- **Value Objects**:
  - `LockoutRegistry`: Nguyên liệu đang bị khóa và ngày mở khóa.
- **Repository**: `MealHistoryRepository`
- **Domain Events**:
  - `MealLogged`: Ghi nhận bữa ăn thành công.
  - `LockoutApplied`: Khóa nguyên liệu thành công.
- **Invariants**:
  - Thêm `MealLog` mới → tự động cập nhật `LockoutRegistry`: Protein 7 ngày, Carb 5 ngày, Chủ đề món 3 ngày (BR-NU-02).

---

## 5. Context Catalog

#### Aggregate Root: `Exercise`
- **Nhiệm vụ**: Thư viện bài tập chuẩn, quản lý vòng đời phê duyệt.
- **Value Objects**:
  - `ExerciseInfo`: Tên bài, nhóm cơ chính/phụ, video hướng dẫn URL, dụng cụ, bài thay thế.
- **Repository**: `ExerciseRepository`
- **Domain Events**:
  - `ExerciseCreated`: Bài tập tạo mới (trạng thái `Draft`).
  - `ExerciseApproved`: Admin phê duyệt → trạng thái `Active`.
- **Invariants**:
  - Lifecycle: `Draft` → `PendingApproval` → `Active`.
  - Chỉ bài tập `Active` mới được tham chiếu bởi các Context khác.

#### Aggregate Root: `FoodItem`
- **Nhiệm vụ**: Thư viện thực phẩm chuẩn, quản lý vòng đời phê duyệt.
- **Value Objects**:
  - `FoodNutrient`: Tên, calo, macro trên 100g, nhãn chay/Halal, nhãn dị ứng.
- **Repository**: `FoodItemRepository`
- **Domain Events**:
  - `FoodItemCreated`: Thực phẩm tạo mới (trạng thái `Draft`).
  - `FoodItemApproved`: Admin phê duyệt → trạng thái `Active`.
- **Invariants**:
  - Lifecycle: `Draft` → `PendingApproval` → `Active`.
