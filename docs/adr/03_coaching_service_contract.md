# ADR 03: Ý Định Thiết Kế Contract Cho Coaching Service

* **Trạng thái**: Proposed
* **Tác giả**: Codex & Developer

---

## 1. Bối cảnh

Context **Coaching & Planning** chịu trách nhiệm trả lời câu hỏi: *"Tôi nên tập gì? Khi nào điều chỉnh?"*.
Theo thiết kế DDD hiện tại, context này quản lý ba Aggregate chính:

- `WorkoutRoadmap`: lộ trình tập luyện 4 tuần.
- `WeeklySchedule`: lịch tập/nghỉ theo từng tuần.
- `DailyWorkoutPlan`: giáo án chi tiết sinh theo ngày, theo cơ chế Just-In-Time.

Trong khi đó, `WorkoutService` hiện tại đang đại diện cho phần **Workout Execution**: bắt đầu buổi tập, log set, hoàn thành buổi tập.
Nếu tiếp tục nhét cả lập kế hoạch, sinh lịch, sinh giáo án và adaptive review vào `WorkoutService`, contract sẽ bị trộn hai nghiệp vụ khác nhau:

- **Coaching**: quyết định nên tập gì, khi nào sinh lịch, khi nào điều chỉnh.
- **Workout Execution**: ghi nhận user đã tập như thế nào.

Vì `contracts/` là Single Source of Truth cho API, gRPC, REST gateway và OpenAPI, ranh giới này cần được thể hiện trực tiếp trong proto.

---

## 2. Ý Định Thiết Kế

Tách **Coaching & Planning** thành bounded context riêng:

- Contract: `proto/contracts/core/coaching/v1`.
- Go module: `internal/coaching`.
- PostgreSQL schema: `coaching`.

- `WorkoutExecutionService`: điều phối và ghi nhận buổi tập thực tế.
- `CoachingService`: lập lộ trình, sinh lịch tuần, sinh giáo án ngày, và xử lý adaptive review.

Cách tách này giữ domain, dữ liệu và contract của hai nghiệp vụ độc lập.

---

## 3. Danh Sách Command

| Command | Ý nghĩa |
|---|---|
| `InitiateRoadmap` | Nội bộ: tạo roadmap và lịch tuần đầu sau `ProfileCompleted`. |
| `GenerateNextWeeklySchedule` | Nội bộ: sinh lịch tuần kế tiếp từ kết quả tập thực tế. |
| `GenerateDailyWorkoutPlan` | Public: sinh giáo án JIT từ check-in của user. |
| `RegenerateDailyWorkoutPlan` | Public: thay plan chưa sử dụng khi ngữ cảnh thay đổi. |
| `RespondAdaptiveRecommendation` | Public: nhận lựa chọn của user cho đề xuất thích ứng. |
| `ResumeRoadmap` | Public: cho user tiếp tục roadmap đang tạm dừng. |
| `PauseRoadmap` | Nội bộ: tạm dừng roadmap theo adaptive recommendation. |
| `SkipScheduledDay` | Nội bộ: đánh dấu bỏ buổi, không tự dồn lịch. |
| `RescheduleScheduleDay` | Nội bộ: dời ngày tập sau khi user xác nhận. |
| `RunAdaptiveReview` | Nội bộ: đánh giá CR và các signal B1–B4. |
| `CompleteRoadmap` | Nội bộ: hoàn thành roadmap khi kết thúc chu kỳ. |

---

## 4. Danh Sách Query

| Query | Ý nghĩa |
|---|---|
| `ListWorkoutRoadmaps` | Lấy danh sách roadmap của user, có thể lọc theo trạng thái. |
| `GetWorkoutRoadmap` | Lấy chi tiết một roadmap. |
| `ListWeeklySchedules` | Lấy lịch tuần theo roadmap hoặc khoảng ngày. |
| `GetWeeklySchedule` | Lấy chi tiết một lịch tuần. |
| `GetDailyWorkoutPlan` | Lấy giáo án ngày để hiển thị hoặc bắt đầu buổi tập. |
| `ListAdaptiveRecommendations` | Lấy đề xuất thích ứng đang chờ hoặc lịch sử. |

Không dùng endpoint kiểu `/workout-roadmaps/active`.
`active` là trạng thái của resource, không phải resource.
Thiết kế phù hợp hơn là:

```http
GET /api/v1/users/{user_id}/workout-roadmaps?status=ROADMAP_STATUS_ACTIVE
GET /api/v1/users/{user_id}/workout-roadmaps/{roadmap_id}
```

---

## 5. Vì Sao Cần Adaptive Review

BR-AC-04 đến BR-AC-08 mô tả các rule điều chỉnh coaching sau khi có dữ liệu thực tế.
Nếu chỉ có API generate roadmap/schedule/plan, hệ thống chưa đủ khả năng phản ứng với hành vi thật của user.

Adaptive review cần tồn tại vì:

- user có thể bỏ tập nhiều ngày;
- user có thể bỏ cùng một ngày trong tuần lặp lại;
- user có thể tập quá tải;
- user có thể bị plateau;
- cuối chu kỳ cần tính completion rate để quyết định roadmap tiếp theo.

