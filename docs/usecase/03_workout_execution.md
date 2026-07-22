# UC-03 Workout Execution

> Nguồn: [BRD](../NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](../02_bounded_context.md) · [Tactical Design](../03_ddd_tactical_design.md)

**Actor**: `User` (người tập) · `System` (AI Coach / AI Camera / AI Nutrition)  
**Format mỗi Use Case**: Precondition → Main Flow → Alternative Flow → Error / Edge Cases → Postcondition → Domain Events

---

### UC-03.1 StartWorkoutSession

| | |
|---|---|
| **Actor** | User |
| **Precondition** | `DailyWorkoutPlan` đã tồn tại. Không có `WorkoutSession` nào đang `InProgress` cho User. |

**Main Flow**
1. User check-in, chọn playlist âm nhạc (phát chạy ngầm xuyên suốt session).
2. System tạo `WorkoutSession` với trạng thái `InProgress`, khởi động `SessionTimer`.
3. System phát `WorkoutSessionStarted`.
4. **[Bước chuyển tiếp]** System kiểm tra giáo án:
   - Nếu giáo án có bài khởi động (`Warm-up`), hệ thống chuyển sang giao diện Khởi động (chạy qua UC-03.2 hoặc UC-03.3 tùy cấu hình bài tập) kèm nút **Skip Warm-up**.
   - Nếu không có Warm-up hoặc User nhấn Skip, chuyển sang bài tập chính đầu tiên.

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
3. System tracking 17 điểm khớp theo thời gian thực. Nhạc nền phát chạy ngầm.
4. User có thể chọn **Xem/Nghe hướng dẫn kỹ thuật** (on-demand):
   - **Xem**: Bật/tắt overlay video demo kỹ thuật ở góc màn hình (mặc định thu gọn).
   - **Nghe**: Bật/tắt giọng nói của AI Coach hướng dẫn động tác.
5. Mỗi rep: Nếu ROM% ≥ 70% → `repCount++`, tính `FormScore`. Nếu ROM% < 70% → rep không hợp lệ.
6. Nếu phát hiện lỗi tư thế hoặc khi phát audio hướng dẫn → Audio Ducking (giảm 70% âm lượng nhạc nền) + phát cảnh báo/hướng dẫn giọng nói (độ trễ < 500ms).
7. User xác nhận kết quả set (hệ thống tự động điền rep, tạ, FormScore). System tạo `WorkoutSetLog` (rep, tạ, FormScore trung bình, RPE).
8. System bắt đầu đếm ngược nghỉ.

**Alternative Flow**
- A1: User tự sửa kết quả set trước khi xác nhận.
- A2: Ánh sáng không đủ / bài nằm sàn → System tự động chuyển sang nhánh Phi AI (UC-03.3).

**Error / Edge Cases**
- E1: Tỷ lệ frame skeleton hợp lệ < 50% toàn buổi → gắn cờ `AntiCheat`, thông báo "Không đạt chuẩn xác thực" khi kết thúc. (Chỉ áp dụng cho bài tập chính có camera, không áp dụng cho khởi động/dãn cơ).
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
1. System bắt đầu timer đếm ngược theo set. Nhạc nền phát chạy ngầm.
2. User có thể chọn **Xem/Nghe hướng dẫn kỹ thuật** (on-demand):
   - **Xem**: Màn hình mặc định hiển thị video hướng dẫn nhưng cho phép ẩn/thu nhỏ để tập trung vào timer và nhạc nền.
   - **Nghe**: Cho phép bật/tắt thuyết minh giọng nói hướng dẫn kỹ thuật (có hỗ trợ Audio Ducking giảm nhạc nền khi phát tiếng).
3. User thực hiện, tự nhập số rep và mức tạ khi xong.
4. System tạo `WorkoutSetLog` (rep, tạ, FormScore = N/A, RPE do user nhập).

**Error / Edge Cases**
- E1: User nhập tạ = 0 và rep = 0 → cảnh báo, không tạo log (trừ các bài bodyweight/khởi động/dãn cơ không dùng tạ).
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
2. **[Bước chuyển tiếp]** System kiểm tra giáo án:
   - Nếu giáo án có bài dãn cơ (`Cooldown`), hệ thống hiển thị giao diện dãn cơ (chạy qua UC-03.2 hoặc UC-03.3) kèm nút **Skip Cooldown**.
   - Nếu không có Cooldown hoặc User nhấn Skip, tiếp tục luồng hoàn thành.
