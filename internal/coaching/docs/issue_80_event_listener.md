# Issue #80: Lắng nghe UserProfileCompleted → Sinh lộ trình 4 tuần & lịch tuần đầu

## Data Flow

```mermaid
sequenceDiagram
    participant Profile as Profile Module
    participant Kafka as Kafka (profile.events)
    participant Consumer as Consumer (infra/kafka)
    participant Handler as ProfileCompletedHandler (app/event)
    participant Command as InitiateRoadmapHandler (app/command)
    participant ExSvc as ExerciseService (gRPC)
    participant AI as WorkoutPlanner (Mock → Gemini)
    participant Domain as Domain Layer
    participant DB as PostgreSQL (coaching schema)

    Profile->>Kafka: Publish ProfileCompleted
    Kafka->>Consumer: Poll message
    Consumer->>Consumer: Check outbox_log (idempotent)
    Consumer->>Handler: Dispatch ProfileCompletedEvent
    Handler->>Command: Dispatch InitiateRoadmapCommand
    Command->>Command: Check roadmap ACTIVE exists?
    Command->>ExSvc: SearchExercises(avoid_injury_areas)
    ExSvc-->>Command: Danh sách bài tập hợp lệ
    Command->>AI: PlanWorkout(exercises, goals)
    AI-->>Command: Exercise arrangement (mock)
    Command->>Domain: Roadmap.Initiate() + Schedule.Generate() + Plan.Create()
    Domain-->>Command: Aggregates created
    Command->>DB: TX: Save roadmaps + schedules + plans + outbox events
```

## Notes

- **Idempotency**: Consumer check `outbox_log.event_id` trước khi dispatch. Handler check roadmap ACTIVE đã tồn tại.
- **Mock Planner**: Implement interface `WorkoutPlanner`. Swap Gemini = thay constructor, không sửa logic.
- **Injury filtering**: Profile giữ dữ liệu chấn thương. Coaching chỉ truyền `registered_injuries` → `avoid_injury_areas` khi gọi ExerciseService. Không lưu lại.
- **Schema isolation**: Không JOIN chéo. Gọi gRPC sang Exercise module để lấy bài tập.
- **3 Aggregate Root riêng biệt**: WorkoutRoadmap (chiến lược chu kỳ), WeeklySchedule (phân bổ tải/nhóm cơ), DailyWorkoutPlan (prescription JIT).
