# FITAI — Đặc tả Thiết kế Chiến thuật DDD (Tactical Design)

> Nguồn: [Đặc tả Yêu cầu Nghiệp vụ Cốt lõi BABOK](./NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](./02_bounded_context.md)

---

## 1. Ngôn ngữ Thống nhất (Ubiquitous Language)

Để toàn bộ team phát triển (Web, App, Backend, AI) thống nhất tên gọi trong code, API và Database:

| Thuật ngữ | Tên trong Code | Ý nghĩa nghiệp vụ | Context sở hữu |
|---|---|---|---|
| Lộ trình 4 tuần | `WorkoutRoadmap` | Kế hoạch tập tổng quan trong 4 tuần | Coaching |
| Lịch tập tuần | `WeeklySchedule` | Phân bổ nhóm cơ tập theo từng ngày trong tuần | Coaching |
| Giáo án theo buổi | `DailyWorkoutPlan` | Bài tập, set, rep, tạ chi tiết sinh ra ngay trước buổi tập | Coaching |
| Buổi tập thực tế | `WorkoutSession` | Trạng thái buổi tập đang diễn ra (Timer, log thực tế) | Workout & Motion |
| Nhật ký Set | `SetLog` | Số rep thực tế, mức tạ, ROM%, Form Score, RPE của 1 set | Workout & Motion |
| Chỉ số cơ thể | `BodyMetric` | Cân nặng, % mỡ, số đo các vòng, ảnh tiến trình | User Profile |
| Lịch sử chỉ số | `BodyMetricTimeline`| Dòng lịch sử thay đổi cân nặng/mỡ cơ thể của user | User Profile |
| Kỷ lục cá nhân | `PersonalRecord` (PR) | Mức tạ 1RM ước tính cao nhất đạt được cho từng bài tập | Workout & Motion |
| Chỉ số gắng sức | `RPE` | Thang điểm từ 1-10 đánh giá độ mệt sau set/buổi tập | Workout & Motion |
| Cấu hình AI bài tập | `MotionSpecification`| Pose mẫu, quy tắc đếm rep, thang điểm Form chuẩn | Workout & Motion |
| Tỷ lệ hoàn thành | `CompletionRate` (CR) | Tỷ lệ số buổi tập thực tế hoàn thành trên tổng số buổi lên lịch | Coaching |
| Chống lặp món | `AntiRepetitionLock` | Khóa thực phẩm đã ăn không cho gợi ý lại | Nutrition |

---

## 2. Đặc tả 5 Bounded Contexts Nghiệp vụ

---

### 2.1 User Profile Context

#### [Aggregate Root] User
- **Vai trò**: Đại diện cho người tập và thông tin tài khoản của họ.
- **Entities**:
  - `Profile`: Thông tin định danh của người dùng (tên, tuổi, giới tính, mục tiêu, slot nhắc lịch).
  - `InjuryRecord`: Vùng chấn thương, bệnh lý mãn tính, trạng thái phục hồi.
- **Value Objects**:
  - `Age`: Tự động validate giá trị sinh học (> 0).
  - `TrainingScheduleSlot`: Khung giờ tập cố định bắt buộc chọn [FR-UM-03].
  - `ChatbotContext`: Thiết bị có sẵn, dị ứng thức ăn thu thập dần [Assumption-03].
- **Domain Events**:
  - `UserProfileCompleted`: Phát ra khi hồ sơ hoàn thiện ≥ 80% (Kích hoạt tạo lộ trình).
  - `InjuryReported`: Phát ra khi khai báo chấn thương mới (Kích hoạt đổi giáo án).

#### [Aggregate Root] HealthProfile
- **Vai trò**: Quản lý lịch sử và trạng thái chỉ số cơ thể hiện tại (Source of Truth).
- **Entities**:
  - `BodyMetricTimeline`: Lưu lịch sử biến động cân nặng, mỡ cơ thể, số đo các vòng.
- **Value Objects**:
  - `Weight`: Tự validate (giá trị > 0, đơn vị kg/lbs).
  - `Height`: Tự validate (giá trị > 0, đơn vị cm/inch).
  - `BodyFat`: Tự validate (% mỡ cơ thể trong khoảng [0, 100]).
  - `ProgressImage`: URL ảnh tiến trình lưu trên Cloud.
- **Domain Events**:
  - `BodyMetricLogged`: Đã cập nhật chỉ số cơ thể mới.
- **Physical Storage (PostgreSQL / MinIO)**:
  - Table `users` (id, email, phone, status).
  - Table `user_profiles` (user_id, age, gender, height, weight, goal).
  - Table `user_injuries` (user_id, body_part, description, status).
  - Table `body_metrics` (id, user_id, recorded_at, weight, fat_percent, arm_size, waist_size) - Lưu trữ dòng lịch sử cân nặng.
  - Image files (Ảnh tiến trình) -> Lưu vào MinIO/S3 Path: `/users/{userId}/progress/images/{date}.jpg`.

