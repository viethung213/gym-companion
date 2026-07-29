# UC-02 Coaching & Planning

> Nguồn: [BRD](../NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](../02_bounded_context.md) · [Tactical Design](../03_ddd_tactical_design.md)

**Actor**: `User` (người tập) · `System` (AI Coach / AI Camera / AI Nutrition)  
**Format mỗi Use Case**: Precondition → Main Flow → Alternative Flow → Error / Edge Cases → Postcondition → Domain Events

---

### UC-02.1 InitiateRoadmap

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Event `UserProfileCompleted` được nhận. `ActiveCoachEnabled = true`. |

**Main Flow**
1. System đọc `biological_metrics`, `primary_goal`, `preferred_muscle_groups`, `available_equipment` và `available_slots` từ `User`.
2. System khởi tạo Baseline RPE 6–7 cho lộ trình làm quen ban đầu.
3. System tạo `Roadmap` 4 tuần (28 ngày) với `status = ACTIVE`.
4. System tạo 4 `WeekPlan` tương ứng 4 tuần, phân bổ theo 4 pha tiến trình chuẩn: Tuần 1 (Accumulation RPE 6–7), Tuần 2 (Overload RPE 7–8), Tuần 3 (Peak RPE 8–9), Tuần 4 (Deload RPE 5–6 giảm 30% volume & 10% tạ).
5. System khởi tạo `DayPlan` tương ứng các ngày có lịch tập trên lịch (lưu `scheduled_date`, `available_slots`). Các ngày không có lịch tập được ngầm hiểu là ngày nghỉ phục hồi (implicit rest day).
6. Với các ngày tập, System khởi tạo `SessionPlan` gắn theo từng khung giờ rảnh (`slot_time`), chứa `prescription` bài tập (warm-up, main, cool-down, set, rep, weight, target RPE) với `status = PENDING`.
7. System gọi `OverloadValidator` kiểm soát giới hạn điều chỉnh tải trọng ($\pm 30\%$, tối đa $+5\text{kg}$ bài free-weight nặng, `BR-AC-02`).
8. System lưu `Roadmap` (gồm 4 `WeekPlan` và các `DayPlan` ngày tập) và phát event `RoadmapInitiated`.

**Alternative Flow**
- A1: User có `Injury` active → System loại bỏ bài tập tác động vùng chấn thương khi sinh giáo án `prescription`.
- A2: `preferred_muscle_groups` rỗng → System tự động áp dụng `MuscleSplit` cân bằng chuẩn (Push/Pull/Legs hoặc Upper/Lower split).

**Error / Edge Cases**
- E1: `biological_metrics` không đủ dữ liệu → Hủy quy trình, yêu cầu hoàn thiện hồ sơ.
- E2: `OverloadValidator` từ chối bài tập (vượt trần 30%) → Hạ mức tạ về trần tối đa hợp lệ và tiếp tục.

**Postcondition**: `Roadmap` 4 tuần và các `SessionPlan` (chứa `prescription` NOT NULL) được khởi tạo sẵn sàng trong cơ sở dữ liệu.  
> *`CoachingService.InitiateRoadmap()` gọi `RoadmapRepository.Save()`.*

**Domain Events**: `RoadmapInitiated`

---

### UC-02.2 ExecuteSessionPlan

| | |
|---|---|
| **Actor** | System (AI Coach) · User |
| **Precondition** | `Roadmap` đang ở trạng thái `ACTIVE`. Đến khung giờ tập theo lịch (`scheduled_date = today`, `slot_time`). |

**Main Flow**
1. User mở ứng dụng trong khung giờ tập. System đọc trực tiếp `SessionPlan` của buổi tập hôm nay từ cơ sở dữ liệu.
2. System kiểm tra trạng thái các buổi tập trước đó. Nếu buổi trước chưa thực hiện ➔ Tự động đánh dấu buổi cũ là `SKIPPED` (`BR-AC-03`), các set bỏ lỡ tính $0$ vào $SCR$, giữ nguyên giáo án buổi hôm nay.
3. System hiển thị câu hỏi Check-in ngắn về tình trạng sức khỏe hôm nay (chấn thương mới, mệt mỏi/giờ ngủ).
4. User xác nhận giáo án `prescription` buổi tập hôm nay và bắt đầu tập (chuyển sang UC-03 Workout Execution).
5. Sau khi kết thúc buổi tập, System cập nhật `status = COMPLETED` cho `SessionPlan` buổi hôm nay.

**Alternative Flow**
- A1: Check-in phát hiện chấn thương mới hoặc mệt mỏi cấp tính ($\Delta RPE \ge +2.0$) ➔ System kích hoạt Re-generate `prescription` riêng cho `SessionPlan` hôm nay (giảm tải hoặc đổi bài phục hồi nhẹ).
- A2: Ngày hôm nay là ngày nghỉ theo lịch ➔ System thông báo "Hôm nay nghỉ phục hồi".

**Error / Edge Cases**
- E1: `SessionPlan` không tồn tại do lỗi dữ liệu ➔ System tự động khởi tạo JIT `SessionPlan` bổ sung cho khung giờ hôm nay.

**Postcondition**: Buổi tập hôm nay được thực thi thành công và `SessionPlan.status` cập nhật thành `COMPLETED`.  
> *`CoachingService.GetSessionPlan()` đọc trực tiếp từ `RoadmapRepository`.*

**Domain Events**: `SessionPlanExecuted`

---

### UC-02.3 RegenerateScheduleOnProfileUpdated

| | |
|---|---|
| **Actor** | System (AI Coach) |
| **Precondition** | Event `ProfileUpdated` hoặc tín hiệu điều chỉnh (`Signal B1–B4`, `Trigger A BR-AC-04`) được nhận. `Roadmap` đang ở trạng thái `ACTIVE`. |

**Main Flow**
1. System đọc thông tin snapshot mới từ `User` (`available_equipment`, `available_slots`, `preferred_muscle_groups`, `primary_goal`).
2. System xác định danh sách các buổi tập chưa thực thi (`scheduled_date >= today` và `status = PENDING`).
3. System kích hoạt Re-generation cho `prescription` của các `SessionPlan` chưa thực thi:
   - Cập nhật bài tập tương thích với `available_equipment` mới.
   - Điều chỉnh phân bổ ngày tập theo `available_slots` mới.
   - Điều chỉnh `MuscleSplit` theo `preferred_muscle_groups` và `primary_goal`.
4. System lưu thông tin cập nhật vào `Roadmap` và phát event `RoadmapAdjusted`.

**Postcondition**: Toàn bộ giáo án các buổi tập chưa thực thi trong 4 tuần được cập nhật.  
> *`CoachingService.RegenerateSchedule()` gọi `RoadmapRepository.Save()` và phát `RoadmapAdjusted`.*

**Domain Events**: `RoadmapAdjusted`
