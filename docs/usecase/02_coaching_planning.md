# UC-02 Coaching & Planning

> Nguồn: [BRD](../NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](../02_bounded_context.md) · [Tactical Design](../03_ddd_tactical_design.md)

**Actor**: `User` (người tập) · `System` (AI Coach / AI Camera / AI Nutrition)  
**Format mỗi Use Case**: Precondition → Main Flow → Alternative Flow → Error / Edge Cases → Postcondition → Domain Events

---

### UC-02.1 InitiateWorkoutRoadmap

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | `UserProfileCompleted` event được nhận. `ActiveCoachEnabled = true`. |

**Main Flow**
1. System đọc `BiologicalMetrics` và mục tiêu từ `User`.
2. System tính `FitnessScore` và xác định giai đoạn khởi điểm.
3. System tạo `WorkoutRoadmap` (4 tuần) với `RoadmapPhase` và `CompletionRate = 0`.
4. System tạo `WeeklySchedule` đầu tiên với `MuscleSplit` phù hợp mục tiêu, tuân thủ `BR-AC-01`.
5. System gọi `OverloadValidator` để xác nhận volume tuần 1 hợp lệ.
6. System phát `RoadmapInitiated`.

**Alternative Flow**
- A1: User có `Injury` active → System loại bỏ bài tập tác động vùng chấn thương khi sinh lịch tuần.

**Error / Edge Cases**
- E1: `BiologicalMetrics` không đủ dữ liệu → không sinh được lộ trình, yêu cầu hoàn thiện hồ sơ.
- E2: `OverloadValidator` từ chối volume tuần 1 (quá cao) → tự điều chỉnh xuống và retry.

**Postcondition**: `WorkoutRoadmap` và `WeeklySchedule` tuần 1 được tạo. Chưa sinh `DailyWorkoutPlan`.  
> *`CoachingService.InitiateRoadmap()` gọi `WorkoutRoadmapRepository.Save()` và `WeeklyScheduleRepository.Save()`.*

**Domain Events**: `RoadmapInitiated` · `WeeklyScheduleGenerated`

---

### UC-02.2 GenerateDailyWorkoutPlan (JIT)

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Đến ngày tập theo `WeeklySchedule`. `DailyWorkoutPlan` chưa tồn tại cho ngày hôm nay. Trạng thái buổi tập trước đã được xử lý (hoặc đã xác nhận Bỏ qua/Dồn bù). |

**Main Flow**
1. User mở ứng dụng trong ngày tập, System hiển thị một câu hỏi tương tác ngắn (Check-in pop-up) về tình trạng sức khỏe hôm nay (phát hiện chấn thương mới, cảm giác mỏi mệt/độ phục hồi).
2. System kiểm tra trạng thái buổi tập trước đó theo lịch. Nếu buổi trước bị bỏ tập:
   - System kích hoạt đề xuất dồn/bù: Hỏi user muốn dồn buổi tập cũ sang hôm nay (và đẩy lịch các buổi sau) hay bỏ qua.
   - Nếu user chọn dồn/bù → System cập nhật lại `WeeklySchedule` và tiến hành sinh giáo án cho buổi tập bị dồn đó.
   - Nếu user chọn bỏ qua (hoặc không phản hồi) → System đánh dấu buổi tập cũ là `Skipped` (`BR-AC-03`), tiếp tục sinh giáo án cho ngày hôm nay theo lịch.
3. System sinh `WorkoutPrescription` (bài tập, set, rep, tạ gợi ý, warm-up/cool-down) dựa trên `MuscleSplit` của ngày hôm nay và `PersonalRecord` (1RM) hiện tại.
4. System kiểm tra `Injury` active — loại bỏ bài tập tác động vùng chấn thương và thay thế bằng bài tương đương.
5. System tạo `DailyWorkoutPlan`, phát `DailyWorkoutPlanGenerated`.

**Alternative Flow**
- A1: User mở app và hỏi trạng thái hôm nay qua chatbot — System hỏi 1–2 câu về thiết bị, dị ứng nếu chưa có trong `ChatbotContext`.
- A2: Ngày hôm nay là ngày nghỉ theo lịch → không sinh giáo án, thông báo "Hôm nay nghỉ phục hồi".
- A3: Buổi tập gần nhất bị tự động đóng vì không tương tác (Anomalous Session - đánh dấu từ UC-03.4) → System bắt buộc sinh giáo án phục hồi nhẹ (Active Recovery), không cho phép tập nặng ở buổi này (BR-WL-01).

**Error / Edge Cases**
- E1: Không tìm thấy `WeeklySchedule` cho ngày hôm nay → báo lỗi, đề xuất tạo lịch mới.
- E2: Toàn bộ bài tập bị loại bỏ do chấn thương → sinh giáo án phục hồi nhẹ (active recovery), không tập nặng.
- E3: `PersonalRecord` chưa có (user mới) → dùng tạ gợi ý mặc định theo `BiologicalMetrics`.

**Postcondition**: `DailyWorkoutPlan` với `WorkoutPrescription` đầy đủ sẵn sàng để user thực thi.  
> *`CoachingService.GenerateDailyPlan()` gọi `DailyWorkoutPlanRepository.Save()` và `WorkoutPerformanceRepository.GetLatest1RM()`.*

**Domain Events**: `DailyWorkoutPlanGenerated`

---

### UC-02.3 GenerateNextWeeklySchedule

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Lịch tuần hiện tại chuẩn bị kết thúc (hoặc kết thúc). `WorkoutRoadmap` đang ở trạng thái `Active`. |

**Main Flow**
1. System đọc lịch sử tập luyện thực tế của tuần vừa rồi (tổng volume thực tế).
2. System sinh `WeeklySchedule` tiếp theo (tuần 2, 3, hoặc 4) của lộ trình hiện tại, tuân thủ `BR-AC-01`.
3. System gọi `OverloadValidator` để kiểm tra Progressive Overload của lịch tuần mới không vượt quá 10% volume thực tế của tuần trước (BR-AC-02).
4. System lưu `WeeklySchedule` mới và phát `WeeklyScheduleGenerated`.

**Alternative Flow**
- A1: `OverloadValidator` từ chối volume tuần mới (vượt 10%) → AI Coach tự động điều chỉnh tải trọng (giảm volume/cường độ tạ) xuống ngưỡng hợp lệ và kiểm tra lại.

**Error / Edge Cases**
- E1: Không tìm thấy volume tuần trước (dữ liệu bị lỗi hoặc tuần trước bị bỏ qua hoàn toàn) → Dùng volume mặc định theo thiết lập ban đầu của lộ trình.

**Postcondition**: `WeeklySchedule` tuần tiếp theo được tạo sẵn sàng để sinh giáo án hàng ngày.
> *`CoachingService.GenerateWeeklySchedule()` gọi `WeeklyScheduleRepository.Save()` và phát `WeeklyScheduleGenerated`.*

**Domain Events**: `WeeklyScheduleGenerated`
