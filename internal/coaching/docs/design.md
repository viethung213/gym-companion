# Coaching & Planning Context — Technical Design Document

## 1. Luồng Vận hành Tổng quan (Core Operating Architecture)

Cả 4 luồng nghiệp vụ chính của phân hệ Coaching đều vận hành theo nguyên tắc tự động hóa trơn trượt:

```mermaid
flowchart LR
    A[Start Request / Event] --> B[Build Context]
    B --> C[AI Coach Agent]
    C -->|Thiếu thông tin| D[Hỏi thêm Question]
    D --> E[Nhận Answer]
    E --> C
    C --> F[Guardrail Engine]
    F -->|Vi phạm| C
    F -->|Đạt An toàn| H[Lưu DB PostgreSQL]
    H --> I[SAVED & Event Outbox]
```

### 🟢 Core Flow 1 — Khởi tạo Lộ trình 4 tuần (`Initiate Roadmap`)
- **Kích hoạt khi**: Học viên hoàn thành Onboarding / cập nhật hồ sơ đủ điều kiện.
- **Context Backend build**: Hồ sơ, mục tiêu (`primary_goal`), thiết bị (`available_equipment`), lịch rảnh (`available_slots`), chấn thương active.
- **Agent suy luận**: Tạo Lộ trình 4 tuần (`Roadmap`), các `WeekPlan` (phân bổ pha: Accumulation, Overload, Peak, Deload), `DayPlan` và **sinh sẵn toàn bộ 28 giáo án `SessionPlan`** ở trạng thái `PENDING`.
- **Guardrail**: Kiểm tra trần 6 buổi/tuần, ngày nghỉ bắt buộc, kiểm soát chấn thương.
- **Lưu DB (`SAVED`)**: Ghi toàn bộ cây đối tượng 4 tầng vào PostgreSQL và phát event `RoadmapInitiated`.

---

### 🔵 Core Flow 2 — Thực thi Buổi tập (`Execute Session Plan`)
- **Kích hoạt khi**: Học viên chuẩn bị tập hoặc hoàn thành buổi tập hôm nay.
- **Quy trình thực thi**:
  1. **Xem giáo án**: App đọc `SessionPlan` đang `PENDING` hôm nay có sẵn trong DB để hiển thị bài tập, set, rep, mức tạ gợi ý và target RPE. *(Không cần gọi AI Agent)*.
  2. **Tập luyện**: Học viên tập dưới camera/app. Khi hoàn thành, `Workout Execution Context` phát event `WorkoutSessionCompleted`.
  3. **Nghiệm thu & Cập nhật**: `Coaching Context` tính toán $SCR$ và $\Delta RPE$, cập nhật trạng thái `SessionPlan.status` từ `PENDING` $\rightarrow$ **`COMPLETED`**, và phát event `SessionPlanExecuted`.

---

### ⚪ Core Flow 2.1 — Mở App ngoài giờ tập (`Dashboard / Non-Workout Hours Flow`)
- **Kích hoạt khi**: Học viên mở App vào các thời điểm ngoài khung giờ tập (`slot_time`).
- **Hành vi hệ thống**: **KHÔNG gọi AI Agent** để tiết kiệm tài nguyên.
- **Màn hình Dashboard hiển thị (đọc trực tiếp từ DB)**:
  1. **Lộ trình hiện tại**: Hiển thị Tiến độ Lộ trình 4 tuần (Ví dụ: `Tuần 2 / 4 - Pha Overload`).
  2. **Buổi tập sắp tới**: Hiển thị tóm tắt `SessionPlan` tiếp theo (`status = PENDING`) kèm đồng hồ đếm ngược / nhắc nhở khung giờ tập (`slot_time`).
  3. **Tiến độ Tuần**: Tỷ lệ hoàn thành buổi tập trong tuần (Ví dụ: `3/4 buổi completed`).
  4. **Các nút thao tác chủ động**: *"Xem chi tiết Lịch 4 tuần"*, *"Yêu cầu đổi lịch tập"* (`RegenerateSchedule`), *"Khai báo chấn thương mới"*.

---

### 🟡 Core Flow 3 — Điều chỉnh Lịch tập (`Regenerate Schedule`)
- **Kích hoạt khi**: Học viên chủ động bấm nút yêu cầu sửa lịch (VD: giảm số buổi/tuần, đổi ngày cố định, đổi mục tiêu, yêu cầu Deload).
- **Context Backend build**: `Roadmap` active, `WeekPlan` hiện tại, tiến độ đã tập, yêu cầu của user.
- **Sau khi qua Guardrail (`SAVED`)**:
  - Giữ nguyên các buổi đã tập xong (`status = COMPLETED`).
  - Re-generate lại toàn bộ các `WeekPlan` và `SessionPlan` chưa tập trong tương lai (`status = PENDING`).

---

