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
1. System đọc `BiologicalMetrics`, `primary_goal`, `secondary_goal` (nếu có), `experience_level`, `availability` và `available_equipment` từ `User`.
2. System tính `FitnessScore`, xác định giai đoạn khởi điểm và suy ra `PlanningTier` (Beginner | Experienced) từ `experience_level` theo BR-AC-12.
3. System tạo `WorkoutRoadmap` (4 tuần) với `RoadmapPhase`, `PlanningTier`, `PrimaryGoal` và `CompletionRate = 0`.
4. System xác định số buổi/tuần khả thi bằng cách đối chiếu số buổi mục tiêu với số slot rảnh trong `availability` (BR-AC-10) — nếu slot rảnh ít hơn, hạ số buổi xuống đúng số slot.
5. System tạo `WeeklySchedule` đầu tiên: xếp buổi vào đúng slot rảnh, phân bổ `MuscleSplit` theo `primary_goal` và `secondary_goal` (tuân thủ sàn cân bằng + phục hồi ≥48h của BR-AC-11), chỉ chọn bài tập trong `available_equipment` (BR-AC-14), tuân thủ `BR-AC-01`.
6. System gọi `OverloadValidator` để xác nhận volume tuần 1 hợp lệ theo trần Overload tương ứng `PlanningTier` (BR-AC-12).
7. System phát `RoadmapInitiated`.

**Alternative Flow**
- A1: User có `Injury` active → System loại bỏ bài tập tác động vùng chấn thương khi sinh lịch tuần.
- A2: `PlanningTier = Beginner` → System bắt buộc chọn từ bộ Fixed Template (split đơn giản, bài compound nền tảng, có biến thể theo `available_equipment`) thay vì để AI tự chọn bài tự do (BR-AC-12).

**Error / Edge Cases**
- E1: `BiologicalMetrics` không đủ dữ liệu → không sinh được lộ trình, yêu cầu hoàn thiện hồ sơ.
- E2: `OverloadValidator` từ chối volume tuần 1 (quá cao) → tự điều chỉnh xuống và retry.
- E3: `availability` có 0 slot rảnh (User bỏ qua ở Onboarding và chưa cập nhật) → dùng mặc định 3 buổi tối/tuần không cố định ngày, cảnh báo User nên cập nhật availability thật để lịch chính xác hơn.
- E4: `available_equipment` rỗng → chỉ chọn bài Bodyweight cho toàn bộ `WeeklySchedule` (BR-AC-13), không chặn sinh lộ trình.

**Postcondition**: `WorkoutRoadmap` và `WeeklySchedule` tuần 1 được tạo. Chưa sinh `DailyWorkoutPlan`.  
> *`CoachingService.InitiateRoadmap()` gọi `WorkoutRoadmapRepository.Save()` và `WeeklyScheduleRepository.Save()`.*

**Domain Events**: `RoadmapInitiated` · `WeeklyScheduleGenerated`

---

### UC-02.2 ActivateDailyWorkoutPlan

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Đến ngày tập theo `WeeklySchedule`. Giáo án nháp (`draft_cached`) cho ngày hôm nay đã được Backend sinh sẵn sau khi buổi tập hôm qua kết thúc. Trạng thái buổi tập trước đã được xử lý (hoặc đã xác nhận Bỏ qua/Dồn bù). |

**Main Flow**

*Happy Path:*
1. User mở ứng dụng trong ngày tập, System hiển thị Check-in pop-up (tình trạng sức khỏe, chấn thương mới, mỏi mệt).
2. User xác nhận không có chấn thương mới và thiết bị không thay đổi.
3. System kiểm tra trạng thái buổi tập trước đó theo lịch. Nếu buổi trước bị bỏ tập:
   - Hỏi user muốn dồn buổi cũ sang hôm nay hay bỏ qua.
   - Nếu user chọn dồn/bù → System cập nhật lại `WeeklySchedule`; Backend sinh lại draft cho buổi dồn đó.
   - Nếu user chọn bỏ qua (hoặc không phản hồi) → System đánh dấu buổi tập cũ là `Skipped` (`BR-AC-03`).
4. System activate `draft_cached` → `DailyWorkoutPlan` chuyển sang trạng thái `active`, phát `DailyWorkoutPlanActivated`.

*Regenerate Path:*
1. User mở ứng dụng, Check-in pop-up xuất hiện.
2. User báo chấn thương mới **hoặc** tự đề cập thiết bị thay đổi đột xuất (xem A1).
3. AI Agent gọi `UpdateWorkoutContext` với `avoid_joints` mới và/hoặc `override_equipments` tạm thời.
4. Backend Go xóa `draft_cached` cũ, chạy background job sinh draft mới (stream NDJSON nếu draft chưa kịp sẵn sàng).
5. System activate draft mới → `DailyWorkoutPlan` chuyển sang trạng thái `active`, phát `DailyWorkoutPlanActivated`.

