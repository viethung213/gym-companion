# FITAI — Use Case Specification (Application Layer)

> Nguồn: [BRD](./NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](./02_bounded_context.md) · [Tactical Design](./03_ddd_tactical_design.md)

**Actor**: `User` (người tập) · `System` (AI Coach / AI Camera / AI Nutrition)  
**Format mỗi Use Case**: Precondition → Main Flow → Alternative Flow → Error / Edge Cases → Postcondition → Domain Events

---

## UC-01 Onboarding

---

### UC-01.1 RegisterUser

| | |
|---|---|
| **Actor** | User |
| **Precondition** | User chưa có tài khoản. |

**Main Flow**
1. User cung cấp Email hoặc SĐT.
2. System gửi OTP xác thực.
3. User nhập OTP hợp lệ.
4. System tạo tài khoản, trả về session token.

**Alternative Flow**
- A1: Đăng ký qua Google / Apple / Facebook — System nhận OAuth token, tạo tài khoản liên kết.

**Error / Edge Cases**
- E1: OTP sai 3 lần → khóa 15 phút.
- E2: SĐT/Email đã tồn tại → trả lỗi `ACCOUNT_ALREADY_EXISTS`.
- E3: OTP hết hạn (5 phút) → yêu cầu gửi lại.

**Postcondition**: Tài khoản `User` được tạo với trạng thái `Incomplete`. `ActiveCoachEnabled = false`.

**Domain Events**: —

---

### UC-01.2 CompleteHealthProfile

| | |
|---|---|
| **Actor** | User |
| **Precondition** | User đã đăng nhập. Hồ sơ chưa hoàn thiện ≥ 80%. |

**Main Flow**
1. User nhập tuổi, giới tính, chiều cao, cân nặng, mục tiêu (Tăng cơ / Giảm mỡ), khung giờ tập cố định.
2. System tính `ProfileCompletionRate` dựa trên các trường bắt buộc của `BiologicalMetrics`.
3. Khi tỷ lệ ≥ 80%, System kích hoạt `ActiveCoachEnabled = true`.

**Alternative Flow**
- A1: User bỏ qua bước nhập — System lưu trạng thái hiện tại, nhắc lại ở lần mở app tiếp theo.

**Error / Edge Cases**
- E1: Giá trị cân nặng / chiều cao không hợp lệ (≤ 0) → từ chối lưu, hiển thị lỗi inline.
- E2: Hoàn thiện < 80% → `ActiveCoachEnabled` giữ `false`, không sinh lộ trình.

**Postcondition**: `User.BiologicalMetrics` được cập nhật. Nếu đủ điều kiện, `UserProfileCompleted` được phát.  
> *`UserService.CompleteProfile()` gọi `UserRepository.Save()` và publish `UserProfileCompleted`.*

**Domain Events**: `UserProfileCompleted`

---

### UC-01.3 ReportInjury

| | |
|---|---|
| **Actor** | User |
| **Precondition** | User đã đăng nhập. |

**Main Flow**
1. User chọn vùng cơ bị thương (ví dụ: `Shoulder`, `Knee`) và mô tả ngắn.
2. System thêm `Injury` vào `User` với trạng thái `Active`.
3. System phát `InjuryReported` để `Coaching Context` loại bỏ bài tập tác động vùng đó.

**Alternative Flow**
- A1: User báo đã hồi phục → System cập nhật `Injury.status = Recovered`, phát `InjuryRecovered`.

**Error / Edge Cases**
- E1: Vùng cơ chọn không hợp lệ (không có trong danh sách) → từ chối.

**Postcondition**: `Injury` được ghi nhận. Giáo án sắp tới sẽ không chứa bài tập tác động vùng chấn thương.

**Domain Events**: `InjuryReported` | `InjuryRecovered`

---

## UC-02 Coaching & Planning

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

---

## UC-03 Workout Execution

---

### UC-03.1 StartWorkoutSession

| | |
|---|---|
| **Actor** | User |
| **Precondition** | `DailyWorkoutPlan` đã tồn tại. Không có `WorkoutSession` nào đang `InProgress` cho User. |

**Main Flow**
1. User check-in, chọn playlist âm nhạc.
2. System tạo `WorkoutSession` với trạng thái `InProgress`, khởi động `SessionTimer`.
3. System phát `WorkoutSessionStarted`.

**Error / Edge Cases**
- E1: Đã có session `InProgress` → từ chối, hiển thị tuỳ chọn tiếp tục hoặc đóng session cũ.
- E2: Không tìm thấy `DailyWorkoutPlan` hôm nay → kích hoạt UC-02.2 trước.