Tuy nhiên, không phải adaptive signal nào cũng được tự động áp dụng.
Một số signal cần tạo đề xuất và chờ user chọn.
Vì vậy cần model `AdaptiveRecommendation`.

Ví dụ:

- BR-AC-05: không hoạt động 7 ngày → đề xuất tiếp tục, reset tuần, hoặc pause roadmap.
- BR-AC-06: bỏ cùng ngày ≥ 3 lần → đề xuất dời lịch, chỉ áp dụng nếu user đồng ý.
- BR-AC-08: plateau → đề xuất deload, đổi bài, hoặc tăng set.

Ngược lại, BR-AC-07 về overtraining có tính an toàn cao hơn, nên hệ thống có thể bắt buộc chèn ngày nghỉ.

---

## 6. Vì Sao Không Dùng `Update` / `Delete` Chung Chung

`WorkoutRoadmap`, `WeeklySchedule`, và `DailyWorkoutPlan` là dữ liệu nghiệp vụ do hệ thống sinh ra.
Chúng cần audit được theo thời gian.

Không nên có API generic như:

```http
PUT /workout-roadmaps/{id}
DELETE /weekly-schedules/{id}
```

Các thao tác thay đổi phải là command nghiệp vụ rõ nghĩa:

- `RespondAdaptiveRecommendation`
- `RescheduleScheduleDay`
- `SkipScheduledDay`
- `RegenerateDailyWorkoutPlan`
- `PauseRoadmap`
- `ResumeRoadmap`

Cách này giúp contract thể hiện đúng lý do thay đổi trạng thái, thay vì chỉ cho phép client sửa dữ liệu trực tiếp.

---

## 7. Vì Sao Event Payload Phải Tối Thiểu

Event trong hệ thống phải đi theo CloudEvents.
Envelope chứa các thông tin kỹ thuật như:

- `id`
- `source`
- `type`
- `time`
- extension attributes, ví dụ `userid`

Phần `data` chỉ nên chứa dữ liệu nghiệp vụ tối thiểu cần cho consumer.

Vì vậy event mới của coaching không nên nhét `user_id` vào payload nếu user id đã nằm trong CloudEvents extension và được dùng làm Kafka partition key.

Ví dụ payload tốt:

```proto
message WeeklyScheduleGenerated {
  string weekly_schedule_id = 1;
  string roadmap_id = 2;
  int32 week_number = 3;
  google.type.Date start_date = 4;
  google.type.Date end_date = 5;
  google.protobuf.Timestamp generated_at = 6;
}
```

Payload này đủ để consumer biết lịch nào được sinh, thuộc roadmap nào, tuần mấy, có hiệu lực trong khoảng nào.
Thông tin routing/user ownership nằm ở envelope.

---

## 8. Quy Ước Thời Gian

Contract cần phân biệt rõ:

- `google.type.Date`: ngày nghiệp vụ, không kèm giờ.
- `google.protobuf.Timestamp`: thời điểm chính xác hệ thống ghi nhận.

Áp dụng:

- `start_date`, `end_date`, `scheduled_date` dùng `google.type.Date`.
- `generated_at`, `created_at`, `responded_at`, `paused_at`, `resumed_at` dùng `google.protobuf.Timestamp`.

Điều này tránh nhầm giữa "ngày dự kiến tập" và "thời điểm hệ thống sinh dữ liệu".

---

## 9. Hệ Quả Triển Khai

Khi triển khai proto cho `CoachingService`:

- dùng command/query rõ ràng;
- REST URL dùng danh từ số nhiều;
- command có input ngoài path dùng `body: "payload"`;
- query không có body;
- khai báo security bằng OpenAPI annotation trong proto;
- không viết manual route hoặc manual OpenAPI;
- chạy `buf lint`, `buf breaking`, và `buf generate`.

Backend Go phải là nơi tính toán số liệu an toàn:

- set;
- rep;
- tạ;
- volume;
- progressive overload;
- validation theo BR-AC-01 và BR-AC-02.

Agent chỉ hỗ trợ chọn bài tập hoặc thay thế bài tập theo ngữ cảnh.
Agent không được là nguồn quyết định cuối cùng cho các con số tải trọng.

---

## 10. Kết Luận

`CoachingService` cần tồn tại để tách nghiệp vụ lập kế hoạch khỏi nghiệp vụ thực thi buổi tập.
Các API planning tạo dữ liệu theo lifecycle của roadmap, weekly schedule và daily plan.
Các API adaptive xử lý phản ứng sau khi có dữ liệu tập thực tế.

Thiết kế này giúp contract:

- bám đúng DDD boundary;
- tránh endpoint mơ hồ như `/active`;
- tránh `Update/Delete` chung chung;
- giữ event payload tối thiểu;
- tuân thủ contract-first và CloudEvents;
- phù hợp với quyết định ADR 02: Backend Go tính toán an toàn, Agent chỉ hỗ trợ chọn bài.
