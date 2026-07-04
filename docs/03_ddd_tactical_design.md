# FITAI — Thiết Kế Aggregate & DDD Tactical

> Nguồn: [Đặc tả Yêu cầu Nghiệp vụ Cốt lõi BABOK](./NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](./02_bounded_context.md)

---

## 1. Đặc Tả Chi Tiết Từng Aggregate

### 1.1 Ngữ cảnh User Profile & Health

#### Aggregate Root: `User`
- **Nhiệm vụ**: Quản lý định danh người dùng và kiểm soát hồ sơ sức khỏe.
- **Entities**:
  - `Injury`: Vùng cơ bị thương (ví dụ: `Shoulder`, `Knee`), ngày báo chấn thương, trạng thái (`Active`, `Healed`). (Theo dõi vòng đời chấn thương).
- **Value Objects**:
  - `BiologicalMetrics`: Tuổi, giới tính, chiều cao, cân nặng hiện tại, tỷ lệ mỡ hiện tại. (Bất biến, thay thế toàn bộ đối tượng).
  - `TrainingScheduleSlot`: Khung giờ tập cố định [FR-UM-03].
  - `ChatbotContext`: Thiết bị có sẵn, dị ứng thức ăn [Assumption-03].
- **Repository**: `UserRepository` (CRUD thông tin User và Injury).
- **Domain Events**:
  - `UserProfileCompleted`: Kích hoạt tạo lộ trình khi hồ sơ đạt ≥ 80%.
  - `InjuryReported`: Kích hoạt đổi giáo án khi có chấn thương mới.
- **Invariants (Quy tắc bất biến tại ranh giới)**:
  - Trạng thái `User` chỉ có thể chuyển sang `ActiveCoachEnabled = true` khi hồ sơ đạt ≥ 80% độ hoàn thiện.
- **Chia sẻ (Ubiquitous Language)**:
  - `BiologicalMetrics` (Shared VO) ──► Tiêu thụ bởi `Nutrition`, `Coaching`.
  - `Injury` (Shared Entity) ──► Tiêu thụ bởi `Coaching`.

#### Aggregate Root: `BodyMetricsHistory`
- **Nhiệm vụ**: Theo dõi tiến trình thay đổi hình thể theo thời gian.
- **Entities**: Không có.
- **Value Objects**:
  - `MetricsLogEntry`: Ghi nhận cân nặng, tỷ lệ mỡ (body fat %), số đo các vòng, ảnh tiến trình tại một thời điểm (ngày ghi nhận). (Snapshot dữ liệu bất biến).
- **Repository**: `BodyMetricsHistoryRepository` (Thêm mới MetricsLogEntry, truy vấn lịch sử).
- **Domain Events**:
  - `UserMetricsUpdated`: Phát đi khi có `MetricsLogEntry` mới được thêm vào.
- **Invariants (Quy tắc bất biến tại ranh giới)**:
  - Thêm bản ghi `MetricsLogEntry` mới sẽ tự động trigger Event `UserMetricsUpdated` mang giá trị mới nhất để đồng bộ.
- **Chia sẻ (Ubiquitous Language)**:
  - `MetricsLogEntry` (Shared VO) ──► Event `UserMetricsUpdated` ──► Đồng bộ `BiologicalMetrics` (`User`), điều chỉnh calo (`Nutrition`), điều chỉnh lộ trình (`Coaching`).

---

### 1.2 Ngữ cảnh AI Coaching & Planning

#### Aggregate Root: `WorkoutRoadmap`
- **Nhiệm vụ**: Kiểm soát lộ trình tập luyện 4 tuần và lịch tuần.
- **Entities**:
  - `WeeklySchedule`: Lịch các ngày tập/nghỉ trong tuần.
- **Value Objects**:
  - `MuscleSplit`: Nhóm cơ chính được phân bổ cho ngày tập cụ thể.
  - `WorkoutPlanIds`: Danh sách ID của các `DailyWorkoutPlan` con trong tuần (ID-based reference để tránh lock dữ liệu khi tập).
- **Repository**: `WorkoutRoadmapRepository` (Lưu lộ trình và lịch tuần).
- **Invariants (Quy tắc bất biến tại ranh giới)**:
  - Một `WeeklySchedule` bắt buộc chứa tối thiểu 1 ngày nghỉ hoàn toàn và tối đa 6 ngày tập (BR-AC-01).
  - Giáo án các buổi bỏ tập đánh dấu là "Bỏ qua", không tự động dồn/bù nếu chưa có xác nhận (BR-AC-03).
- **Chia sẻ (Ubiquitous Language)**:
  - `WorkoutRoadmap` (Shared Aggregate) ──► Tiêu thụ bởi `Workout Session` (quét lộ trình).

#### Aggregate Root: `DailyWorkoutPlan`
- **Nhiệm vụ**: Giáo án chi tiết một ngày tập, được sinh JIT dưới dạng Specification riêng biệt tại thời điểm tập để tránh lock `WeeklySchedule`.
- **Entities**: Không có.
- **Value Objects**:
  - `WorkoutPrescription`: Bài tập, set, rep, mức tạ gợi ý cho từng bài, bao gồm warm-up/cool-down.