**Postcondition**: `WorkoutSession` `InProgress`.  

**Domain Events**: `WorkoutSessionStarted`

---

### UC-03.2 LogSet — Nhánh AI Camera

| | |
|---|---|
| **Actor** | User · System (AI Camera) |
| **Precondition** | `WorkoutSession` đang `InProgress`. Bài tập hiện tại có hỗ trợ AI Camera. Camera đã được cấp quyền. |

**Main Flow**
1. User bật camera, System hiệu chỉnh khoảng cách (1.5m–2m) qua `CalibrationConfig`.
2. System tải `PoseTemplate` và `RepCountingRules` từ `MotionSpecification` của bài tập.
3. System tracking 33 điểm khớp theo thời gian thực.
4. Mỗi rep: Nếu ROM% ≥ 70% → `repCount++`, tính `FormScore`. Nếu ROM% < 70% → rep không hợp lệ.
5. Nếu phát hiện lỗi tư thế → Audio Ducking + phát cảnh báo giọng nói (độ trễ < 500ms).
6. User xác nhận kết quả set. System tạo `WorkoutSetLog` (rep, tạ, FormScore trung bình, RPE).
7. System bắt đầu đếm ngược nghỉ.

**Alternative Flow**
- A1: User tự sửa kết quả set trước khi xác nhận.
- A2: Ánh sáng không đủ / bài nằm sàn → System tự động chuyển sang nhánh Phi AI (UC-03.3).

**Error / Edge Cases**
- E1: Tỷ lệ frame skeleton hợp lệ < 50% toàn buổi → gắn cờ `AntiCheat`, thông báo "Không đạt chuẩn xác thực" khi kết thúc.
- E2: Camera bị mất kết nối giữa chừng → tạm dừng, chờ user kết nối lại hoặc chuyển Phi AI.
- E3: 0 rep hợp lệ sau khi hết thời gian set → `WorkoutSetLog` ghi nhận 0 rep, không tính volume.

**Postcondition**: `WorkoutSetLog` được thêm vào `WorkoutSession`.  

**Domain Events**: —

---

### UC-03.3 LogSet — Nhánh Phi AI

| | |
|---|---|
| **Actor** | User |
| **Precondition** | `WorkoutSession` đang `InProgress`. |

**Main Flow**
1. System hiển thị video/hướng dẫn bài tập và bắt đầu timer đếm ngược theo set.
2. User thực hiện, tự nhập số rep và mức tạ khi xong.
3. System tạo `WorkoutSetLog` (rep, tạ, FormScore = N/A, RPE do user nhập).

**Error / Edge Cases**
- E1: User nhập tạ = 0 và rep = 0 → cảnh báo, không tạo log.
- E2: RPE không được nhập → mặc định N/A.

**Postcondition**: `WorkoutSetLog` được thêm vào `WorkoutSession`. FormScore = N/A, không tính vào điểm kỹ thuật buổi.  

**Domain Events**: —

---

### UC-03.4 CompleteWorkoutSession

| | |
|---|---|
| **Actor** | User · System |
| **Precondition** | `WorkoutSession` đang `InProgress`. User đã hoàn thành tất cả set trong giáo án hoặc chủ động kết thúc. |

**Main Flow**
1. User nhấn kết thúc buổi tập.
2. System gọi `TrainingLoadGuard`: so sánh tổng volume buổi này với trung bình 5 buổi gần nhất cùng nhóm cơ.
   - Nếu vượt 250% → yêu cầu user xác nhận trước khi lưu, chèn ≥ 1 ngày nghỉ vào `WeeklySchedule`.
3. System tính `SessionSummary` (tổng set, volume, FormScore trung bình).
4. System cập nhật `WorkoutSession` sang `Completed`, phát `WorkoutSessionCompleted`.
5. System hiển thị Post-session Report.

**Alternative Flow**
- A1: Buổi tập vượt 240 phút không tương tác → System tự đóng, ghi nhãn `AnomalousSession`, loại khỏi tính Overload.
- A2: Buổi tập vượt 90 phút (người mới) hoặc 180 phút (người cũ) → cảnh báo, user có thể tiếp tục hoặc kết thúc.

**Error / Edge Cases**
- E1: Không có `WorkoutSetLog` nào → không lưu, hỏi user có muốn huỷ buổi tập không.
- E2: `TrainingLoadGuard` từ chối và user không xác nhận → giữ session `InProgress`, không lưu.

**Postcondition**: `WorkoutSession` = `Completed`. `SessionSummary` sẵn sàng để `Coaching Context` xử lý adaptive.  
> *`WorkoutService.CompleteSession()` gọi `TrainingLoadGuard`, `WorkoutSessionRepository.Save()`, publish `WorkoutSessionCompleted`.*

