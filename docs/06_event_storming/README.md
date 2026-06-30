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
  * `RegisterUser`: Đăng ký tài khoản mới.
  * `SubmitHealthProfile`: Nhập thông tin sinh học (tuổi, giới tính, chiều cao, cân nặng, mục tiêu).
  * `ReportInjury`: Khai báo vùng chấn thương mới.
  * `LogBodyMetrics`: Ghi nhận thêm chỉ số cơ thể mới (cân nặng, tỷ lệ mỡ %, ảnh tiến trình) vào lịch sử theo thời gian.
* **Domain Events (Sự kiện miền)**:
  * `UserRegistered`: Tài khoản người dùng được tạo thành công.
  * `ProfileCompleted`: Hồ sơ đạt mức độ hoàn thiện $\ge 80\%$ (kích hoạt AI Coach).
  * `InjuryReported`: Phát hiện chấn thương mới được khai báo.
  * `BodyMetricsLogged`: Bản ghi chỉ số cơ thể mới được thêm vào lịch sử thành công.
  * `UserMetricsUpdated`: Chỉ số cơ thể hiện tại của người dùng thay đổi (sau khi đồng bộ từ bản ghi mới).
* **Read Models (Mô hình đọc)**:
  * `HealthProfileOverview`: Xem chi tiết chỉ số cơ thể hiện tại và trạng thái kích hoạt dịch vụ.
  * `BodyMetricsHistoryView`: Xem biểu đồ diễn biến cân nặng và tỷ lệ mỡ theo thời gian tập.
* **Policies (Chính sách)**:
  * *Policy: Khi nhận sự kiện `ProfileCompleted`* ──► Tự động phát lệnh khởi tạo AI Coach.
  * *Policy: Khi nhận sự kiện `BodyMetricsLogged`* ──► Phát lệnh đồng bộ chỉ số hiện tại của Aggregate `User` và phát sự kiện `UserMetricsUpdated` ra toàn hệ thống.

---

### 2. AI Coaching & Planning Context

* **Commands (Lệnh)**:
  * `ActivateAICoach`: Kích hoạt huấn luyện viên ảo và chọn phong cách (Drill Sergeant, Best Friend, Data Analyst).
  * `GenerateWorkoutPlan`: Khởi tạo giáo án 4 tuần đầu tiên.
  * `AdjustSchedule`: Tự động tinh chỉnh giáo án sau 2 tuần.
  * `SubstituteExercise`: Thay thế bài tập tập trung vào nhóm cơ bị chấn thương.
  * `ScheduleWorkoutReminder`: Đặt lịch gửi tin nhắn nhắc nhở trước 15 phút.
* **Domain Events (Sự kiện miền)**:
  * `AICoachActivated`: AI Coach được cấu hình thành công.
  * `WorkoutPlanGenerated`: Giáo án 4 tuần được tạo.
  * `WeeklyScheduleAdjusted`: Lịch tập trong tuần được điều chỉnh độ khó hoặc chèn Deload Week.
  * `ExerciseSubstituted`: Bài tập được thay thế thành công để tránh nhóm cơ chấn thương.
  * `WorkoutReminderScheduled`: Đã thiết lập thông báo nhắc nhở thành công.
* **Read Models (Mô hình đọc)**:
  * `ActiveWorkoutPlanView`: Giao diện hiển thị giáo án 4 tuần hiện tại.
  * `TodayWorkoutSession`: Danh sách bài tập cần thực hiện trong ngày hôm nay.
* **Policies (Chính sách)**:
  * *Policy: Khi nhận sự kiện `InjuryReported`* ──► Tự động phát lệnh `SubstituteExercise` để loại bỏ các bài tập tải lực lên vùng chấn thương.
  * *Policy: Khi nhận sự kiện `WorkoutSessionCompleted` (được gửi từ Workout Execution Context) và đạt chu kỳ 2 tuần* ──► Kích hoạt lệnh `AdjustSchedule` để đánh giá hiệu suất và điều chỉnh độ khó.

---

### 3. Workout Execution Context

