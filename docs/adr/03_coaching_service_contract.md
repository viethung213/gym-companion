# ADR 03: Thiết Kế Contract Cho Coaching Service

## API & Commands Hệ Thống

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
