# Coaching — Expected Inbound Events

Coaching Context is decoupled at runtime from the other bounded contexts (D2).
Adapters that implement the reader ports may be **stub** during phase-1. This
file documents the minimum event shape Coaching expects from each producer so
the concrete Kafka consumers can be wired later without changing the domain or
application layer.

All events are wrapped in CloudEvents 1.0 envelopes. Coaching parses the
envelope, uses `id` for idempotency (via `coaching.outbox_log`), and dispatches
based on `type`.

---

## Workout Execution → Coaching

### `contracts.core.workout_execution.v1.event.WorkoutSessionCompleted`

Handler: `command.CompleteSessionHandler`
Effect: Marks the SessionPlan referenced by `plan_id` as COMPLETED, computes
SCR and ΔRPE, emits `SessionPlanExecuted`.

```json
{
  "session_id":          "string (workout execution session id)",
  "user_id":             "string",
  "completed_at":        "RFC3339 timestamp",
  "total_sets":          42,
  "total_volume":        1234.5,
  "average_form_score":  85.0,
  "average_rpe":         7.5,
  "plan_id":             "coaching SessionPlan id"
}
```

Notes:
- `plan_id` MUST equal a coaching `session_plan_id`. Coaching drops events with empty `plan_id`.
- If `plan_id` refers to an already-final session, the handler is idempotent (no-op).
- D7: `average_rpe` is a scalar; ΔRPE = `average_rpe − week.target_rpe`.

### `contracts.core.workout_execution.v1.event.WorkoutSessionAborted`

Handler: `command.AbortSessionHandler`
Effect: Marks the SessionPlan referenced by `plan_id` as SKIPPED.

```json
{
  "session_id":  "string",
  "user_id":     "string",
  "aborted_at":  "RFC3339 timestamp",
  "reason":      "timeout | user_cancelled | ...",
  "plan_id":     "coaching SessionPlan id"
}
```

Notes:
- Per commit 85b9647, aborted workouts map to SKIPPED (no separate ABORTED state).

### `contracts.core.workout_execution.v1.event.AdHocWorkoutStarted` (planned; tracked in #171)

Handler: `command.CreateAdHocSessionHandler` (dự kiến)
Effect: Khởi tạo & lưu mới `SessionPlan` vào PostgreSQL của Coaching với `status = PENDING` và `is_ad_hoc = true`.

```json
{
  "session_id": "string (workout execution session id)",
  "user_id":    "string",
  "plan_id":    "string (coaching SessionPlan id do WE tự sinh)",
  "started_at": "RFC3339 timestamp"
}
```

Notes:
- `plan_id` do Workout Execution sinh ra khi user bấm "Bắt đầu tập" ngoài lịch ➔ Coaching adopt làm `session_plan_id`.
- Trạng thái ban đầu của `SessionPlan` trong DB Coaching là `PENDING`.

### `contracts.core.workout_execution.v1.event.AdHocWorkoutCompleted` (planned; tracked in #171)

Handler: `command.CompleteAdHocSessionHandler` (dự kiến)
Effect: Cập nhật trạng thái `SessionPlan` từ `PENDING` $\rightarrow$ `COMPLETED` trong PostgreSQL của Coaching.

```json
{
  "session_id":          "string",
  "user_id":             "string",
  "completed_at":        "RFC3339 timestamp",
  "total_sets":          42,
  "total_volume":        1234.5,
  "average_rpe":         7.5,
  "plan_id":             "coaching SessionPlan id"
}
```

Notes:
- Cập nhật `SessionPlan.status = COMPLETED` trong DB Coaching.
- Tính toán $SCR$ và $\Delta RPE$ dựa trên số set thực tế và số set kê trong `prescription` người dùng đã chọn khi bấm tập, lưu vào `session_scr` và `session_delta_rpe`.
- KHÔNG phát event `RoadmapAdjusted` và KHÔNG áp dụng BR-AC-01.

---

## Profile → Coaching (planned; consumer not yet wired)

The following contracts are not yet consumed at runtime but the ports and
handlers exist. When the Profile bounded context emits them, wire a Kafka
consumer that dispatches to the corresponding coaching handler.

### `ProfileCompleted`

Handler: `command.InitiateRoadmapHandler`
Effect: Kick off UC-02.1 for the user.

Minimum shape needed:
```json
{ "user_id": "string" }
```

### `ProfileUpdated`

Handler: `command.RegenerateScheduleHandler` with `reason=profile_updated`.
Effect: Regenerate PENDING sessions from today onward.

```json
{ "user_id": "string" }
```

### `InjuryReported`

Handler: `command.RegenerateScheduleHandler` with `reason=injury_reported`.
Effect: Regenerate PENDING sessions; exclude affected muscle group.

```json
{ "user_id": "string", "muscle_group": "string" }
```

### `InjuryRecovered`

Handler: `command.RegenerateScheduleHandler` with `reason=injury_recovered`.
Effect: Post-injury protective window (BR-AC-09).

```json
{ "user_id": "string", "muscle_group": "string" }
```

---

## Read-only flow — no event emitted

### Flow 5: Ad-hoc session suggestion

Handler: `query.SuggestAdHocSessionHandler`
Endpoint: (not yet exposed via gRPC — internal query for now)

Request: `SuggestAdHocSessionQuery{ UserID, Hint { FreeText, MuscleGroups, AvailableEquipment, DurationMinutes, IntensityHint } }`

Response: `port.SuggestedSession { MuscleGroups, Prescription, Reasoning, EstimatedRPE }`

Behaviour:
- Read-only. **No DB write, no outbox event, no transaction.**
- Loads active roadmap only to seed phase context (Accumulation default if absent).
- Injury awareness: `InjuryStatus` from profile filters exercises.
- Duration cap: shrinks main-exercise set counts to fit budget.
- Intensity hint (`light` / `normal` / `hard`) tunes weight scaling + target RPE.

Rationale: user's flow "bấm bắt đầu tập ngoài lịch → 2 lựa chọn":
1. **Gọi agent gợi ý** → gọi handler này, agent trả về draft, frontend hiển thị.
2. **Pick tay từ catalog** → frontend gọi thẳng Exercise service, không đụng coaching.

Cả 2 nhánh sau khi user chốt sẽ bàn giao cho Workout Execution để tập thật. Nếu user muốn thêm buổi này vào roadmap chính thức, đó là 1 quyết định riêng (chưa impl — sẽ là RegenerateSchedule hoặc AddAdHocSession command).

---

## Coaching → Downstream (produced events)

Coaching writes these CloudEvents to `coaching.outbox`. The `OutboxWorker`
publishes them to Kafka.

| Type | Trigger | Payload fields |
|---|---|---|
| `contracts.core.coaching.v1.event.RoadmapInitiated` | UC-02.1 success | `roadmap_id`, `user_id`, `initiated_at` |
| `contracts.core.coaching.v1.event.RoadmapAdjusted` | UC-02.3, Trigger A, Signal B*, injury handling, WorkoutSessionAborted | `roadmap_id`, `user_id`, `reason`, `adjusted_at` |
| `contracts.core.coaching.v1.event.SessionPlanExecuted` | WorkoutSessionCompleted → CompleteSession | `session_plan_id`, `roadmap_id`, `user_id`, `executed_at`, `session_scr`, `session_delta_rpe` |
