# FITAI — Bounded Context

> Nguồn: [Đặc tả Yêu cầu Nghiệp vụ Cốt lõi BABOK](./NGHIEP_VU_COT_LOI_BABOK.md)

---

## 1. Tổng quan

| # | Context | Câu hỏi nghiệp vụ | Phân loại |
|---|---|---|---|
| 1 | User Profile | "Tôi là ai? Thể trạng ra sao?" | Supporting |
| 2 | Coaching & Planning | "Tôi nên tập gì? Khi nào điều chỉnh?" | Core |
| 3 | Workout Execution & Motion | "Tôi tập thế nào? Kết quả ra sao?" | Core |
| 4 | Nutrition | "Tôi ăn gì hôm nay?" | Core |
| 5 | Catalog | "Hệ thống có những bài tập/thực phẩm nào?" | Supporting |

> Notification và Auth là Shared Infrastructure Services, không phải Bounded Context nghiệp vụ.

---

## 2. Đặc Tả Từng Bounded Context

### 1. User Profile Context
- **Trách nhiệm**: Xác thực tài khoản, quản lý chỉ số cơ thể và lịch sử (cân nặng, % mỡ, số đo, ảnh tiến trình), quản lý chấn thương và khung giờ tập. [FR-UM-01, FR-UM-02, FR-PT-01]
- **Không trách nhiệm**: Không tính Fitness Score, không sinh lộ trình, không chạy timer buổi tập.
- **Aggregates**: `User`, `BodyMetricsHistory`
- **Quy tắc nghiệp vụ**:
  - BR-UM-01: Hồ sơ đạt ≥ 80% mới kích hoạt AI Coach và tạo lộ trình.
- **Context liên quan**:
  - Cung cấp `BiologicalMetrics`, `Injury` cho `Coaching` và `Nutrition`.
  - Lắng nghe Event `BodyMetricUpdated` từ `Workout Execution` để cập nhật lịch sử chỉ số.

---

### 2. Coaching & Planning Context
- **Trách nhiệm**: Lập lộ trình 4 tuần, lịch tuần, giáo án JIT chi tiết; thực thi thích ứng (Signal B1–B4, CR cuối chu kỳ); quản lý phong cách Coach và nhắc lịch. [FR-AC-01 → FR-AC-07, FR-UM-04]
- **Không trách nhiệm**: Không ghi nhận thực tế buổi tập, không đếm rep, không tính 1RM.
- **Aggregates**: `WorkoutRoadmap`, `WeeklySchedule`, `DailyWorkoutPlan`
- **Domain Services**: `AdaptiveCoachEngine`, `OverloadValidator`
- **Quy tắc nghiệp vụ**:
  - BR-AC-01: Tối đa 6 buổi/tuần, ≥ 1 ngày nghỉ.
  - BR-AC-02: Progressive Overload ≤ 10% volume/tuần.
  - BR-AC-03: Buổi bỏ tập = "Bỏ qua", không tự dồn bù.
  - BR-AC-04: Quy tắc CR cuối chu kỳ (4 mức: <40%, 40–70%, 70–90%, ≥90%).
  - BR-AC-05: Signal B1 — Không hoạt động 7 ngày.
  - BR-AC-06: Signal B2 — Lịch không tương thích.
  - BR-AC-07: Signal B3 — Overtraining (≥ 2 buổi/ngày hoặc RPE ≥ 8.5 liên tục ≥ 5 buổi).
  - BR-AC-08: Signal B4 — Plateau (1RM + Form không tăng 3 tuần liên tiếp với CR ≥ 70%).
- **Context liên quan**:
  - Đọc `BiologicalMetrics`, `Injury` từ `User Profile`.
  - Đọc bài tập từ `Catalog`.
  - Lắng nghe `WorkoutSessionCompleted` từ `Workout Execution`.
  - Gọi Shared Infrastructure để gửi Push Notification.

---

