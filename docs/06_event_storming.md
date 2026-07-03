# 6. Event Storming - FITAI

Tài liệu này tổng hợp kết quả phân tích **Event Storming** của hệ thống **FITAI**, định nghĩa các Sự kiện miền (Domain Events), Lệnh (Commands), Mô hình đọc (Read Models) và Chính sách (Policies) điều khiển toàn bộ luồng nghiệp vụ.

---

## 6.1 Tổng Quan Luồng Sự Kiện (Event Storming Flow Diagram)

```
[Command] ──► (Aggregate) ──► [Domain Event] ──► [Policy] ──► [Command]
                                    │
                                    ▼
                             [Read Model]
```

---

## 6.2 Chi Tiết Theo Từng Ngữ Cảnh Bounded Context

### 1. User Profile & Health Context

* **Commands (Lệnh)**:
  - `RegisterUser`: Đăng ký tài khoản người dùng mới.
  - `SubmitHealthProfile`: Nhập thông tin sinh học ban đầu (tuổi, giới tính, chiều cao, cân nặng, mục tiêu).
  - `ReportInjury`: Khai báo vùng chấn thương mới.
  - `LogBodyMetrics`: Ghi nhận thêm chỉ số cơ thể mới vào lịch sử để vẽ biểu đồ tiến trình.
  - `UpdateChatbotContext`: Cập nhật thông tin thiết bị tập luyện (`equipment_list`) và dị ứng thực phẩm (`food_restrictions`) thu thập qua chatbot.
* **Domain & Integration Events (Sự kiện)**:
  - `UserRegistered` [Integration]: Tài khoản người dùng được tạo thành công trên hệ thống.
  - `ProfileCompleted` [Integration]: Hồ sơ đạt mức độ hoàn thiện $\ge 80\%$ (kích hoạt dịch vụ AI Coach).
  - `InjuryReported` [Integration]: Phát hiện chấn thương mới được khai báo.
  - `BodyMetricsLogged`: Bản ghi chỉ số cơ thể mới được thêm vào lịch sử thành công.
  - `UserMetricsUpdated` [Integration]: Chỉ số cơ thể hiện tại của người dùng thay đổi (cân nặng/chiều cao).
  - `ChatbotContextUpdated`: Thông tin thiết bị hoặc dị ứng thực phẩm được cập nhật.
* **Read Models (Mô hình đọc)**:
  - `HealthProfileOverview`: Xem chi tiết chỉ số cơ thể hiện tại, danh sách chấn thương và trạng thái kích hoạt AI Coach.
  - `BodyMetricsHistoryView`: Xem biểu đồ diễn biến cân nặng và tỷ lệ mỡ theo dòng thời gian tập.
* **Policies (Chính sách)**:
  - *Policy: Khi nhận sự kiện `UserRegistered`* ──► Tự động phát lệnh khởi tạo hồ sơ sức khỏe trống.
  - *Policy: Khi nhận sự kiện `ProfileCompleted`* ──► Tự động phát lệnh khởi tạo AI Coach và tính toán TDEE dinh dưỡng lần đầu.
  - *Policy: Khi nhận sự kiện `BodyMetricsLogged`* ──► Phát lệnh đồng bộ chỉ số hiện tại của Aggregate `User` và phát sự kiện `UserMetricsUpdated` ra toàn hệ thống.

---

### 2. AI Coaching & Planning Context

* **Commands (Lệnh)**:
  - `ActivateAICoach`: Thiết lập phong cách cho huấn luyện viên ảo (Drill Sergeant, Best Friend, Data Analyst).
  - `InitializeCoaching`: Khởi tạo Lộ trình tổng quan 4 tuần và Lịch tập tuần đầu tiên (phân bổ cơ, không sinh bài chi tiết).
  - `GenerateJITWorkout`: Sinh giáo án chi tiết JIT hàng ngày (bài tập, set, rep, tạ gợi ý, warm-up/cool-down) khi user mở app hoặc chuẩn bị tập.
  - `EvaluateRoadmapCycle` (Trigger A): Tính toán Completion Rate (CR) cuối mỗi chu kỳ 4 tuần để đề xuất/điều chỉnh lộ trình tiếp theo.
  - `ProcessBehaviorSignal` (Trigger B): Liên tục phân tích 4 tín hiệu hành vi (Signals B1-B4) để điều chỉnh nhanh giáo án.
  - `SubstituteExercise`: Thay thế bài tập tập trung vào nhóm cơ bị chấn thương.
* **Domain & Integration Events (Sự kiện)**:
  - `AICoachActivated`: AI Coach được cấu hình thành công.
  - `CoachingInitialized`: Lộ trình 4 tuần đầu tiên đã sẵn sàng.
  - `JITWorkoutGenerated`: Giáo án chi tiết hôm nay đã được sinh ra JIT.
  - `RoadmapAdapted` (Trigger A): Lộ trình 4 tuần kế tiếp đã được thích ứng đổi mới.
  - `PlateauDetected` [Integration]: Phát hiện tiến bộ đình trệ 3 tuần liên tiếp.
  - `OvertrainingDetected` [Integration]: Phát hiện tập quá tải RPE $\ge 8.5$ hoặc $\ge 2$ buổi/ngày.
  - `ScheduleIncompatible` [Integration]: Bỏ tập cùng ngày $\ge 3$ lần liên tiếp.
* **Read Models (Mô hình đọc)**:
  - `ActiveWorkoutRoadmapView`: Hiển thị lộ trình 4 tuần và các buổi tập dự kiến của tuần này.
  - `TodayWorkoutSession`: Hiển thị giáo án chi tiết (bài tập, tạ, set, rep) ngày hôm nay sau khi sinh JIT.
