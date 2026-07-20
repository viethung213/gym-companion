# ADR 03: Thiết Kế Contract Cho Coaching Service

* **Trạng thái**: Accepted
* **Tác giả**: Codex, Developer & Lead Architect

---

## 1. Bối cảnh & Quyết định
Tách biệt hoàn toàn hai cấu phần để đảm bảo ranh giới Bounded Context (DDD):
- **Coaching Service (`internal/coaching`)**: Lập lộ trình, quản lý lịch tuần, sinh giáo án ngày JIT, và xử lý adaptive check-in.
- **Workout Execution Service (`internal/workout_execution`)**: Điều phối và ghi nhận log buổi tập thực tế.

---

## 2. API & Commands Hệ Thống

### API Public (Client gọi qua gRPC/HTTP)
- `InitiateRoadmap`: Khởi tạo lộ trình 4 tuần ban đầu.
- `SubmitPreWorkoutCheckIn`: Gửi câu trả lời check-in động (dùng `body: "*"`).
- `GetActiveRoadmap`: Lấy nhanh Roadmap đang chạy kèm lịch tuần hiện tại (Endpoint `/active`).
- `GetPreWorkoutCheckIn`: Lấy câu hỏi check-in động đã sinh sẵn.
- `GetDailyWorkoutPlan`: Lấy giáo án tập luyện ngày.
- `ListWorkoutRoadmaps` & `GetWorkoutRoadmap`: Xem lại lịch sử lộ trình cũ.

### Commands Nội Bộ (Backend xử lý ngầm qua Event/Worker)
- `GenerateNextWeeklySchedule`: Tự động sinh lịch tuần kế tiếp khi nhận event kết thúc buổi tập cuối tuần.
- `GenerateDailyWorkoutPlanDraft`: Chạy background job sinh trước giáo án nháp (pre-caching) cho ngày hôm sau.
- `RegenerateDailyWorkoutPlan`: Sinh lại giáo án JIT khi check-in báo chấn thương mới hoặc thiếu thiết bị.

---

## 3. Các Nguyên Tắc Thiết Kế Cốt Lõi
- **Kiểm soát thông số an toàn**: Backend Go chịu trách nhiệm tính toán tải trọng (sets, reps, weight) bằng thuật toán/Rule Engine. AI Agent chỉ đóng vai trò hỗ trợ chọn bài tập theo ngữ cảnh an toàn từ DB.
- **Không dùng API CRUD chung chung**: Các thay đổi dữ liệu phải thông qua các Command nghiệp vụ rõ nghĩa, không dùng API generic `PUT/DELETE`.
- **Event payload tối thiểu**: Event tuân theo CloudEvents. Payload chỉ chứa dữ liệu nghiệp vụ tối thiểu để tránh dư thừa (routing và user ownership nằm ở envelope).
- **Quy ước thời gian**: Dùng `google.type.Date` cho ngày nghiệp vụ (start_date, scheduled_date) và `google.protobuf.Timestamp` cho thời điểm hệ thống ghi nhận (generated_at).
