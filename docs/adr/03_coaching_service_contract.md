# ADR 03: Ý Định Thiết Kế Contract Cho Coaching Service

* **Trạng thái**: Accepted
* **Tác giả**: Codex, Developer & Lead Architect

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
- `CoachingService`: lập lộ trình, quản lý lịch tuần, sinh giáo án ngày JIT, và xử lý adaptive check-in.

Cách tách này giữ domain, dữ liệu và contract của hai nghiệp vụ độc lập.

---

## 3. Danh Sách Command Thực Tế

| Command | Ý nghĩa |
|---|---|
| `InitiateRoadmap` | Khởi tạo roadmap và lịch tuần đầu tiên dựa trên thông tin khảo sát. |
| `SubmitPreWorkoutCheckIn` | Gửi câu trả lời khảo sát check-in động trước buổi tập (báo mệt, đau khớp, thiếu dụng cụ). |

Các hành động điều chỉnh khác như dời ngày tập, tạm dừng, deload... được thực thi trực tiếp qua các câu lệnh nghiệp vụ nội bộ ở backend hoặc do AI Agent tự động gọi Tool cập nhật DB, tránh phơi ra các API CRUD tĩnh phức tạp.

---

## 4. Danh Sách Query Thực Tế

| Query | Ý nghĩa |
|---|---|
| `ListWorkoutRoadmaps` | Lấy danh sách lịch sử các roadmap cũ (đã COMPLETED hoặc CANCELLED). |
| `GetWorkoutRoadmap` | Lấy chi tiết một roadmap cũ theo ID. |
| `GetActiveRoadmap` | Lấy nhanh duy nhất 1 roadmap đang hoạt động (ACTIVE) kèm theo lịch tuần hiện tại. |
| `GetPreWorkoutCheckIn` | Lấy bộ câu hỏi check-in động (trắc nghiệm + tự điền) được AI sinh trước cho hôm nay. |
| `GetDailyWorkoutPlan` | Lấy giáo án tập luyện của ngày hôm nay. |

*Lưu ý về endpoint `/active`*: Endpoint `GET /api/v1/users/{user_id}/workout-roadmaps/active` được chấp thuận như một ngoại lệ thiết kế hợp lý nhằm tối ưu hóa trải nghiệm (UX-driven API design), giúp Client lấy nhanh dữ liệu trang chủ chỉ với 1 lượt gọi.

---

## 5. Cơ Chế Thích Ứng Động (Dynamic Adaptive Check-In)

Quy tắc thích ứng (BR-AC-04 đến BR-AC-08) được vận hành động thông qua 2 cơ chế tương tác:

1. **Khảo sát động trước buổi tập (Pre-generated Check-In)**:
   - Cuối mỗi buổi tập trước, hệ thống chạy background job gọi AI Agent sinh sẵn bộ câu hỏi check-in động cá nhân hóa cho buổi tiếp theo (ví dụ hỏi thăm chấn thương cũ).
   - Khi mở app, Client lấy bộ câu hỏi từ DB lên hiển thị lập tức (0ms latency).
   - Người dùng trả lời, hệ thống cập nhật DB và tự động kích hoạt AI sửa lại giáo án ngày (JIT) nếu phát hiện chấn thương mới hoặc thiếu thiết bị.
2. **Tương tác qua Chat tự do với AI Coach**:
   - AI Coach tự động phát hiện các sự kiện bất thường (ví dụ: bùng tập 3 buổi liên tiếp) và bắt đầu hội thoại hỏi han.
   - Người dùng chat trả lời, AI Agent tự trích xuất intent và gọi Tool Backend (`UpdateWorkoutContext`) để trực tiếp thay đổi lịch tập hoặc cơ cấu lại bài tập dưới database.

---

## 6. Vì Sao Không Dùng `Update` / `Delete` Chung Chung

Các thao tác thay đổi phải là command nghiệp vụ rõ nghĩa để đảm bảo tính audit lịch sử và giữ vững ranh giới nghiệp vụ, thay vì cho phép client tự sửa dữ liệu DB trực tiếp bằng API generic `PUT/DELETE`.

---

## 7. Vì Sao Event Payload Phải Tối Thiểu

Event trong hệ thống tuân theo CloudEvents. Phần `data` chỉ chứa dữ liệu nghiệp vụ tối thiểu để tránh dư thừa. Thông tin định tuyến (ví dụ: `user_id`) nằm ở envelope và được dùng làm Kafka partition key.

---

## 8. Quy Ước Thời Gian

- `google.type.Date`: ngày nghiệp vụ (không kèm giờ), ví dụ: `start_date`, `end_date`, `scheduled_date`.
- `google.protobuf.Timestamp`: thời điểm chính xác hệ thống ghi nhận, ví dụ: `generated_at`, `initiated_at`.

---

## 9. Hệ Quả Triển Khai

- Dùng command/query rõ ràng, không viết manual route.
- Backend Go là nơi chịu trách nhiệm tính toán số liệu tải trọng an toàn (set, rep, tạ, volume) theo công thức Epley/Rule Engine.
- AI Agent đóng vai trò Reasoning Layer: chỉ hỗ trợ chọn bài tập, sắp xếp bài tập và giải thích điều chỉnh dựa trên ngữ cảnh an toàn do Backend cung cấp.

---

## 10. Kết Luận

Tách biệt rõ ràng **Coaching** (Lập kế hoạch) và **Execution** (Thực thi) giúp hệ thống giữ vững Bounded Context, tối giản cấu trúc API gRPC/REST, và đảm bảo an toàn tuyệt đối cho người tập nhờ cơ chế kiểm soát kép giữa Backend Go và AI Agent.
