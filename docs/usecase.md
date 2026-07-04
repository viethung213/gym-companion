# FITAI — Use Case Specification (Application Layer)

> Nguồn: [BRD](./NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](./02_bounded_context.md) · [Tactical Design](./03_ddd_tactical_design.md)

Tài liệu đặc tả Use Case của hệ thống FITAI được chia nhỏ thành các phân hệ tương ứng để dễ dàng quản lý và theo dõi:

- [Xem hướng dẫn chung tại README](./usecase/README.md)

## Danh sách các phân hệ Use Case

### 1. [UC-01 Onboarding](./usecase/01_onboarding.md)
Đăng ký tài khoản, thiết lập hồ sơ sức khỏe và báo cáo chấn thương.
- UC-01.1 RegisterUser
- UC-01.2 CompleteHealthProfile
- UC-01.3 ReportInjury

### 2. [UC-02 Coaching & Planning](./usecase/02_coaching_planning.md)
Khởi tạo lộ trình tập luyện và tự động sinh giáo án hàng ngày (JIT).
- UC-02.1 InitiateWorkoutRoadmap
- UC-02.2 GenerateDailyWorkoutPlan (JIT)

### 3. [UC-03 Workout Execution](./usecase/03_workout_execution.md)
Thực thi buổi tập (phương án hỗ trợ qua AI Camera và phương án Phi AI), tự động ghi nhận PR (Personal Record).
- UC-03.1 StartWorkoutSession
- UC-03.2 LogSet — Nhánh AI Camera
- UC-03.3 LogSet — Nhánh Phi AI
- UC-03.4 CompleteWorkoutSession
- UC-03.5 RecordPersonalRecord

### 4. [UC-04 Adaptive Review Cycle](./usecase/04_adaptive_review_cycle.md)
Đánh giá chu kỳ tập luyện, phát hiện các tín hiệu bất thường (không hoạt động, lịch không tương thích, overtraining, plateau) và điều chỉnh lộ trình thích ứng.
- UC-04.1 EvaluateEndOfCycleCompletionRate
- UC-04.2 DetectSignalB1 — Không hoạt động
- UC-04.3 DetectSignalB2 — Lịch không tương thích
- UC-04.4 DetectSignalB3 — Overtraining
- UC-04.5 DetectSignalB4 — Plateau

### 5. [UC-05 Nutrition](./usecase/05_nutrition.md)
Khởi tạo kế hoạch dinh dưỡng hàng ngày và ghi nhận các bữa ăn.
- UC-05.1 GenerateDailyNutritionPlan
- UC-05.2 LogMeal