* **Commands (Lệnh)**:
  * `StartWorkoutSession`: Check-in và bắt đầu buổi tập.
  * `ProcessSkeletonFrame`: Xử lý từng khung hình video từ camera để xác định 33 khớp.
  * `TriggerAudioDucking`: Giảm âm lượng nhạc nền.
  * `LogWorkoutSet`: Lưu kết quả Set tập (số rep, cân nặng, điểm kỹ thuật, RPE).
  * `OverrideWorkoutSetLog`: Người dùng sửa thủ công kết quả Set tập nếu AI nhận diện sai.
  * `CompleteWorkoutSession`: Kết thúc buổi tập và xuất báo cáo.
* **Domain Events (Sự kiện miền)**:
  * `WorkoutSessionStarted`: Buổi tập được bắt đầu.
  * `InvalidRepDetected`: Phát hiện động tác sai tư thế (lệch góc ROM, sai tư thế lưng...).
  * `ValidRepCounted`: Rep tập hợp lệ được ghi nhận (ROM% $\ge 70\%$).
  * `AudioDuckingTriggered`: Âm lượng nhạc nền được giảm xuống.
  * `WorkoutSetSaved`: Set tập được ghi nhận (hệ thống tự điền hoặc chỉnh sửa thủ công).
  * `WorkoutSessionCompleted`: Kết thúc buổi tập thành công và gửi dữ liệu về Server.
  * `WorkoutSessionRejected`: Buổi tập bị từ chối lưu tự động vì tỷ lệ nhận diện khung xương < 50% (chống gian lận).
* **Read Models (Mô hình đọc)**:
  * `LiveWorkoutHUD`: Màn hình overlay hiển thị khung xương skeleton và cảnh báo trực quan thời gian thực.
  * `PostSessionReportView`: Báo cáo tổng kết buổi tập (Tổng volume, điểm Form trung bình, lỗi tư thế thường gặp).
* **Policies (Chính sách)**:
  * *Policy: Khi nhận sự kiện `InvalidRepDetected`* ──► Phát lệnh `TriggerAudioDucking` đồng thời phát âm thanh cảnh báo Voice Alert sửa tư thế.
  * *Policy: Khi nhận sự kiện `WorkoutSessionCompleted`* ──► Phát sự kiện tích hợp ra toàn hệ thống để cập nhật Meal Plan và lịch sử tập luyện.

---

### 4. AI Nutrition Context

* **Commands (Lệnh)**:
  * `CalculateTDEE`: Tính toán tổng lượng tiêu thụ calo hàng ngày.
  * `GenerateMealSuggestions`: Tạo thực đơn gợi ý hàng ngày theo 3 nhóm ngân sách.
  * `LogMeal`: Người dùng ghi chép món ăn thực tế đã dùng.
  * `ApplyAntiRepetitionLock`: Khóa các nguồn nguyên liệu vừa ăn.
* **Domain Events (Sự kiện miền)**:
  * `TDEECalculated`: Calo mục tiêu được thiết lập.
  * `MealPlanGenerated`: Thực đơn 3 bữa + 1 bữa phụ đã được chuẩn bị.
  * `AntiRepetitionLockApplied`: Nguồn protein/carbs/chủ đề món ăn được đưa vào danh sách khóa.
  * `MealLogged`: Người dùng lưu nhật ký ăn uống thành công.
* **Read Models (Mô hình đọc)**:
  * `DailyNutritionHUD`: Giao diện gợi ý 3 lựa chọn món ăn kèm trạng thái calo/macros còn lại trong ngày.
* **Policies (Chính sách)**:
  * *Policy: Khi nhận sự kiện `UserMetricsUpdated`* ──► Kích hoạt lệnh `CalculateTDEE` để cập nhật calo mục tiêu mới.
  * *Policy: Khi nhận sự kiện `MealLogged`* ──► Phát lệnh `ApplyAntiRepetitionLock` (khóa protein 7 ngày, carb 5 ngày, chủ đề món ăn 3 ngày).
  * *Policy: Khi nhận sự kiện `WorkoutCaloriesBurned`* ──► Tự động cộng calo tiêu thụ thực tế từ buổi tập vào hạn mức năng lượng nạp trong ngày.