- **Repository**: `DailyWorkoutPlanRepository` (Lưu giáo án JIT chi tiết của từng ngày).
- **Domain Events**:
  - `DailyWorkoutPlanGenerated`: Giáo án chi tiết ngày đã được sinh (JIT).
- **Chia sẻ (Ubiquitous Language)**:
  - `WorkoutPrescription` (Shared VO) ──► Tiêu thụ bởi `Workout Session` (thực thi buổi tập).

#### Aggregate Root: `CoachConfig`
- **Nhiệm vụ**: Quản lý tương tác và phong cách huấn luyện viên ảo.
- **Entities**: Không có.
- **Value Objects**:
  - `CoachPersonality`: Phong cách (`DrillSergeant`, `BestFriend`, `DataAnalyst`).
- **Repository**: `CoachConfigRepository` (Lưu cấu hình Coach của user).
- **Chia sẻ (Ubiquitous Language)**:
  - `CoachPersonality` (Shared VO) ──► Tiêu thụ bởi `Notification` (Hạ tầng gửi tin).

#### [Domain Service] OverloadValidator
- **Nhiệm vụ**: Kiểm tra volume của `WeeklySchedule` tiếp theo do AI sinh JIT không được vượt quá 10% volume của tuần trước đó (BR-AC-02).

---

### 1.3 Ngữ cảnh Workout Execution

#### Aggregate Root: `WorkoutSession`
- **Nhiệm vụ**: Kiểm soát một buổi tập đang diễn ra thực tế (cả AI Camera và Phi AI) và áp dụng các quy tắc an toàn/chống gian lận.
- **Entities**:
  - `WorkoutSetLog`: Kết quả thực nâng của từng Set (rep, tạ, Form trung bình của Set, RPE).
- **Value Objects**:
  - `SessionSummary`: Tổng set, tổng volume nâng thực tế, điểm Form trung bình buổi tập (= N/A nếu tập Phi AI).
  - `RepLog` (Chỉ dùng cho AI): Ghi nhận tọa độ skeleton thô, ROM% và trạng thái lỗi của từng rep.
  - `SessionTimer`: Cấu hình giới hạn thời gian (90/180/240 phút) [BR-WL-01].
- **Repository**: `WorkoutSessionRepository` (Lưu nhật ký buổi tập đang diễn ra hoặc đã hoàn thành).
- **Domain Events**:
  - `WorkoutSessionStarted`: Bắt đầu buổi tập.
  - `WorkoutSessionCompleted`: Kết thúc buổi tập (mang theo `SessionSummary`).
  - `WorkoutSessionAborted`: Tự động đóng do quá thời gian.
  - `BodyMetricUpdated`: User log cân nặng mới trong buổi tập.
- **Invariants (Quy tắc bất biến tại ranh giới)**:
  - [Nhánh AI] repCount++ chỉ được tính khi ROM% >= 70% (BR-CC-01).
  - [Nhánh AI] Cảnh báo gian lận khi tỷ lệ khung hình skeleton hợp lệ / tổng thời gian quay < 50% (BR-CC-02).
  - Giới hạn thời gian (BR-WL-01): Vượt quá 240 phút không tương tác → tự động đóng, lưu Anomalous Session và loại bỏ khỏi Overload tuần sau.
  - Tải lượng bất thường (BR-WL-02): Volume buổi tập vượt 250% trung bình 5 buổi gần nhất của cùng nhóm cơ → yêu cầu xác nhận và tự động chèn ít nhất 1 ngày nghỉ cho nhóm cơ đó.
- **Chia sẻ (Ubiquitous Language)**:
  - `SessionSummary` (Shared VO) ──► Event `WorkoutSessionCompleted` ──► Tiêu thụ bởi `Coaching` (thuật toán thích ứng).
  - `BodyMetricUpdated` (Shared Event) ──► Tiêu thụ bởi `User Profile` (ghi nhận cân nặng mới).

#### Aggregate Root: `WorkoutPerformance`
- **Nhiệm vụ**: Lưu trữ kỷ lục cá nhân phục vụ CQRS Read Model.
- **Entities**:
  - `PersonalRecord`: Kỷ lục 1RM ước tính theo Epley Formula cho từng bài tập.
- **Repository**: `WorkoutPerformanceRepository` (CRUD kỷ lục 1RM).
- **Domain Events**:
  - `NewPersonalRecordAchieved`: Đạt kỷ lục 1RM mới (Vinh danh PR).
- **Hoạt động Consistency**: Cập nhật PR thông qua **Eventual Consistency** bằng cách lắng nghe sự kiện `WorkoutSessionCompleted`.
- **Chia sẻ (Ubiquitous Language)**:
  - `PersonalRecord` (Shared Entity) ──► Tiêu thụ bởi `Coaching` (tính overload và giới hạn tạ).

