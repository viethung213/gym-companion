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
4. System tạo `WeeklySchedule` đầu tiên với `MuscleSplit` phù hợp mục tiêu.
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
| **Precondition** | Đến ngày tập theo `WeeklySchedule`. `DailyWorkoutPlan` chưa tồn tại cho ngày hôm nay. |

**Main Flow**
1. System kiểm tra trạng thái sức khỏe hôm nay (RPE buổi trước, chấn thương mới).
2. System sinh `WorkoutPrescription` (bài tập, set, rep, tạ gợi ý, warm-up/cool-down) dựa trên `MuscleSplit` của ngày và `PersonalRecord` (1RM) hiện tại.
3. System kiểm tra `Injury` active — loại bỏ bài tập tác động vùng chấn thương và thay thế bằng bài tương đương.
4. System tạo `DailyWorkoutPlan`, phát `DailyWorkoutPlanGenerated`.

**Alternative Flow**
- A1: User mở app và hỏi trạng thái hôm nay qua chatbot — System hỏi 1–2 câu về thiết bị, dị ứng nếu chưa có trong `ChatbotContext`.
- A2: Ngày hôm nay là ngày nghỉ theo lịch → không sinh giáo án, thông báo "Hôm nay nghỉ phục hồi".

**Error / Edge Cases**
- E1: Không tìm thấy `WeeklySchedule` cho ngày hôm nay → báo lỗi, đề xuất tạo lịch mới.
- E2: Toàn bộ bài tập bị loại bỏ do chấn thương → sinh giáo án phục hồi nhẹ (active recovery), không tập nặng.
- E3: `PersonalRecord` chưa có (user mới) → dùng tạ gợi ý mặc định theo `BiologicalMetrics`.

**Postcondition**: `DailyWorkoutPlan` với `WorkoutPrescription` đầy đủ sẵn sàng để user thực thi.  
> *`CoachingService.GenerateDailyPlan()` gọi `DailyWorkoutPlanRepository.Save()` và `WorkoutPerformanceRepository.GetLatest1RM()`.*

**Domain Events**: `DailyWorkoutPlanGenerated`