**Domain Events**: `WorkoutSessionCompleted` | `WorkoutSessionAborted`

---

### UC-03.5 RecordPersonalRecord

| | |
|---|---|
| **Actor** | System |
| **Precondition** | `WorkoutSessionCompleted` event được nhận. |

**Main Flow**
1. System tính 1RM theo Epley Formula cho từng bài tập trong session.
2. System so sánh với `PersonalRecord` hiện tại.
3. Nếu vượt → cập nhật `PersonalRecord`, phát `NewPersonalRecordAchieved`.
4. App hiển thị thông báo vinh danh PR.

**Error / Edge Cases**
- E1: FormScore = N/A (bài Phi AI) → vẫn tính 1RM nhưng gắn nhãn `Unverified`.
- E2: Session là `AnomalousSession` → không tính 1RM.

**Postcondition**: `PersonalRecord` được cập nhật (nếu có PR mới).  
> *Xử lý bất đồng bộ qua Eventual Consistency, không chặn UC-03.4.*

**Domain Events**: `NewPersonalRecordAchieved`

---

## UC-04 Adaptive Review Cycle

---

### UC-04.1 EvaluateEndOfCycleCompletionRate

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Lộ trình 4 tuần vừa kết thúc. |

**Main Flow**
1. System tính `CompletionRate` (CR) = số buổi hoàn thành / tổng số buổi đã lên lịch (loại trừ `AnomalousSession`).
2. System áp dụng quy tắc BR-AC-04:
   - CR < 40%: Hỏi lý do, chờ phản hồi, đề xuất giảm số buổi/tuần và rút ngắn thời lượng.
   - 40% ≤ CR < 70%: Giảm tải lượng 10–15%, chèn xen kẽ buổi Express 30 phút. Tự sinh lộ trình mới.
   - 70% ≤ CR < 90%: Giữ cấu trúc, tăng Progressive Overload ≤ 10%.  Tự sinh lộ trình mới.
   - CR ≥ 90%: Đề xuất tăng cường độ hoặc thêm 1 buổi/tuần (không vượt BR-AC-01), gắn badge "Xuất sắc".
3. System tạo `WorkoutRoadmap` mới, phát `RoadmapInitiated`.

**Error / Edge Cases**
- E1: CR < 40% và user không phản hồi sau 48h → tự áp dụng phương án giảm tải mặc định.

**Postcondition**: Lộ trình mới được khởi tạo.  
> *`AdaptiveCoachEngine.EvaluateEndOfCycle()` đọc `WorkoutRoadmapRepository` và `WorkoutSessionRepository`.*

**Domain Events**: `RoadmapAdjusted` · `RoadmapInitiated`

---

### UC-04.2 DetectSignalB1 — Không hoạt động

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | User không có `WorkoutSession` nào trong 7 ngày liên tiếp. |

**Main Flow**
1. `AdaptiveCoachEngine` phát hiện Signal B1.
2. System gửi tin nhắn check-in theo phong cách `CoachPersonality`.
3. User phản hồi, chọn phương án:
   - (a) Tiếp tục từ buổi bỏ gần nhất.
   - (b) Đặt lại lịch tuần này.
   - (c) Tạm dừng lộ trình (Pause).
4. System thực thi phương án đã chọn.

**Alternative Flow**
- A1: Chọn Pause → `WorkoutRoadmap` chuyển sang `Paused`. Tối đa 4 tuần. Sau 4 tuần tự chuyển lại `Active` và hỏi lại user.

**Error / Edge Cases**
- E1: User không phản hồi trong 24h → không tự thay đổi lịch, gửi nhắc lại sau 24h.

**Postcondition**: Lịch tập được cập nhật theo lựa chọn của user (hoặc giữ nguyên nếu không phản hồi).

**Domain Events**: `RoadmapPaused` | `WeeklyScheduleGenerated`

---

### UC-04.3 DetectSignalB2 — Lịch không tương thích

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | User bỏ tập cùng 1 ngày trong tuần ≥ 3 lần liên tiếp. |

**Main Flow**
1. `AdaptiveCoachEngine` phát hiện Signal B2.
2. System đề xuất dời slot ngày đó sang ngày khác còn trống trong tuần.
3. Nếu user đồng ý → cập nhật `WeeklySchedule`, phát `ScheduleDayRescheduled`.
4. Nếu user từ chối → giữ nguyên, không hỏi lại về vấn đề này.

**Error / Edge Cases**
- E1: Không còn ngày trống trong tuần (đã tối đa 6 buổi) → chỉ thông báo, không đề xuất đổi.

