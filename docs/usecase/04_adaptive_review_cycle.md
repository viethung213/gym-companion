# UC-04 Adaptive Review Cycle

> Nguồn: [BRD](../NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](../02_bounded_context.md) · [Tactical Design](../03_ddd_tactical_design.md)

**Actor**: `User` (người tập) · `System` (AI Coach / AI Camera / AI Nutrition)  
**Format mỗi Use Case**: Precondition → Main Flow → Alternative Flow → Error / Edge Cases → Postcondition → Domain Events

---

### UC-04.1 EvaluateEndOfCycleCompletionRate

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Lộ trình 4 tuần vừa kết thúc. |

**Main Flow**
1. System tính `CompletionRate` (CR) = số buổi hoàn thành / tổng số buổi đã lên lịch (loại trừ `AnomalousSession`).
2. System áp dụng quy tắc BR-AC-04:
   - CR < 40%: Hỏi lý do, chờ phản hồi, đề xuất giảm số buổi/tuần và rút ngắn thời lượng. Nếu user đồng ý → cấu hình lại; nếu từ chối → chuyển sang **A1**.
   - 40% ≤ CR < 70%: Giảm tải lượng 10–15%, chèn xen kẽ buổi Express 30 phút. Tự sinh lộ trình mới.
   - 70% ≤ CR < 90%: Giữ cấu trúc, tăng Progressive Overload ≤ 10%. Tự sinh lộ trình mới.
   - CR ≥ 90%: Đề xuất tăng cường độ hoặc thêm 1 buổi/tuần (không vượt BR-AC-01), gắn badge "Xuất sắc". Nếu user đồng ý → thêm buổi; nếu từ chối → chuyển sang **A1**.
3. System tạo `WorkoutRoadmap` mới, phát `RoadmapInitiated`.

**Alternative Flow**
- A1: User từ chối đề xuất thay đổi số buổi tập (khi CR < 40% hoặc CR ≥ 90%) → System giữ nguyên cấu trúc số buổi/tuần cũ của lộ trình, nhưng tự động điều chỉnh Progressive Overload (tăng/giảm mức tạ hoặc volume tạ gợi ý) tương ứng để đảm bảo tính an toàn và thích ứng.

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
3. If user đồng ý → cập nhật `WeeklySchedule`, phát `ScheduleDayRescheduled`.
4. Nếu user từ chối → giữ nguyên, không hỏi lại về vấn đề này.

**Error / Edge Cases**
- E1: Không còn ngày trống trong tuần (đã tối đa 6 buổi) → chỉ thông báo, không đề xuất đổi.

**Domain Events**: `ScheduleDayRescheduled`

---

### UC-04.4 DetectSignalB3 — Overtraining

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | User tập ≥ 2 buổi/ngày hoặc Session RPE (điểm đánh giá nỗ lực cả buổi tập do người dùng khai báo) trung bình ≥ 8.5 liên tục ≥ 5 buổi. |

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