3. System gọi `TrainingLoadGuard`: so sánh tổng volume buổi này với trung bình 5 buổi gần nhất cùng nhóm cơ.
   - Nếu vượt 250% → yêu cầu user xác nhận trước khi lưu, chèn ≥ 1 ngày nghỉ vào `WeeklySchedule`.
4. System tính `SessionSummary` (tổng set, volume, FormScore trung bình).
5. System cập nhật `WorkoutSession` sang `Completed`, phát `WorkoutSessionCompleted`.
6. System hiển thị Post-session Report.

**Alternative Flow**
- A1: Buổi tập vượt 240 phút không tương tác → System tự đóng, ghi nhãn `AnomalousSession`, loại khỏi tính Overload.
- A2: Buổi tập vượt 90 phút (người mới) hoặc 180 phút (người cũ) → cảnh báo, user có thể tiếp tục hoặc kết thúc.
- A3: User cập nhật cân nặng hiện tại của mình lúc kết thúc buổi tập → System ghi nhận chỉ số cơ thể mới, phát event `BodyMetricUpdated`.

**Error / Edge Cases**
- E1: Không có `WorkoutSetLog` nào → không lưu, hỏi user có muốn huỷ buổi tập không.
- E2: `TrainingLoadGuard` từ chối và user không xác nhận → giữ session `InProgress`, không lưu.

**Postcondition**: `WorkoutSession` = `Completed`. `SessionSummary` sẵn sàng để `Coaching Context` xử lý adaptive.  
> *`WorkoutService.CompleteSession()` gọi `TrainingLoadGuard`, `WorkoutSessionRepository.Save()`, publish `WorkoutSessionCompleted`.*

**Domain Events**: `WorkoutSessionCompleted` | `WorkoutSessionAborted` | `BodyMetricUpdated`

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

### UC-03.6 SelectExtraWorkouts (Tập thêm bài bổ trợ - FR-AC-08)

| | |
|---|---|
| **Actor** | User · System (Go Backend API) |
| **Precondition** | `WorkoutSession` vừa đạt trạng thái `Completed` (bao gồm cả phần Cooldown của FR-AC-07). User tại màn hình Post-session Summary. |

**Main Flow**
1. User nhấn nút **"Tập thêm bài bổ trợ (Extra Workouts)"**.
2. Client gọi API `POST /api/v1/users/{user_id}/daily-workout-plans/today/extra-exercises` gửi kèm `session_id`.
3. System (Backend Go Engine) thực hiện deduplication: loại bỏ 100% các bài tập đã hoàn thành trong buổi chính.
4. System loại bỏ bài compound nặng và bài thuộc danh sách chấn thương active.
5. System trả về 3 danh mục bài tập thêm an toàn:
   - `Accessory/Core`: Bài tập cơ phụ (bụng, cẳng tay, bắp chân, tay sau).
   - `Cardio LISS`: Cardio thả lỏng 10-15 phút.
   - `Deep Mobility`: Giãn cơ dẻo khớp/Foam Rolling chuyên sâu.
6. User chọn bài tập và bắt đầu tập thêm (phản hồi API < 100ms, Go thuần không gọi AI Agent).

**Error / Edge Cases**
- E1: Không còn bài tập an toàn khả thi trong DB → hiển thị thông báo "Bạn đã hoàn thành đủ volume hôm nay!".

**Postcondition**: Các set tập thêm được ghi nhận nối tiếp vào `WorkoutSessionCompleted`.

**Domain Events**: —

---

### UC-03.7 SubmitUserExerciseFeedback (Phản hồi bài tập - FR-AC-09)

| | |
|---|---|
| **Actor** | User · System (AI Coach / Go Backend) |
| **Precondition** | `WorkoutSession` đang `InProgress` hoặc ở màn hình Summary. |

**Main Flow**
1. User gửi phản hồi về bài tập qua giao diện:
   - **Phản hồi "Bài quá dễ"**: System kích hoạt Fast-Track (`BR-AC-02`), áp mức tăng 15-30% tạ/volume cho bài này ở các set/buổi kế tiếp.
   - **Phản hồi "Chán bài tập"**: System lưu log `exercise_feedback: BORING` vào Prompt Context của User Profile.
2. Với phản hồi `BORING`, khi sinh giáo án buổi sau: AI Agent tự nhận biết từ Prompt Context, chọn bài thay thế cùng nhóm cơ và tạo lời giải thích cá nhân hóa.

**Postcondition**: Feedback được lưu trữ; Fast-Track được kích hoạt hoặc Prompt Context được cập nhật cho AI Agent.

**Domain Events**: `UserExerciseFeedbackSubmitted`