**Alternative Flow**
- A1: User tự đề cập thiết bị thay đổi đột xuất qua chatbot (ví dụ: *"Hôm nay tôi không có tạ, chỉ có Bodyweight"*) — AI Agent gọi `UpdateWorkoutContext(override_equipments=["bodyweight"])` để ghi đè tạm thời **cho 1 buổi này**; `available_equipment` trong Profile không bị thay đổi. Lưu ý: System **không chủ động hỏi** về thiết bị mỗi buổi — chỉ phản ứng khi user tự đề cập.
- A2: Ngày hôm nay là ngày nghỉ theo lịch → không activate giáo án, thông báo "Hôm nay nghỉ phục hồi".
- A3: Buổi tập gần nhất bị tự động đóng vì không tương tác (Anomalous Session - đánh dấu từ UC-03.4) → System bắt buộc activate giáo án phục hồi nhẹ (Active Recovery), không cho phép tập nặng ở buổi này (BR-WL-01).

**Error / Edge Cases**
- E1: Không tìm thấy `WeeklySchedule` cho ngày hôm nay → báo lỗi, đề xuất tạo lịch mới.
- E2: `draft_cached` không tồn tại (lần đầu dùng app hoặc cache bị mất) → Backend sinh JIT tại chỗ; Client hiển thị Warm-up cũ ngay lập tức (0ms) trong khi Backend stream bài tập chính qua NDJSON.
- E3: Toàn bộ bài tập bị loại bỏ do chấn thương → activate giáo án phục hồi nhẹ (active recovery).
- E4: `PersonalRecord` chưa có (user mới) → dùng tạ gợi ý mặc định theo `BiologicalMetrics`.
- E5: Sau khi lọc theo `available_equipment` + tránh lặp (BR-AC-16), không còn đủ bài cho nhóm cơ hôm nay → nới lỏng ràng buộc chống lặp trước (cho phép lặp lại bài cũ) thay vì vi phạm ràng buộc dụng cụ/chấn thương.

**Postcondition**: `DailyWorkoutPlan` với `WorkoutPrescription` đầy đủ sẵn sàng để user thực thi.  
> *`CoachingService.ActivateDailyPlan()` gọi `DailyWorkoutPlanRepository.Activate()`.*

**Domain Events**: `DailyWorkoutPlanActivated`

---

### UC-02.3 GenerateNextWeeklySchedule

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Lịch tuần hiện tại chuẩn bị kết thúc (hoặc kết thúc). `WorkoutRoadmap` đang ở trạng thái `Active`. |

**Main Flow**
1. System đọc lịch sử tập luyện thực tế của tuần vừa rồi (tổng volume thực tế, RPE trung bình, delta 1RM) và cập nhật `FitScore` (BR-AC-13).
2. System sinh `WeeklySchedule` tiếp theo (tuần 2, 3, hoặc 4) của lộ trình hiện tại, xếp vào đúng slot rảnh hiện hành trong `availability` (BR-AC-10) và tuân thủ `BR-AC-01`.
3. System gọi `OverloadValidator` để kiểm tra Progressive Overload của lịch tuần mới không vượt quá trần theo `PlanningTier` (10% cho Experienced, 5% cho Beginner theo BR-AC-12) so với volume thực tế của tuần trước.
4. Nếu `FitScore` lệch nhẹ về hướng Too-Little hoặc Too-Much nhưng chưa đủ điều kiện kích hoạt Signal B3/B4 → System điều chỉnh nhẹ tải lượng trong biên trần ở bước 3 (BR-AC-13).
5. System lưu `WeeklySchedule` mới và phát `WeeklyScheduleGenerated`.

**Alternative Flow**
- A1: `OverloadValidator` từ chối volume tuần mới (vượt trần theo `PlanningTier` — BR-AC-12) → AI Coach tự động điều chỉnh tải trọng (giảm volume/cường độ tạ) xuống ngưỡng hợp lệ và kiểm tra lại.

**Error / Edge Cases**
- E1: Không tìm thấy volume tuần trước (dữ liệu bị lỗi hoặc tuần trước bị bỏ qua hoàn toàn) → Dùng volume mặc định theo thiết lập ban đầu của lộ trình.

**Postcondition**: `WeeklySchedule` tuần tiếp theo được tạo sẵn sàng để sinh giáo án hàng ngày.
> *`CoachingService.GenerateWeeklySchedule()` gọi `WeeklyScheduleRepository.Save()` và phát `WeeklyScheduleGenerated`.*

**Domain Events**: `WeeklyScheduleGenerated`
