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