* **Policies (Chính sách)**:
  - *Policy: Khi nhận sự kiện `NewInjuryReported`* ──► Tự động phát lệnh `SubstituteExercise` để loại bỏ các bài tập tải lực lên vùng chấn thương.
  - *Policy: Khi nhận sự kiện `WorkoutSessionCompleted`* ──► Phát lệnh cập nhật lịch sử tập luyện và chạy quy trình quét `ProcessBehaviorSignal` (Trigger B). Nếu đến ngày cuối chu kỳ 4 tuần, phát lệnh `EvaluateRoadmapCycle` (Trigger A).

---

### 3. Workout Execution Context

* **Commands (Lệnh)**:
  - `StartWorkoutSession`: Check-in và lấy danh sách bài hát từ Audio Context.
  - `ProcessSkeletonFrame` (Nhánh AI): Xử lý từng khung hình video từ camera tại client để xác định 33 khớp.
  - `TriggerLocalAudioDucking` (Nhánh AI): Giảm âm lượng nhạc nền và phát Voice Alert sửa lỗi khi client phát hiện sai tư thế.
  - `LogWorkoutSet`: Lưu kết quả Set tập (rep, tạ, form score, RPE). Hỗ trợ tự ghi nhận set thủ công cho nhánh tập Phi AI (Form Score = N/A).
  - `OverrideWorkoutSetLog`: Cho phép người dùng sửa thủ công kết quả Set tập.
  - `CompleteWorkoutSession`: Kết thúc buổi tập, tính toán volume, calo tiêu thụ và gửi dữ liệu về Server.
  - `FlagAnomalousSession`: Gắn cờ và tự động đóng buổi tập bất thường nếu quá 240 phút không tương tác.
* **Domain & Integration Events (Sự kiện)**:
  - `WorkoutSessionStarted`: Buổi tập được bắt đầu.
  - `InvalidRepDetected` (Client-side): Phát hiện động tác sai tư thế.
  - `ValidRepCounted` (Client-side): Rep tập hợp lệ được ghi nhận (ROM% $\ge 70\%$).
  - `WorkoutSetSaved`: Set tập được ghi nhận (hệ thống tự điền hoặc chỉnh sửa thủ công).
  - `WorkoutSessionCompleted` [Integration]: Kết thúc buổi tập thành công (chứa calo tiêu thụ, volume, Form Score, cờ AI/Non-AI).
  - `AnomalousSessionFlagged`: Buổi tập bị gắn nhãn Anomalous và tự động đóng.
  - `WorkoutSessionRejected`: Buổi tập bị từ chối lưu tự động vì tỷ lệ nhận diện khung xương < 50% (chống gian lận khi dùng AI Camera).
* **Read Models (Mô hình đọc)**:
  - `LiveWorkoutHUD`: Màn hình overlay hiển thị khung xương skeleton và cảnh báo trực quan thời gian thực (nhánh AI), hoặc màn hình hiển thị Timer và hướng dẫn bài tập (nhánh Phi AI).
  - `PostSessionReportView`: Báo cáo tổng kết buổi tập (Tổng volume, calo tiêu thụ, điểm Form trung bình, lỗi tư thế thường gặp).
* **Policies (Chính sách)**:
  - *Policy: Khi nhận sự kiện `InvalidRepDetected`* ──► Kích hoạt ngay lệnh `TriggerLocalAudioDucking` tại client và phát Voice Alert cảnh báo sửa tư thế.
  - *Policy: Khi thời gian tập luyện vượt quá 240 phút* ──► Tự động kích hoạt lệnh `FlagAnomalousSession` để đóng buổi tập và lưu nhãn Anomalous.

---

### 4. AI Nutrition Context

* **Commands (Lệnh)**:
  - `CalculateTDEE`: Tính toán tổng lượng tiêu thụ calo hàng ngày theo Mifflin-St Jeor.
  - `GenerateMealSuggestions`: Tạo thực đơn gợi ý hàng ngày theo 3 nhóm ngân sách, hỗ trợ cả bữa ăn tự nấu và ăn ngoài quán (ưu tiên đối tác).
  - `LogMeal`: Người dùng ghi chép món ăn thực tế đã dùng.
  - `ApplyAntiRepetitionLock`: Khóa các nguồn nguyên liệu vừa ăn.
* **Domain & Integration Events (Sự kiện)**:
  - `TDEECalculated`: Calo mục tiêu được thiết lập.
  - `MealPlanGenerated`: Thực đơn gợi ý bữa ăn (tự nấu & ăn ngoài) đã sẵn sàng.
  - `AntiRepetitionLockApplied`: Nguồn protein/carbs/chủ đề món ăn được đưa vào danh sách khóa.
  - `MealLogged`: Người dùng lưu nhật ký ăn uống thành công.
* **Read Models (Mô hình đọc)**:
  - `DailyNutritionHUD`: Giao diện gợi ý các lựa chọn món ăn tự chuẩn bị hoặc ăn ngoài tiệm theo ngân sách, kèm trạng thái calo/macros còn lại trong ngày.
* **Policies (Chính sách)**:
  - *Policy: Khi nhận sự kiện `UserMetricsUpdated`* ──► Kích hoạt lệnh `CalculateTDEE` để cập nhật calo mục tiêu mới.
  - *Policy: Khi nhận sự kiện `MealLogged`* ──► Phát lệnh `ApplyAntiRepetitionLock` (khóa protein 7 ngày, carb 5 ngày, chủ đề món ăn 3 ngày).
  - *Policy: Khi nhận sự kiện `WorkoutSessionCompleted`* ──► Tự động trích xuất lượng `caloriesBurned` từ sự kiện hoàn thành buổi tập để phát lệnh cộng thêm calo nạp vào hạn mức năng lượng nạp trong ngày của người dùng.
