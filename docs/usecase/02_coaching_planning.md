# UC-02 Coaching & Planning

> Nguồn: [BRD](../NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](../02_bounded_context.md) · [Tactical Design](../03_ddd_tactical_design.md)

**Actor**: `User` (người tập) · `System` (AI Coach / AI Camera / AI Nutrition)  
**Format mỗi Use Case**: Precondition → Main Flow → Alternative Flow → Error / Edge Cases → Postcondition → Domain Events

---

### UC-02.1 InitiateWorkoutRoadmap

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Event `UserProfileCompleted` được nhận. `ActiveCoachEnabled = true`. |

**Main Flow**
1. System đọc `BiologicalMetrics`, `primary_goal`, `preferred_muscle_groups`, `available_equipment` và `available_slots` từ `User`.
2. System tính `UserFitnessScore` (thể trạng khởi điểm người mới - Baseline RPE 6–7).
3. System tạo `WorkoutRoadmap` 4 tuần (28 ngày) với `CompletionRate = 0`.
4. System phân bổ 4 tuần theo 4 pha tiến trình chuẩn (`BR-AC-09`): Tuần 1 (Accumulation RPE 6–7), Tuần 2 (Overload RPE 7–8), Tuần 3 (Peak RPE 8–9), Tuần 4 (Deload RPE 5–6 giảm 40–50% volume).
5. System phân bổ các ngày tập vào `available_slots`, tuân thủ tối đa 6 buổi/tuần và tối thiểu 1 ngày nghỉ hoàn toàn (`BR-AC-01`).
6. System tạo **28 `DailyWorkoutPlan` chi tiết** (`WorkoutPrescription` gồm bài tập, set, rep, tạ gợi ý, rest time, target RPE) tương ứng các ngày tập trong 4 tuần.
7. System gọi `OverloadValidator` kiểm soát giới hạn điều chỉnh tải trọng ($\pm 30\%$, `BR-AC-02`).
8. System lưu `WorkoutRoadmap` (chứa 4 tuần, 28 ngày và 28 giáo án chi tiết) và phát event `RoadmapInitiated`.

**Alternative Flow**
- A1: User có `Injury` active → System loại bỏ bài tập tác động vùng chấn thương khi sinh giáo án.
- A2: `preferred_muscle_groups` rỗng → System tự động áp dụng `MuscleSplit` cân bằng chuẩn (Push/Pull/Legs hoặc Upper/Lower split).

**Error / Edge Cases**
- E1: `BiologicalMetrics` không đủ dữ liệu → Hủy quy trình, yêu cầu hoàn thiện hồ sơ.
- E2: `OverloadValidator` từ chối bài tập (vượt trần 30%) → Hạ mức tạ về trần tối đa hợp lệ và tiếp tục.

**Postcondition**: `WorkoutRoadmap` 4 tuần và 28 `DailyWorkoutPlan` được khởi tạo sẵn sàng trong cơ sở dữ liệu.  
> *`CoachingService.InitiateRoadmap()` gọi `WorkoutRoadmapRepository.Save()`.*

**Domain Events**: `RoadmapInitiated`

---

### UC-02.2 ExecuteDailyWorkoutPlan

| | |
|---|---|
| **Actor** | System (AI Coach) · User |
| **Precondition** | `WorkoutRoadmap` đang ở trạng thái `Active`. Đến ngày tập theo lịch. |

**Main Flow**
1. User mở ứng dụng trong ngày tập. System đọc trực tiếp `DailyWorkoutPlan` đã khởi tạo của ngày hôm nay từ cơ sở dữ liệu.
2. System hiển thị câu hỏi Check-in ngắn về tình trạng sức khỏe hôm nay (chấn thương mới, độ mệt mỏi/giờ ngủ).
3. System kiểm tra trạng thái buổi tập trước đó. Nếu buổi trước bị bỏ tập ➔ Tự động đánh dấu buổi cũ là `Skipped` (`BR-AC-03`), giữ nguyên giáo án ngày hôm nay.
4. User nhận giáo án và bắt đầu buổi tập (chuyển sang UC-03 Workout Execution).

**Alternative Flow**
- A1: Check-in phát hiện chấn thương mới hoặc mệt mỏi cấp tính ➔ System kích hoạt Re-generate giáo án riêng cho ngày hôm nay (giảm tải hoặc đổi bài phục hồi nhẹ).
- A2: Ngày hôm nay là ngày nghỉ theo lịch ➔ System thông báo "Hôm nay nghỉ phục hồi".

**Error / Edge Cases**
- E1: `DailyWorkoutPlan` không tồn tại do dữ liệu lỗi ➔ System tự động sinh JIT giáo án cho ngày hôm nay theo `ScheduleDay`.

**Postcondition**: Giáo án của ngày hôm nay sẵn sàng cho User thực thi.  
> *`CoachingService.GetDailyPlan()` đọc trực tiếp từ `WorkoutRoadmapRepository`.*

**Domain Events**: `DailyWorkoutPlanExecuted`

---

### UC-02.3 ReevaluateScheduleOnProfileUpdated

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Event `ProfileUpdated` hoặc tín hiệu điều chỉnh (`Signal B1–B4`) được nhận. `WorkoutRoadmap` đang ở trạng thái `Active`. |

**Main Flow**
1. System đọc thông tin snapshot mới từ `User` (`available_equipment`, `available_slots`, `preferred_muscle_groups`, `primary_goal`).
2. System xác định danh sách các ngày chưa thực thi (`scheduled_date >= today` và `status != COMPLETED`).
3. System kích hoạt Re-generation cho các `DailyWorkoutPlan` chưa thực thi:
   - Cập nhật bài tập tương thích với `available_equipment` mới.
   - Điều chỉnh phân bổ ngày tập theo `available_slots` mới.
   - Điều chỉnh `MuscleSplit` theo `preferred_muscle_groups` và `primary_goal`.
4. System lưu thông tin cập nhật vào `WorkoutRoadmap` và phát event `RoadmapAdjusted`.

**Postcondition**: Toàn bộ giáo án các ngày chưa thực thi trong 4 tuần được cập nhật.  
> *`CoachingService.ReevaluateSchedule()` gọi `WorkoutRoadmapRepository.Save()` và phát `RoadmapAdjusted`.*

**Domain Events**: `RoadmapAdjusted`
