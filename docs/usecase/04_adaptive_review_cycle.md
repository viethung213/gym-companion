# UC-04 Adaptive Review Cycle

> Nguồn: [BRD](../NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](../02_bounded_context.md) · [Tactical Design](../03_ddd_tactical_design.md)

**Actor**: `User` (người tập) · `System` (AI Coach / AI Camera / AI Nutrition)  
**Format mỗi Use Case**: Precondition → Main Flow → Alternative Flow → Error / Edge Cases → Postcondition → Domain Events

---

### UC-04.1 EvaluateWeeklyAdaptiveCycle

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Hoàn thành 1 tuần tập (hoặc cuối chu kỳ 4 tuần). |

**Main Flow**
1. System tính Tỷ lệ hoàn thành Set ($SCR = \frac{\text{Số Set thực tế}}{\text{Số Set giao}} \times 100\%$) và Độ lệch mệt mỏi ($\Delta RPE = RPE_{\text{Thực tế}} - RPE_{\text{Target}}$).
2. System áp dụng quy tắc thích ứng định kỳ [BR-AC-04](../NGHIEP_VU_COT_LOI_BABOK.md#L135):
   - **Tăng tải lũy tiến** ($SCR \ge 80\%$ và $-1 \le \Delta RPE \le +1$): Tự động tăng mức tạ gợi ý $+2.5\% \rightarrow +5\%$ cho tuần kế tiếp.
   - **Quản lý mệt mỏi & Deload** ($RPE_{\text{Thực tế}} \ge 9.0$ liên tục 3 buổi hoặc $\Delta RPE \ge +2.0$): Tự động kích hoạt tuần Deload (giảm 30% volume & 10% tạ).
   - **Thích ứng linh hoạt** ($50\% \le SCR < 80\%$): Giữ nguyên mức tạ, điều chỉnh rep/set nếu cần, gợi ý tăng buổi tập/tuần nếu user đồng ý.
3. System cập nhật `prescription` bài tập của các `SessionPlan` tuần kế tiếp, phát `RoadmapAdjusted`.

**Alternative Flow**
- A1: User từ chối giảm số buổi tập khi $SCR < 50\%$ ➔ System giữ nguyên cấu trúc số buổi cũ, nhưng tự động chuyển sang giáo án Express 30 phút để hỗ trợ hoàn thành.

**Error / Edge Cases**
- E1: $SCR < 50\%$ và user không phản hồi sau 48h ➔ Tự áp dụng phương án chuyển sang giáo án Express 30 phút.

**Postcondition**: Giáo án tuần kế tiếp được cập nhật thích ứng theo $SCR$ và $\Delta RPE$.  
> *`AdaptiveCoachEngine.EvaluateWeeklyCycle()` đọc `RoadmapRepository` và `WorkoutSessionRepository`.*

**Domain Events**: `RoadmapAdjusted`

---

### UC-04.2 DetectSignalB1 — Bỏ tập cấp tính / Mất tích

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | User bỏ tập / hủy dở 3 buổi liên tiếp (hoặc không có `WorkoutSession` hoàn thành trong 7 ngày). |

**Main Flow**
1. `AdaptiveCoachEngine` phát hiện Signal B1.
2. System gửi tin nhắn check-in theo phong cách `CoachPersonality`.
3. User phản hồi, chọn phương án:
   - (a) Tiếp tục từ buổi bỏ gần nhất.
   - (b) Đặt lại lịch tuần này.
4. System thực thi phương án đã chọn.

**Error / Edge Cases**
- E1: User không phản hồi trong 24h → không tự thay đổi lịch, gửi nhắc lại sau 24h.

**Postcondition**: Lịch tập được cập nhật theo lựa chọn của user (hoặc giữ nguyên nếu không phản hồi).

**Domain Events**: `RoadmapAdjusted`

---

### UC-04.3 DetectSignalB2 — Kẹt lịch cố định

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | User bỏ tập cùng 1 ngày trong tuần ≥ 3 lần liên tiếp. |

**Main Flow**
1. `AdaptiveCoachEngine` phát hiện Signal B2.
2. System đề xuất dời slot ngày đó sang ngày khác còn trống trong tuần.
3. Nếu user đồng ý → Cập nhật phân bổ ngày tập trong `DayPlan`, phát `RoadmapAdjusted`.
4. Nếu user từ chối → giữ nguyên, không hỏi lại về vấn đề này.

**Error / Edge Cases**
- E1: Không còn ngày trống trong tuần (đã tối đa 6 buổi) → chỉ thông báo, không đề xuất đổi.

**Domain Events**: `RoadmapAdjusted`

---

### UC-04.4 AdHocWorkout — Tập ngoài lịch (Unscheduled Workout)

| | |
|---|---|
| **Actor** | User, Mobile App, Coaching Context |
| **Precondition** | User chọn tập ngoài lịch (tập vào ngày nghỉ hoặc tập buổi thứ 2+ trong ngày). |

**Main Flow**
1. User chọn bài tập ngoài lịch theo 1 trong 2 tùy chọn:
   - **Tự chọn bài tập**: Chọn danh sách bài tập trực tiếp từ Exercise Catalog.
   - **Nhấn nút "Tạo bài tập"**: Nhập thông tin tùy chọn (nhóm cơ, thời lượng, cường độ). Mobile App gọi trực tiếp API `SuggestAdHocSession` (function tư vấn đồng bộ) để nhận gợi ý giáo án JIT.
2. System tạo `SessionPlan` (`source = USER_ADHOC`, `status = PENDING`) và trả về `plan_id`.
3. User thực thi buổi tập như bình thường.

**Domain Events**: `SessionPlanExecuted`

---

### UC-04.5 DetectSignalB4 — Cơ bắp bị chai lỳ (Plateau)

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Sức mạnh ước tính (1RM) của bài tập chính không tăng trong 2 tuần liên tiếp (chỉ tính các tuần có $SCR \ge 80\%$). |

**Main Flow**
1. `AdaptiveCoachEngine` phát hiện Signal B4 khi kết thúc buổi tập cuối Tuần 2.
2. System gửi Push Notification / In-App Alert thông báo cho User.
3. Khi User tương tác qua gRPC Stream, System đề xuất 3 phương án phá Plateau:
   - (a) Đổi biến thể bài tập tương đương.
   - (b) Điều chỉnh dải Rep/Set (VD: chuyển từ 8–10 reps sang 4–6 reps heavy).
   - (c) Thay đổi thứ tự bài tập trong buổi tập.
4. User chọn phương án và bấm Confirm ➔ System cập nhật `prescription` trong `SessionPlan` của các tuần tới.

**Error / Edge Cases**
- E1: User không tương tác ➔ Tự động áp dụng đổi biến thể bài tập tương đương cho tuần tới.

**Postcondition**: Bài tập tuần tới được làm mới để phá vỡ giai đoạn đình trệ.

**Domain Events**: `RoadmapAdjusted`

---

### UC-04.6 AdaptPostInjuryRecovery — Thích ứng sau phục hồi chấn thương

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Một chấn thương được xác nhận phục hồi (`recovered`). |

**Main Flow**
1. System áp dụng cơ chế bảo vệ trong **3 buổi tập đầu tiên** liên quan đến nhóm cơ của khớp vừa phục hồi:
   - Giới hạn mức tạ gợi ý tối đa không vượt quá **50%** mức tạ PR trước chấn thương.
   - Ưu tiên gợi ý bài Bodyweight hoặc Machine/Cable (đường chuyển động cố định).
2. Với mỗi buổi tập bảo vệ hoàn thành:
   - Nếu đạt $RPE \le 7$ (với bài dùng AI Camera bổ sung thêm $Form\ Score \ge 80\%$) ➔ Ghi nhận 1 buổi bảo vệ thành công.
3. Sau khi hoàn thành đủ 3 buổi bảo vệ đạt chuẩn ➔ System mở lại cơ chế Progressive Overload bình thường.

**Error / Edge Cases**
- E1: Sau 3 buổi mà $RPE > 7$ hoặc $Form\ Score < 80\%$ ➔ Kéo dài giai đoạn bảo vệ cho đến khi đạt đủ điều kiện an toàn.

**Postcondition**: Người tập quay lại chu kỳ tăng tải bình thường sau khi phục hồi an toàn.

**Domain Events**: `RoadmapAdjusted`
