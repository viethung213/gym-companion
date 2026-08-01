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
  - `PrimaryGoal`: Mục tiêu chính (`PRIMARY_GOAL_MUSCLE_GAIN` hoặc `PRIMARY_GOAL_FAT_LOSS`).
  - `AvailableEquipment`: Danh sách dụng cụ có sẵn (luôn mặc định chứa `BODYWEIGHT`).
  - `PreferredMuscleGroups`: Danh sách nhóm cơ ưu tiên.
  - `AvailableSlots`: Khung giờ rảnh trong tuần.
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

#### Aggregate Root: `Roadmap`
- **Nhiệm vụ**: Quản lý toàn bộ Lộ trình 4 tuần (28 ngày), 4 `WeekPlan` và các `DayPlan` ngày tập theo khung giờ `available_slots`.
- **Entities & Value Objects**:
  - `WeekPlan` (Entity): Đại diện cho 1 tuần tập (chứa các `DayPlan` ngày tập và `muscle_split_type`).
  - `DayPlan` (Entity): Quản lý 1 ngày tập trên lịch (`scheduled_date`, `available_slots`). Các ngày không có `DayPlan` được ngầm hiểu là ngày nghỉ.
  - `SessionPlan` (Entity): Kế hoạch buổi tập gắn theo khung giờ rảnh trong ngày (`slot_time`, `status`: `PENDING`, `COMPLETED`, `SKIPPED`, `ABORTED`, `prescription` JSONB).
  - `RoadmapPhase`: Giai đoạn hiện tại (`Accumulation`, `Overload`, `Peak`, `Deload`) và RPE target.
- **Repository**: `RoadmapRepository`
- **Domain Events**:
  - `RoadmapInitiated`: Khởi tạo thành công lộ trình 4 tuần (gồm 4 `WeekPlan`, các `DayPlan` ngày tập và `SessionPlan` ban đầu).
  - `RoadmapAdjusted`: Phát ra khi hệ thống tự động re-generate hoặc điều chỉnh lại giáo án các buổi tập chưa thực thi (`status = PENDING`) do tín hiệu thích ứng (Signal B1-B4, Trigger A) hoặc do người dùng cập nhật Profile. Các phân hệ khác (VD: Dinh dưỡng, Nhắc lịch) lắng nghe event này để cập nhật kế hoạch tương ứng.
  - `SessionPlanExecuted`: Phát ra ngay khi người dùng hoàn thành một buổi tập (`SessionPlan.status` chuyển thành `COMPLETED`).
- **Invariants (Các ràng buộc điều kiện bất biến)**:
  - **Vòng đời Lộ trình (Roadmap Lifecycle)**: Trạng thái của Aggregate Root `Roadmap` chỉ đi từ `ACTIVE` $\rightarrow$ `COMPLETED` (khi hoàn thành 4 tuần).
  - **Giới hạn số buổi tập tuần (`BR-AC-01`)**: Lịch tập tối đa **6 buổi/tuần**.
  - **Xử lý Bỏ tập (`BR-AC-03`)**: Ngày tập trôi qua mà không thực hiện sẽ tự động chuyển sang trạng thái **`SKIPPED`** (tính 0 set hoàn thành vào $SCR$). Lịch tập của các ngày tiếp theo giữ nguyên, tuyệt đối không dồn dời lịch.

#### [Domain Service] `AdaptiveCoachEngine`
- **Nhiệm vụ**: Phát hiện, đánh giá và thực thi các quy tắc thích ứng huấn luyện.
- **Input**: `Roadmap`, lịch sử `WorkoutSession`.
- **Quy tắc thực thi**:
  - [BR-AC-04](NGHIEP_VU_COT_LOI_BABOK.md#L135): Trigger A — Quy tắc Thích ứng & Điều chỉnh Định kỳ.
  - [BR-AC-05](NGHIEP_VU_COT_LOI_BABOK.md#L136): Signal B1 — Bỏ tập cấp tính / Mất tích.
  - [BR-AC-06](NGHIEP_VU_COT_LOI_BABOK.md#L137): Signal B2 — Kẹt lịch cố định (Pattern Mismatch).
  - [BR-AC-07](NGHIEP_VU_COT_LOI_BABOK.md#L138): Signal B3 — Tập ngoài lịch (Unscheduled Workout).
  - [BR-AC-08](NGHIEP_VU_COT_LOI_BABOK.md#L139): Signal B4 — Cơ bắp bị chai lỳ (Plateau).
  - [BR-AC-09](NGHIEP_VU_COT_LOI_BABOK.md#L140): Thích ứng sau phục hồi chấn thương (Post-Injury Adaptation).

#### [Domain Service] `OverloadValidator`
- **Nhiệm vụ**: Kiểm soát giới hạn biên độ điều chỉnh tải trọng.
- **Quy tắc thực thi**: [BR-AC-02](NGHIEP_VU_COT_LOI_BABOK.md#L129) — Giới hạn điều chỉnh tải trọng $\pm 30\%$.

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
  - `PoseTemplate`: Tọa độ khớp chuẩn (17 điểm MMPose).
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

---

## 5. Context Exercise

#### Aggregate Root: `Exercise`
- **Nhiệm vụ**: Thư viện bài tập chuẩn, quản lý vòng đời phê duyệt.
- **Value Objects**:
  - `ExerciseInfo`: Tên bài, nhóm cơ chính/phụ, video hướng dẫn URL, dụng cụ, bài thay thế.
- **Repository**: `ExerciseRepository`
- **Domain Events**:
  - `ExerciseCreated`: Bài tập tạo mới (trạng thái `Draft`).
  - `ExerciseSubmittedForApproval`: Bài tập được gửi vào hàng chờ duyệt.
  - `ExerciseApproved`: Admin phê duyệt → trạng thái `Active`.
  - `ExerciseArchived`: Admin lưu trữ bài tập, thay cho xóa vật lý.
- **Invariants**:
  - Lifecycle kích hoạt: `Draft` → `PendingApproval` → `Active`.
  - Lifecycle lưu trữ: `Draft` | `PendingApproval` | `Active` → `Archived`.
  - Chỉ bài tập `Active` mới được tham chiếu bởi các Context khác.
  - Bài tập `Archived` không được trả về trong luồng User đọc/tìm kiếm và không được cập nhật nội dung.
