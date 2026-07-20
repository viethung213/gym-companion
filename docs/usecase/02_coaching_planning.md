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
1. System đọc `BiologicalMetrics`, `primary_goal`/`secondary_goals`, `experience_level`, `availability`, `preferred_muscle_groups` và `available_equipment` từ `User`.
2. System tính `FitnessScore`, xác định giai đoạn khởi điểm và suy ra `PlanningTier` (Beginner | Experienced) từ `experience_level` theo BR-AC-11.
3. System tạo `WorkoutRoadmap` (4 tuần) với `RoadmapPhase`, `PlanningTier`, `PrimaryGoal` và `CompletionRate = 0`.
4. System xác định số buổi/tuần khả thi bằng cách đối chiếu số buổi mục tiêu với số slot rảnh trong `availability` (BR-AC-09) — nếu slot rảnh ít hơn, hạ số buổi xuống đúng số slot.
5. System tạo `WeeklySchedule` đầu tiên: xếp buổi vào đúng slot rảnh, phân bổ `MuscleSplit` theo `primary_goal` và `preferred_muscle_groups` (tuân thủ sàn cân bằng + phục hồi ≥48h của BR-AC-10), chỉ chọn bài tập trong `available_equipment` (BR-AC-13), tuân thủ `BR-AC-01`.
6. System gọi `OverloadValidator` để xác nhận volume tuần 1 hợp lệ theo trần Overload tương ứng `PlanningTier` (BR-AC-11).
7. System phát `RoadmapInitiated`.

**Alternative Flow**
- A1: User có `Injury` active → System loại bỏ bài tập tác động vùng chấn thương khi sinh lịch tuần.
- A2: `PlanningTier = Beginner` → System bắt buộc chọn từ bộ Fixed Template (split đơn giản, bài compound nền tảng, có biến thể theo `available_equipment`) thay vì để AI tự chọn bài tự do (BR-AC-11).
- A3: `secondary_goals` xung đột với `primary_goal` mà chưa được User xác nhận ở Onboarding (UC-01.2 A3) → chặn `InitiateRoadmap`, yêu cầu hoàn thiện lại hồ sơ (BR-AC-14).

**Error / Edge Cases**
- E1: `BiologicalMetrics` không đủ dữ liệu → không sinh được lộ trình, yêu cầu hoàn thiện hồ sơ.
- E2: `OverloadValidator` từ chối volume tuần 1 (quá cao) → tự điều chỉnh xuống và retry.
- E3: `availability` có 0 slot rảnh (User bỏ qua ở Onboarding và chưa cập nhật) → dùng mặc định 3 buổi tối/tuần không cố định ngày, cảnh báo User nên cập nhật availability thật để lịch chính xác hơn.
- E4: `available_equipment` rỗng → chỉ chọn bài Bodyweight cho toàn bộ `WeeklySchedule` (BR-AC-13), không chặn sinh lộ trình.

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
3. System chọn bài tập cho `WorkoutPrescription` trong danh mục đã lọc theo `available_equipment` (BR-AC-13), ưu tiên loại các accessory/finisher đã dùng cho cùng nhóm cơ trong 2 tuần gần nhất (BR-AC-15) — trừ compound nền tảng của Fixed Template nếu `PlanningTier = Beginner` (BR-AC-11, vốn cố định có chủ đích). Set/rep/tạ gợi ý tính từ `PersonalRecord` (1RM) hiện tại.
4. System kiểm tra `Injury` active — loại bỏ bài tập tác động vùng chấn thương và thay thế bằng bài tương đương (vẫn trong `available_equipment`).
5. System tạo `DailyWorkoutPlan`, phát `DailyWorkoutPlanGenerated`.

**Alternative Flow**
- A1: User mở app và hỏi trạng thái hôm nay qua chatbot — System hỏi 1–2 câu về dị ứng nếu chưa có trong `ChatbotContext` (thiết bị/dụng cụ đã thu thập chính thức ở Onboarding qua `available_equipment`, không hỏi lại qua chatbot).
- A2: Ngày hôm nay là ngày nghỉ theo lịch → không sinh giáo án, thông báo "Hôm nay nghỉ phục hồi".
- A3: Buổi tập gần nhất bị tự động đóng vì không tương tác (Anomalous Session - đánh dấu từ UC-03.4) → System bắt buộc sinh giáo án phục hồi nhẹ (Active Recovery), không cho phép tập nặng ở buổi này (BR-WL-01).

**Error / Edge Cases**
- E1: Không tìm thấy `WeeklySchedule` cho ngày hôm nay → báo lỗi, đề xuất tạo lịch mới.
- E2: Toàn bộ bài tập bị loại bỏ do chấn thương → sinh giáo án phục hồi nhẹ (active recovery), không tập nặng.
- E3: `PersonalRecord` chưa có (user mới) → dùng tạ gợi ý mặc định theo `BiologicalMetrics`.
- E4: Sau khi lọc theo `available_equipment` + tránh lặp (BR-AC-15), không còn đủ bài cho nhóm cơ hôm nay → nới lỏng ràng buộc chống lặp trước (cho phép lặp lại bài cũ) thay vì vi phạm ràng buộc dụng cụ/chấn thương.

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
1. System đọc lịch sử tập luyện thực tế của tuần vừa rồi (tổng volume thực tế, RPE trung bình, delta 1RM) và cập nhật `FitScore` (BR-AC-12).
2. System sinh `WeeklySchedule` tiếp theo (tuần 2, 3, hoặc 4) của lộ trình hiện tại, xếp vào đúng slot rảnh hiện hành trong `availability` (BR-AC-09) và tuân thủ `BR-AC-01`.
3. System gọi `OverloadValidator` để kiểm tra Progressive Overload của lịch tuần mới không vượt quá trần theo `PlanningTier` (10% cho Experienced, 5% cho Beginner theo BR-AC-11) so với volume thực tế của tuần trước.
4. Nếu `FitScore` lệch nhẹ về hướng Too-Little hoặc Too-Much nhưng chưa đủ điều kiện kích hoạt Signal B3/B4 → System điều chỉnh nhẹ tải lượng trong biên trần ở bước 3 (BR-AC-12).
5. System lưu `WeeklySchedule` mới và phát `WeeklyScheduleGenerated`.

**Alternative Flow**
- A1: `OverloadValidator` từ chối volume tuần mới (vượt trần theo `PlanningTier` — BR-AC-11) → AI Coach tự động điều chỉnh tải trọng (giảm volume/cường độ tạ) xuống ngưỡng hợp lệ và kiểm tra lại.

**Error / Edge Cases**
- E1: Không tìm thấy volume tuần trước (dữ liệu bị lỗi hoặc tuần trước bị bỏ qua hoàn toàn) → Dùng volume mặc định theo thiết lập ban đầu của lộ trình.

**Postcondition**: `WeeklySchedule` tuần tiếp theo được tạo sẵn sàng để sinh giáo án hàng ngày.
> *`CoachingService.GenerateWeeklySchedule()` gọi `WeeklyScheduleRepository.Save()` và phát `WeeklyScheduleGenerated`.*

**Domain Events**: `WeeklyScheduleGenerated`
