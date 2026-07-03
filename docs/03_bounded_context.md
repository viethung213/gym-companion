# 3. Phân Định Ngữ Cảnh Bounded Context - FITAI

Tài liệu này xác định ranh giới mô hình (Bounded Contexts) trong hệ thống **FITAI**, phân chia nhiệm vụ rõ ràng để chuẩn bị cho việc thiết kế kiến trúc và cơ sở dữ liệu.

---

## 3.1 Bounded Contexts Phân Định

Dựa trên các quy trình nghiệp vụ và các miền con đã khám phá, FITAI được chia thành 4 Bounded Contexts chính:

```
┌────────────────────────────────────────────────────────────────────────┐
│                                 FITAI                                  │
│                                                                        │
│  ┌──────────────────────────────┐         ┌──────────────────────────┐ │
│  │ User Profile & Health        │         │ AI Coaching & Planning   │ │
│  │ Context                      │         │ Context                  │ │
│  │ - Đăng ký, Đăng nhập, Profile│         │ - Lịch tuần & Lộ trình 4w│ │
│  │ - Khai báo chấn thương       │         │ - Sinh bài tập JIT daily │ │
│  │ - Ràng buộc hoàn thành >=80% │         │ - Điều chỉnh Trigger A/B │ │
│  └──────────────────────────────┘         └──────────────────────────┘ │
│                 │                                      ▲               │
│                 ▼                                      │               │
│  ┌───────────────────────────────┐       ┌────────────────────────────┐│
│  │ Workout Execution             │       │ AI Nutrition               ││
│  │ Context                       │       │ Context                    ││
│  │ - Nhánh AI: tracking skeleton │       │ - Tính toán TDEE           ││
│  │ - Nhánh phi AI: Timer/H.dẫn   │       │ - Anti-Repetition          ││
│  │ - Giới hạn time & tải lượng   │       │ - Ăn ngoài & 3 mức giá     ││
│  └───────────────────────────────┘       └────────────────────────────┘│
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 3.2 Đặc Tả Từng Bounded Context

### 1. User Profile & Health Context (Bối cảnh Hồ sơ & Sức khỏe)
* **Phạm vi**: Đăng nhập, đăng ký, xác thực OTP, quản lý tài khoản người dùng, nhập chỉ số cơ thể ban đầu, theo dõi và lưu trữ lịch sử chỉ số cơ thể (cân nặng, tỷ lệ mỡ) theo dòng thời gian, quản lý danh sách vùng chấn thương và tiền sử bệnh lý.
* **Quy tắc nghiệp vụ chính**:
  * **BR-UM-01**: Yêu cầu hồ sơ đạt $\ge 80\%$ độ hoàn thiện mới cho phép kích hoạt AI Coach và tạo lộ trình tập đầu tiên.
* **Mô hình cốt lõi**: `User`, `HealthProfile`, `BodyMetricsHistory`, `InjuryList`, `DeviceRegistration`.

### 2. AI Coaching & Planning Context (Bối cảnh Huấn luyện & Lên kế hoạch)
* **Phạm vi**: Khởi tạo Lộ trình tổng quan 4 tuần và Lịch tuần (phân bổ cơ). Tự động sinh chi tiết giáo án hàng ngày dạng **Just-In-Time (JIT)** (chứa bài tập, set, rep, tạ gợi ý và khởi động/giãn cơ). Thực thi đánh giá điều chỉnh lộ trình thích ứng cuối chu kỳ 4 tuần (Trigger A - BR-AC-04) và liên tục theo dõi hành vi để điều chỉnh khẩn cấp giữa chu kỳ (Trigger B - Signals B1-B4 qua BR-AC-05 -> BR-AC-08). Quản lý phong cách huấn luyện viên.
* **Quy tắc nghiệp vụ chính**:
  * **BR-AC-01**: Tối đa **6 buổi tập/tuần** và bắt buộc phải có ít nhất **1 ngày nghỉ hoàn toàn**.
  * **BR-AC-02**: Tốc độ tăng tiến Progressive Overload không vượt quá **10% tổng volume** của tuần trước.
  * **BR-AC-03**: Giáo án các buổi bỏ tập đánh dấu là "Bỏ qua", không tự động dồn/bù nếu chưa có xác nhận từ người dùng.
* **Mô hình cốt lõi**: `WorkoutRoadmap`, `WeeklySchedule`, `JITWorkoutSessionSpecification`, `CoachPersonality`, `ProgressionRule`.

### 3. Workout Execution Context (Bối cảnh Thực thi Buổi tập)
* **Phạm vi**: Bắt đầu buổi tập, check-in, quản lý danh sách bài hát nền. Rẽ 2 nhánh:
  - **Nhánh AI**: tracking skeleton bằng camera, đo lường góc ROM%, phát hiện lỗi chuyển động và cảnh báo bằng hình ảnh/âm thanh (< 500ms), Audio Ducking khi có cảnh báo lỗi, đếm số rep đạt chuẩn, ước lượng cân nặng tạ thực tế, ghi chép Set tập tự động và cho phép sửa tay.
  - **Nhánh Phi AI**: Giao diện trình bấm giờ (timer) đếm ngược, phát nhạc nền, hiển thị video/hướng dẫn bài tập, người dùng tự tập luyện và ghi nhận kết quả set thủ công.
* **Quy tắc nghiệp vụ chính**:
  * **BR-CC-01**: Rep chỉ hợp lệ để đếm (khi tập AI) nếu ROM đạt tối thiểu **$\ge 70\%$** so với biên độ tiêu chuẩn.
  * **BR-CC-02**: Nếu tỷ lệ khung hình skeleton hợp lệ < 50% trong suốt buổi tập dưới camera, hệ thống đánh dấu buổi tập là "Không đạt chuẩn xác thực" (chống gian lận).
  * **BR-WL-01**: Giới hạn thời gian buổi tập: cảnh báo sau 90m (người mới) / 180m (người cũ). Đóng tự động sau 240m không tương tác (nhãn `Anomalous Session`, loại khỏi Overload).
  * **BR-WL-02**: Giới hạn tải lượng bất thường: volume buổi tập > 250% trung bình 5 buổi gần nhất có cùng nhóm cơ/mục tiêu -> Yêu cầu xác nhận và chèn ít nhất 1 ngày nghỉ cho nhóm cơ đó.
  * **BR-WL-03**: Ghi nhận bài tập phi AI: điểm Form Score ghi nhận N/A/Trống, các chỉ số set/rep/weight ghi nhận thủ công để làm cơ sở tính tải lượng tập luyện.
* **Mô hình cốt lõi**: `WorkoutSession`, `WorkoutSetLog`, `ExerciseExecution`, `RepRecord`, `SkeletonFrame`, `TimerConfig`.

### 4. AI Nutrition Context (Bối cảnh Dinh dưỡng AI)
* **Phạm vi**: Tính toán TDEE (Mifflin-St Jeor) và phân chia tỷ lệ dinh dưỡng đa lượng, tạo thực đơn gợi ý 3 bữa chính + 1 bữa phụ với 3 mức ngân sách (Tiết kiệm, Phổ thông, Thoải mái), hỗ trợ gợi ý bữa ăn tự nấu hoặc ăn ngoài quán tiệm (ưu tiên đối tác), thực thi luật luân chuyển chống lặp món.
* **Quy tắc nghiệp vụ chính**:
  * **BR-NU-01**: Không được phép gợi ý chế độ ăn có tổng năng lượng dưới **1,200 kcal/ngày**.
  * **BR-NU-02**: Khóa nguồn protein chính đã ăn trong 7 ngày, tinh bột 5 ngày và chủ đề món ăn 3 ngày không xuất hiện lại trong thực đơn gợi ý.
  * **BR-NU-03**: Hỗ trợ tư vấn chi tiết định lượng cho đồ ăn tự chuẩn bị hoặc quán ngoài tiệm, ưu tiên đề xuất sản phẩm đối tác tiện lợi tương đương.
* **Mô hình cốt lõi**: `NutritionPlan`, `MealSuggestion` (Tự nấu / Ăn ngoài), `FoodIngredient`, `MealHistory`, `LockoutRegistry`.