### 3. Workout Execution & Motion Context
- **Trách nhiệm**: Thực thi buổi tập (timer, nhạc, video), đếm rep/ROM/Form Score bằng AI Camera, ghi log (AI/Phi AI), quản lý cấu hình AI bài tập (PoseTemplate, RepCountingRules), lưu dữ liệu thô và đo PR (1RM). [FR-WL-01 → FR-WL-04, FR-CC-01 → FR-CC-05, FR-PT-02]
- **Không trách nhiệm**: Không sinh giáo án, không quản lý lịch sử chỉ số cơ thể, không chạy logic thích ứng lộ trình.
- **Aggregates**: `WorkoutSession`, `WorkoutPerformance`, `MotionSpecification`
- **Domain Services**: `TrainingLoadGuard`
- **Quy tắc nghiệp vụ**:
  - BR-CC-01: Rep hợp lệ khi ROM ≥ 70%.
  - BR-CC-02: Frame skeleton hợp lệ < 50% → đánh dấu "Không đạt chuẩn xác thực".
  - BR-WL-01: Cảnh báo 90'/180', tự đóng sau 240' không tương tác → Anomalous Session.
  - BR-WL-02: Tải lượng > 250% trung bình 5 buổi gần nhất → yêu cầu xác nhận (xử lý bởi `TrainingLoadGuard`).
  - BR-WL-03: Bài phi AI không ghi Form Score (N/A).
- **Context liên quan**:
  - Lấy `WorkoutPrescription` từ `Coaching`.
  - Đọc bài tập/video từ `Catalog`.
  - Phát `WorkoutSessionCompleted` cho `Coaching` và `BodyMetricUpdated` cho `User Profile`.

---

### 4. Nutrition Context
- **Trách nhiệm**: Tính calo/macro (Mifflin-St Jeor), gợi ý thực đơn ngày theo 3 mức ngân sách, chống lặp thực phẩm, tư vấn định lượng tự nấu/ăn ngoài và ghi nhật ký bữa ăn. [FR-NU-01 → FR-NU-04]
- **Không trách nhiệm**: Không quản lý danh mục thực phẩm gốc (Catalog quản lý).
- **Aggregates**: `NutritionPlan`, `MealHistory`
- **Quy tắc nghiệp vụ**:
  - BR-NU-01: Thực đơn tối thiểu 1,200 kcal/ngày.
  - BR-NU-02: Khóa protein 7 ngày, tinh bột 5 ngày, chủ đề món 3 ngày.
  - BR-NU-03: Luôn kèm đề xuất sản phẩm đối tác nếu có.
- **Context liên quan**:
  - Đọc `BiologicalMetrics` từ `User Profile` để tính TDEE.
  - Đọc thực phẩm chuẩn từ `Catalog`.

---

### 5. Catalog Context
- **Trách nhiệm**: Cung cấp danh mục bài tập và thực phẩm chuẩn cho Admin CRUD, quản lý vòng đời phê duyệt và cho các Context khác tham chiếu. [FR-SM-01, FR-SM-02, FR-SM-03]
- **Không trách nhiệm**: Không chứa cấu hình AI khớp (Workout & Motion quản lý), không gợi ý giáo án hay thực đơn.
- **Aggregates**: `Exercise`, `FoodItem`
- **Quy tắc nghiệp vụ**:
  - Bài tập/thực phẩm mới phải qua trạng thái `PendingApproval` → Admin duyệt → `Active`.
- **Context liên quan**:
  - Cung cấp danh mục tham chiếu cho tất cả các Context khác.

---

## 3. Context Map

```mermaid
graph TD
    UP["User Profile"]
    CP["Coaching & Planning"]
    WEM["Workout Execution & Motion"]
    NU["Nutrition"]
    CA["Catalog"]

    UP -- "BiologicalMetrics, Injury" --> CP
    UP -- "BiologicalMetrics" --> NU

    CA -- "Exercise (ID ref)" --> CP
    CA -- "Exercise/Video (ID ref)" --> WEM
    CA -- "FoodItem (ID ref)" --> NU

    CP -- "WorkoutPrescription" --> WEM

    WEM -- "WorkoutSessionCompleted" --> CP
    WEM -- "BodyMetricUpdated" --> UP

    style CA fill:#f0f0f0,stroke:#999
    style CP fill:#e6f3ff,stroke:#4a90d9
    style WEM fill:#e6ffe6,stroke:#4a9d4a
    style NU fill:#fff3e6,stroke:#d9904a
    style UP fill:#ffe6e6,stroke:#d94a4a
```
