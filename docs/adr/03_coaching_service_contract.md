# ADR 03: Thiết Kế Contract Cho Coaching Service

* **Trạng thái**: Accepted
* **Tác giả**: Codex, Developer & Lead Architect

---

## 1. Bối cảnh & Quyết định
Tách biệt hoàn toàn hai cấu phần để đảm bảo ranh giới Bounded Context (DDD):
- **Coaching Service (`internal/coaching`)**: Lập lộ trình, quản lý lịch tuần, sinh giáo án ngày JIT, và xử lý adaptive check-in.
- **Workout Execution Service (`internal/workout_execution`)**: Điều phối và ghi nhận log buổi tập thực tế.

---

## 2. API Contract Thực Tế

### Commands
- `InitiateRoadmap`: Khởi tạo lộ trình 4 tuần ban đầu.
- `SubmitPreWorkoutCheckIn`: Gửi câu trả lời check-in động (đóng gói trực tiếp trong request body, dùng `body: "*"`).

### Queries
- `GetActiveRoadmap`: Lấy nhanh Roadmap đang chạy kèm lịch tuần hiện tại (Endpoint `/active` được chấp thuận để tối ưu UX).
- `GetPreWorkoutCheckIn`: Lấy bộ câu hỏi check-in động được AI sinh trước từ buổi tập cũ (giảm thiểu latency).
- `GetDailyWorkoutPlan`: Lấy giáo án tập luyện ngày (đang để Unary, tương lai nâng cấp lên Server Stream theo ADR-01).
- `ListWorkoutRoadmaps` & `GetWorkoutRoadmap`: Phục vụ xem lại lịch sử lộ trình cũ.

---

## 3. Các Nguyên Tắc Thiết Kế Cốt Lõi
- **Kiểm soát thông số an toàn**: Backend Go chịu trách nhiệm tính toán tải trọng (sets, reps, weight) bằng thuật toán/Rule Engine. AI Agent chỉ đóng vai trò hỗ trợ chọn bài tập theo ngữ cảnh an toàn từ DB.
- **Không dùng API CRUD chung chung**: Các thay đổi dữ liệu phải thông qua các Command nghiệp vụ rõ nghĩa, không dùng API generic `PUT/DELETE`.
- **Event payload tối thiểu**: Event tuân theo CloudEvents. Payload chỉ chứa dữ liệu nghiệp vụ tối thiểu để tránh dư thừa (routing và user ownership nằm ở envelope).
- **Quy ước thời gian**: Dùng `google.type.Date` cho ngày nghiệp vụ (start_date, scheduled_date) và `google.protobuf.Timestamp` cho thời điểm hệ thống ghi nhận (generated_at).
