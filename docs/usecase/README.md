# Use Case Specifications

Thư mục này chứa đặc tả Use Case chi tiết cho từng phân hệ (Bounded Context) của hệ thống FITAI:

| File đặc tả | Phân hệ (Context) | Các Use Case cụ thể |
| :--- | :--- | :--- |
| [`01_onboarding.md`](01_onboarding.md) | Onboarding | - UC-01.1 RegisterUser<br>- UC-01.2 CompleteHealthProfile<br>- UC-01.3 ReportInjury |
| [`02_coaching_planning.md`](02_coaching_planning.md) | Coaching & Planning | - UC-02.1 InitiateRoadmap<br>- UC-02.2 ExecuteSessionPlan<br>- UC-02.3 RegenerateScheduleOnProfileUpdated |
| [`03_workout_execution.md`](03_workout_execution.md) | Workout Execution | - UC-03.1 StartWorkoutSession<br>- UC-03.2 LogSet — AI Camera<br>- UC-03.3 LogSet — Phi AI<br>- UC-03.4 CompleteWorkoutSession<br>- UC-03.5 RecordPersonalRecord |
| [`04_adaptive_review_cycle.md`](04_adaptive_review_cycle.md) | Adaptive Review Cycle | - UC-04.1 EvaluateWeeklyAdaptiveCycle<br>- UC-04.2 DetectSignalB1 — Bỏ tập cấp tính / Mất tích<br>- UC-04.3 DetectSignalB2 — Kẹt lịch cố định<br>- UC-04.4 DetectSignalB3 — Tập ngoài lịch<br>- UC-04.5 DetectSignalB4 — Cơ bắp bị chai lỳ (Plateau) |
| [`05_nutrition.md`](05_nutrition.md) | Nutrition | - UC-05.1 GenerateDailyNutritionPlan<br>- UC-05.2 LogMeal |