#### Aggregate Root: `MotionSpecification`
- **Nhiệm vụ**: Cấu hình AI và Pose mẫu chuẩn cho từng bài tập (được package `motion` tiêu thụ và phân phối cho Client).
- **Entities**: Không có.
- **Value Objects**:
  - `PoseTemplate`: Tọa độ khớp chuẩn (33 điểm MediaPipe hoặc 17 điểm YOLO).
  - `CalibrationConfig`: Ngưỡng khoảng cách, góc nghiêng điện thoại.
  - `RepCountingRules`: Ngưỡng góc ROM% tối thiểu (≥ 70%) để đếm rep [BR-CC-01].
  - `FormScoringRules`: Tiêu chí phát hiện lỗi tư thế (võng lưng, gối chụm...).
- **Repository**: `MotionSpecificationRepository` (CRUD cấu hình AI bài tập).
- **Chia sẻ (Ubiquitous Language)**:
  - `PoseTemplate` / `CalibrationConfig` (Shared VO) ──► Tiêu thụ bởi `Edge AI` (Client tracking).

---

### 1.4 Ngữ cảnh AI Nutrition

#### Aggregate Root: `NutritionPlan`
- **Nhiệm vụ**: Quản lý kế hoạch calo nạp vào và đề xuất thực đơn linh hoạt cho người dùng.
- **Entities**: Không có.
- **Value Objects**:
  - `DailyMealOption`: Gợi ý món ăn cho Sáng, Trưa, Tối, Phụ (hỗ trợ cờ tự nấu hoặc ăn ngoài). (Bất biến, thay thế khi đổi món).
  - `CalorieAllocation`: Calo tiêu chuẩn nạp vào cơ thể, chỉ số dinh dưỡng đa lượng (Protein/Carb/Fat).
  - `BudgetTier`: Phân khúc giá (Tiết kiệm, Phổ thông, Thoải mái).
- **Repository**: `NutritionPlanRepository` (Lưu thực đơn gợi ý trong ngày).
- **Domain Events**:
  - `NutritionPlanGenerated`: Thực đơn ngày đã được sinh.
- **Invariants (Quy tắc bất biến tại ranh giới)**:
  - Năng lượng mục tiêu của `CalorieAllocation` tuyệt đối không được nhỏ hơn 1200 kcal/ngày (BR-NU-01).

#### Aggregate Root: `MealHistory`
- **Nhiệm vụ**: Theo dõi lịch sử ăn uống thực tế và áp dụng luật chống lặp món.
- **Entities**: Không có.
- **Value Objects**:
  - `MealLog`: Các món ăn thực tế đã ghi chép. (Dòng ghi nhận lịch sử bất biến).
  - `LockoutRegistry`: Danh sách các nguyên liệu (Protein, Carb, Chủ đề món) đang bị khóa và ngày mở khóa tương ứng.
- **Repository**: `MealHistoryRepository` (Thêm log ăn uống, truy vấn danh sách lockout).
- **Domain Events**:
  - `MealLogged`: Ghi nhận bữa ăn thành công.
  - `LockoutApplied`: Khóa thực phẩm thành công.
- **Invariants (Quy tắc bất biến tại ranh giới)**:
  - Thêm món ăn mới vào `MealLog` → tự động cập nhật `LockoutRegistry` để khóa nguyên liệu Protein chính trong 7 ngày, Carb trong 5 ngày và Chủ đề món trong 3 ngày (BR-NU-02).

---

### 1.5 Ngữ cảnh Catalog

#### Aggregate Root: `Exercise`
- **Nhiệm vụ**: Thư viện bài tập chuẩn (chỉ chứa thông tin nghiệp vụ và nội dung).
- **Entities**: Không có.
- **Value Objects**:
  - `ExerciseInfo`: Tên bài, nhóm cơ chính/phụ, video hướng dẫn URL, dụng cụ, bài thay thế.
- **Repository**: `ExerciseRepository` (CRUD danh mục bài tập chuẩn).
- **Domain Events**:
  - `ExerciseCreated`: Bài tập mới được thêm vào thư viện.
- **Chia sẻ (Ubiquitous Language)**:
  - `Exercise` (Shared Aggregate) ──► Tiêu thụ bởi `Coaching`, `Workout`, `MotionSpecification` (qua ID).

#### Aggregate Root: `FoodItem`
- **Nhiệm vụ**: Thư viện thực phẩm và nguyên liệu chuẩn.
- **Entities**: Không có.
- **Value Objects**:
  - `FoodNutrient`: Tên, calo, macro trên 100g, nhãn chay/Halal, nhãn dị ứng.
- **Repository**: `FoodItemRepository` (CRUD danh mục thực phẩm chuẩn).
- **Domain Events**:
  - `FoodItemCreated`: Thực phẩm mới được thêm vào thư viện.
- **Chia sẻ (Ubiquitous Language)**:
  - `FoodItem` (Shared Aggregate) ──► Tiêu thụ bởi `Nutrition` (qua ID).