---

### 2.2 Coaching & Planning Context

#### [Aggregate Root 1] WorkoutRoadmap
- **Vai trò**: Lộ trình lớn 4 tuần của người dùng.
- **Value Objects**:
  - `RoadmapPhase`: Giai đoạn tăng tiến Progressive Overload.
  - `CompletionRate`: Tỷ lệ hoàn thành cuối chu kỳ (dùng cho Trigger A).
- **Domain Events**:
  - `RoadmapInitiated`: Khởi tạo lộ trình 4 tuần đầu tiên.
  - `RoadmapAdjusted`: Điều chỉnh lộ trình mới (Trigger A/B).

#### [Aggregate Root 2] WeeklySchedule
- **Vai trò**: Lịch tập phân bổ theo tuần.
- **Value Objects**:
  - `MuscleSplit`: Phân bổ nhóm cơ theo ngày (Ngực/Vai/Tay...).
  - `WorkoutPlanIds`: Danh sách ID của các `DailyWorkoutPlan` trong tuần (Tách Aggregate bằng ID-based Reference).
- **Domain Events**:
  - `WeeklyScheduleGenerated`: Tạo lịch tuần mới.
- **Physical Storage (PostgreSQL)**:
  - Table `workout_roadmaps` (id, user_id, start_date, status, completion_rate).
  - Table `weekly_schedules` (id, roadmap_id, week_number, muscle_split).

#### [Aggregate Root 3] DailyWorkoutPlan
- **Vai trò**: Giáo án chi tiết một ngày tập, được tạo lập cô lập để tối ưu hóa ranh giới transaction và tránh lock dữ liệu khi user tập.
- **Value Objects**:
  - `WorkoutPrescription`: Bài tập, set, rep, mức tạ gợi ý cho từng bài, bao gồm cả warm-up/cool-down.
- **Domain Events**:
  - `DailyWorkoutPlanGenerated`: Giáo án chi tiết ngày đã được sinh (JIT).
- **Physical Storage (PostgreSQL)**:
  - Table `daily_workout_plans` (id, schedule_id, date, status, prescription_json).

#### [Domain Service] OverloadValidator
- **Vai trò**: Kiểm tra quy tắc tăng volume Progressive Overload không vượt quá 10% của tuần trước (**BR-AC-02**).
- **Hoạt động**: Nhận dữ liệu tổng volume thực tế tuần trước từ `Workout Execution Context` và so sánh với tổng volume dự kiến của `WeeklySchedule` tuần mới trước khi phát hành.

---

### 2.3 Workout Execution & Motion Context

#### [Aggregate Root 1] WorkoutSession
- **Vai trò**: Buổi tập thực tế đang diễn ra của người dùng.
- **Entities**:
  - `SetLog`: Nhật ký thô của từng set tập (rep, weight, ROM%, FormScore, RPE).
- **Value Objects**:
  - `SessionTimer`: Cấu hình đếm ngược và giới hạn thời gian (90/180/240 phút) [BR-WL-01].
  - `PlaylistConfig`: Playlist âm nhạc của buổi tập [FR-WL-02].
- **Domain Events**:
  - `WorkoutSessionStarted`: User check-in bắt đầu tập.
  - `WorkoutSessionCompleted`: Kết thúc buổi tập (Chứa tổng volume, Form trung bình, calo tiêu hao).
  - `WorkoutSessionAborted`: Buổi tập tự động đóng do quá thời gian [BR-WL-01].
  - `BodyMetricUpdated`: User log cân nặng mới trong buổi tập (Phát Event để User Profile Context hứng và lưu).
- **Physical Storage (PostgreSQL)**:
  - Table `workout_sessions` (id, user_id, date, duration, total_volume, status).
  - Table `set_logs` (id, session_id, exercise_id, set_number, weight, reps, rom_percent, form_score, rpe).

#### [Aggregate Root 2] WorkoutPerformance
- **Vai trò**: Lưu trữ kỷ lục cá nhân phục vụ CQRS Read Model.
- **Entities**:
  - `PersonalRecord`: Kỷ lục 1RM ước tính theo Epley Formula cho từng bài tập.
- **Domain Events**:
  - `NewPersonalRecordAchieved`: Đạt kỷ lục 1RM mới (Vinh danh PR).
- **Hoạt động Consistency**: Cập nhật PR thông qua **Eventual Consistency** bằng cách lắng nghe sự kiện `WorkoutSessionCompleted`.
- **Physical Storage (PostgreSQL)**:
  - Table `personal_records` (user_id, exercise_id, one_rm, achieved_at).