### 🔴 Core Flow 4 — Phản ứng theo Tín hiệu & Sự kiện (`React to Signals & External Events`)
- **Kích hoạt khi**: Backend nhận Event từ context khác hoặc phát hiện các Tín hiệu thích ứng:

| Signal / Event | Nguồn Event (Context) | Quy tắc Nghiệp vụ (SSOT) | Tóm tắt Xử lý |
|---|---|---|---|
| `ProfileUpdated` | `Profile Context` | [FR-AC-06](../../../docs/NGHIEP_VU_COT_LOI_BABOK.md#L79) | Re-generate giáo án tất cả các buổi chưa tập trong 4 tuần (`scheduled_date >= today` & `PENDING`) phù hợp với dụng cụ/lịch rảnh mới |
| `InjuryReported` | `Profile Context` | [FR-AC-03](../../../docs/NGHIEP_VU_COT_LOI_BABOK.md#L76) | Khóa bài vi phạm tác động vào vùng chấn thương, Agent tìm bài thay thế an toàn |
| `InjuryRecovered` (Post-Injury) | `Profile Context` | [BR-AC-09](../../../docs/NGHIEP_VU_COT_LOI_BABOK.md#L140) | Áp dụng cơ chế bảo vệ 3 buổi đầu ($\le 50\%$ tạ PR, Machine/Cable, $RPE \le 7$) |
| B1 — Bỏ tập cấp tính / Mất tích | `Coaching Context` | [BR-AC-05](../../../docs/NGHIEP_VU_COT_LOI_BABOK.md#L136) | Tập tiếp từ buổi gần nhất hoặc đặt lại lịch tuần này |
| B2 — Schedule mismatch | `Coaching Context` | [BR-AC-06](../../../docs/NGHIEP_VU_COT_LOI_BABOK.md#L137) | Dời slot sang ngày rảnh khác |
| B3 — Tập ngoài lịch (Unscheduled Workout) | `Workout Execution` | [BR-AC-07](../../../docs/NGHIEP_VU_COT_LOI_BABOK.md#L138) | Check-in phân loại 3 nguyên nhân (quá nhẹ, thừa thời gian, quá sức/nguy hiểm) |
| B4 — Chai lỳ cơ (Plateau) | `Workout Execution` | [BR-AC-08](../../../docs/NGHIEP_VU_COT_LOI_BABOK.md#L139) | Đổi biến thể bài tập tương đương, điều chỉnh rep/set, hoặc đổi thứ tự bài |
| Trigger A — Đánh giá định kỳ | `Coaching Context` | [BR-AC-04](../../../docs/NGHIEP_VU_COT_LOI_BABOK.md#L135) | Tính $SCR$ và $\Delta RPE$, tự động điều chỉnh tạ ($+2.5\% \to +5\%$), Deload (giảm 30% volume & 10% tạ), hoặc giảm 1 buổi/tuần ($SCR < 50\%$) |

---

## 3. Hexagonal Architecture (Ports & Adapters) Structure

Module `internal/coaching/` được đóng gói độc lập theo mô hình Hexagonal Architecture:

```text
internal/coaching/
├── domain/                         # Core Domain Logic (No external imports, No ORM tags)
│   ├── roadmap/                    # Aggregate Root: Roadmap, WeekPlan, DayPlan, SessionPlan
│   ├── service/                    # Domain Services: AdaptiveCoachEngine, OverloadValidator
│   └── event/                      # Domain Events: RoadmapInitiated, RoadmapAdjusted
├── application/                    # Application Layer (Use Cases, Commands, Queries)
│   ├── command/                    # InitiateRoadmapHandler, RegenerateScheduleHandler
│   ├── query/                      # GetActiveRoadmapHandler, GetSessionPlanHandler
│   ├── contextbuilder/             # ContextBuilder (Aggregate state from Profile & Execution)
│   └── port/                       # Primary & Secondary Ports (Interfaces)
├── infrastructure/                 # Infrastructure Layer (Adapters)
│   ├── persistence/                # PostgreSQL Repository & GORM/SQL Mappers
│   ├── ai/                         # AI Coach Agent Adapter (LLM Client & Tools)
│   └── guardrail/                  # Guardrail Enforcement Pipeline
└── transport/                      # Transport Layer
    └── grpc/                       # gRPC Server implementation (CoachingServiceServer)
```

---

## 4. Danh mục Agent Tools Thiết yếu

| Tên Tool | Chức năng chính |
|---|---|
| `scale_volume_intensity` | Điều chỉnh % tạ, rep, set hàng loạt trên các buổi `PENDING` |
| `shift_session_slots` | Dời lịch các buổi `PENDING` sang ngày rảnh khác |
| `search_eligible_exercises` | Tìm bài tập hợp lệ theo thiết bị & nhóm cơ |
| `replace_injured_exercises` | Tìm bài tập thay thế an toàn né vùng chấn thương |
| `get_exercise_history` | Đọc lịch sử 1RM & tạ PR gần nhất |