**Domain Events**: `ScheduleDayRescheduled`

---

### UC-04.4 DetectSignalB3 — Overtraining

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | User tập ≥ 2 buổi/ngày hoặc RPE trung bình ≥ 8.5 liên tục ≥ 5 buổi. |

**Main Flow**
1. `AdaptiveCoachEngine` phát hiện Signal B3.
2. System cảnh báo nguy cơ quá tải, đề xuất Active Recovery.
3. System bắt buộc chèn 1 ngày nghỉ vào `WeeklySchedule` kế tiếp.

**Error / Edge Cases**
- E1: User từ chối ngày nghỉ → System vẫn chèn (bắt buộc, không cho bypass).

**Domain Events**: `WeeklyScheduleGenerated`

---

### UC-04.5 DetectSignalB4 — Plateau

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | 1RM và FormScore trung bình không tăng trong 3 tuần liên tiếp có CR ≥ 70%. |

**Main Flow**
1. `AdaptiveCoachEngine` phát hiện Signal B4.
2. System đề xuất 3 phương án:
   - (a) Deload Week: giảm 40% tải lượng 1 tuần.
   - (b) Đổi biến thể bài tập tương đương.
   - (c) Tăng số set, giữ nguyên tạ.
3. User chọn phương án. System cập nhật `DailyWorkoutPlan` kế tiếp.

**Error / Edge Cases**
- E1: User không chọn trong 48h → tự áp dụng Deload Week (an toàn nhất).

**Domain Events**: `RoadmapAdjusted`

---

## UC-05 Nutrition

---

### UC-05.1 GenerateDailyNutritionPlan

| | |
|---|---|
| **Actor** | System (AI Nutrition) |
| **Precondition** | `UserProfileCompleted`. Chưa có `NutritionPlan` cho ngày hôm nay. |

**Main Flow**
1. System đọc `BiologicalMetrics` (cân nặng, mục tiêu, mức vận động từ `WorkoutSession` hôm nay).
2. System tính TDEE theo công thức Mifflin-St Jeor, tính `CalorieAllocation` (Protein/Carb/Fat).
3. System kiểm tra `LockoutRegistry` của `MealHistory` để lọc nguyên liệu bị khóa.
4. System sinh `DailyMealOption` cho 3 bữa chính + 1 bữa phụ theo `BudgetTier` user đã chọn.
5. System phát `NutritionPlanGenerated`.

**Alternative Flow**
- A1: Ngày tập nặng (`WorkoutSessionCompleted` → volume cao) → tăng calo target ~10%.
- A2: Ngày nghỉ → giảm calo target ~10%.

**Error / Edge Cases**
- E1: `CalorieAllocation.target` < 1200 kcal sau tính toán → buộc giữ nguyên 1200 kcal (BR-NU-01).
- E2: Tất cả nguyên liệu protein bị `LockoutRegistry` khóa → System tự giải phóng nguyên liệu ít bị khóa nhất (unlock sớm nhất) để đảm bảo có thực đơn.
- E3: `BiologicalMetrics` chưa cập nhật cân nặng > 7 ngày → dùng giá trị cuối cùng và hiển thị cảnh báo.

**Postcondition**: `NutritionPlan` với `DailyMealOption` đầy đủ sẵn sàng hiển thị.  
> *`NutritionService.GenerateDailyPlan()` gọi `NutritionPlanRepository.Save()` và `MealHistoryRepository.GetLockouts()`.*

**Domain Events**: `NutritionPlanGenerated`

---

### UC-05.2 LogMeal

| | |
|---|---|
| **Actor** | User |
| **Precondition** | User đã đăng nhập. |

**Main Flow**
1. User tìm kiếm món ăn theo tên hoặc quét mã vạch.
2. System trả về `FoodItem` phù hợp từ Catalog.
3. User xác nhận khẩu phần (gram).
4. System tạo `MealLog`, cập nhật `LockoutRegistry` (Protein 7 ngày, Carb 5 ngày, Chủ đề 3 ngày).
5. System phát `MealLogged` và `LockoutApplied`.

**Alternative Flow**
- A1: Không tìm thấy món trong Catalog → User nhập thủ công tên + calo/macro ước tính.

**Error / Edge Cases**
- E1: Quét mã vạch thất bại / sản phẩm không có trong database → fallback sang nhập thủ công.
- E2: User cố log cùng một món 2 lần trong ngày → cảnh báo trùng lặp, vẫn cho phép nếu user xác nhận.

**Postcondition**: `MealLog` được ghi, `LockoutRegistry` được cập nhật.

**Domain Events**: `MealLogged` · `LockoutApplied`