#### [Aggregate Root 3] MotionSpecification
- **Vai trò**: Cấu hình AI và Pose mẫu chuẩn cho từng bài tập (được package `motion` tiêu thụ và phân phối cho Client).
- **Value Objects**:
  - `PoseTemplate`: Tọa độ khớp chuẩn (33 điểm MediaPipe hoặc 17 điểm YOLO).
  - `CalibrationConfig`: Ngưỡng khoảng cách, góc nghiêng điện thoại.
  - `RepCountingRules`: Ngưỡng góc ROM% tối thiểu (≥ 70%) để đếm rep [BR-CC-01].
  - `FormScoringRules`: Tiêu chí phát hiện lỗi tư thế (võng lưng, gối chụm...).
- **Domain Events**:
  - `MotionSpecUpdated`: Cập nhật cấu hình AI (Client tự động tải về bản mới).
- **Physical Storage (PostgreSQL / Redis / MinIO)**:
  - Table `motion_specifications` (exercise_id, model_type, pose_template_json, rules_json, version) - Tham chiếu tới ID bài tập trong `Catalog`.
  - Cache PoseTemplate vào **Redis** để Client tải về tốc độ cao.
  - Raw Skeleton Data (Toạ độ khớp thô 30FPS gửi lên) -> Lưu trực tiếp vào **MinIO/S3** dạng JSON gzip: `/raw-skeleton/{userId}/{sessionId}/{exerciseId}_{timestamp}.json.gz`.

---

### 2.4 Nutrition Context

#### [Aggregate Root 1] DailyNutritionPlan
- **Vai trò**: Thực đơn gợi ý và mục tiêu dinh dưỡng trong ngày.
- **Entities**:
  - `MealSuggestion`: 3 bữa chính + 1 bữa phụ [FR-NU-02].
- **Value Objects**:
  - `TargetCalories`: Calo mục tiêu ngày (Tự động validate không dưới 1,200 kcal để bảo vệ **BR-NU-01**).
  - `Macronutrients`: Phân chia Carb/Protein/Fat (Bảo vệ invariant: Calo = carb*4 + protein*4 + fat*9).
  - `BudgetTier`: Phân khúc giá (Tiết kiệm, Phổ thông, Thoải mái).
- **Domain Events**:
  - `DailyNutritionPlanGenerated`: Thực đơn ngày đã được sinh.

#### [Aggregate Root 2] NutritionJournal
- **Vai trò**: Nhật ký ăn uống và kiểm soát lặp món của user.
- **Entities**:
  - `MealLog`: Các món user khai báo đã ăn thực tế [FR-NU-04].
- **Value Objects**:
  - `LockoutRegistry`: Danh sách nguyên liệu bị khóa (Protein 7 ngày, tinh bột 5 ngày, chủ đề 3 ngày) [BR-NU-02].
- **Domain Events**:
  - `MealLogged`: Ghi nhận bữa ăn thành công.
  - `LockoutApplied`: Khóa thực phẩm thành công.
- **Physical Storage (PostgreSQL / Redis)**:
  - Table `daily_nutrition_plans` (id, user_id, date, target_calories, carb, protein, fat).
  - Table `meal_suggestions` (plan_id, meal_type, food_id, weight, budget_tier).
  - Table `meal_logs` (id, user_id, logged_at, food_id, quantity).
  - Khóa lặp món `LockoutRegistry` lưu vào **Redis** (Key: `lockout:{userId}:{ingredientId}`, TTL tương ứng 7 ngày / 5 ngày).

---

### 2.5 Catalog Context

#### [Aggregate Root 1] ExerciseCatalog
- **Vai trò**: Thư viện bài tập chuẩn (chỉ chứa thông tin nghiệp vụ và nội dung).
- **Entities**:
  - `Exercise`: Tên bài, nhóm cơ chính/phụ, video hướng dẫn URL, dụng cụ, bài thay thế [FR-SM-01].
- **Domain Events**:
  - `ExerciseCreated`: Bài tập mới được thêm vào thư viện.

#### [Aggregate Root 2] FoodCatalog
- **Vai trò**: Thư viện thực phẩm và nguyên liệu chuẩn.
- **Entities**:
  - `FoodItem`: Tên, calo, macro trên 100g, nhãn chay/Halal, nhãn dị ứng [FR-SM-02].
- **Domain Events**:
  - `FoodItemCreated`: Thực phẩm mới được thêm vào thư viện.
- **Physical Storage (PostgreSQL)**:
  - Table `exercises` (id, name, main_muscle, equipment, video_url, status).
  - Table `food_items` (id, name, calories, carb, protein, fat, flags_json).
