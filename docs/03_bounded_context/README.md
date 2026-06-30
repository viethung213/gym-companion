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
│  │ - Đăng ký, Đăng nhập, Profile│         │ - Lên lịch tập 4 tuần    │ │
│  │ - Khai báo chấn thương       │         │ - Progressive Overload   │ │
│  │ - Ràng buộc hoàn thành >=80% │         │ - Đổi bài khi chấn thương│ │
│  └──────────────────────────────┘         └──────────────────────────┘ │
│                 │                                      ▲               │
│                 ▼                                      │               │
│  ┌───────────────────────────────┐       ┌────────────────────────────┐│
│  │ Workout Execution             │       │ AI Nutrition               ││
│  │ Context                       │       │ Context                    ││
│  │ - Tracking skeleton           │       │ - Tính toán TDEE           ││
│  │ - Đếm rep, ROM%, Phát hiện lỗi│       │ - Anti-Repetition          ││
│  │ - Audio Ducking, Nhập tạ      │       │ - Gợi ý món theo giá       ││
│  └───────────────────────────────┘       └────────────────────────────┘│
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 3.2 Đặc Tả Từng Bounded Context

### 1. User Profile & Health Context (Bối cảnh Hồ sơ & Sức khỏe)
* **Phạm vi**: Đăng nhập, đăng ký, xác thực OTP, quản lý tài khoản người dùng, nhập chỉ số cơ thể ban đầu, theo dõi và lưu trữ lịch sử chỉ số cơ thể (cân nặng, tỷ lệ mỡ) theo dòng thời gian tập luyện, danh sách vùng chấn thương, tiền sử bệnh lý.
* **Quy tắc nghiệp vụ chính**:
  * **BR-UM-01**: Yêu cầu hồ sơ đạt $\ge 80\%$ độ hoàn thiện mới cho phép kích hoạt AI Coach và tạo kế hoạch tập đầu tiên.
* **Mô hình cốt lõi**: `User`, `HealthProfile`, `BodyMetricsHistory`, `InjuryList`, `DeviceRegistration`.

### 2. AI Coaching & Planning Context (Bối cảnh Huấn luyện & Lên kế hoạch)
* **Phạm vi**: Lên kế hoạch tập luyện 4 tuần đầu, tự động điều chỉnh độ khó giáo án sau mỗi 2 tuần dựa trên kết quả buổi tập thực tế, tự động thay thế bài tập khi người dùng báo chấn thương (loại bỏ vùng cơ ảnh hưởng), quản lý phong cách tương tác của AI Coach.
* **Quy tắc nghiệp vụ chính**:
  * **BR-AC-01**: Tối đa **6 buổi tập/tuần** và bắt buộc phải có ít nhất **1 ngày nghỉ hoàn toàn**.
  * **BR-AC-02**: Tốc độ tăng tiến khối lượng tập luyện (Progressive Overload) không vượt quá **10% tổng volume** của tuần trước.
* **Mô hình cốt lõi**: `WorkoutPlan`, `WeeklySchedule`, `ExerciseSpecification`, `CoachPersonality`, `ProgressionRule`.

### 3. Workout Execution Context (Bối cảnh Thực thi Buổi tập)
* **Phạm vi**: Bắt đầu buổi tập, check-in, tracking skeleton bằng camera, đo lường góc ROM%, phát hiện lỗi chuyển động và cảnh báo bằng hình ảnh/âm thanh (< 500ms), Audio Ducking khi có cảnh báo lỗi, đếm số rep đạt chuẩn, ước lượng cân nặng tạ thực tế, ghi chép Set tập tự động và cho phép sửa tay.
* **Quy tắc nghiệp vụ chính**:
  * **BR-CC-01**: Rep chỉ hợp lệ để đếm khi ROM đạt tối thiểu **$\ge 70\%$** so với biên độ tiêu chuẩn.
  * **BR-CC-02**: Nếu tỷ lệ khung hình skeleton hợp lệ < 50% trong suốt buổi tập, hệ thống đánh dấu buổi tập là "Không đạt chuẩn xác thực" (chống gian lận).
* **Mô hình cốt lõi**: `WorkoutSession`, `WorkoutSet`, `ExerciseExecution`, `RepRecord`, `SkeletonFrame`, `AudioDuckingConfig`.

### 4. AI Nutrition Context (Bối cảnh Dinh dưỡng AI)
* **Phạm vi**: Tính toán TDEE (Mifflin-St Jeor) và phân chia tỷ lệ dinh dưỡng đa lượng, tạo thực đơn 3 bữa chính + 1 bữa phụ với 3 mức ngân sách (Tiết kiệm, Phổ thông, Thoải mái), thực thi thuật toán luân chuyển món ăn.
* **Quy tắc nghiệp vụ chính**:
  * **BR-NU-01**: Không được phép gợi ý chế độ ăn có tổng năng lượng dưới **1,200 kcal/ngày**.
  * **BR-NU-02**: Khóa nguồn protein chính đã ăn và không xuất hiện lại trong thực đơn gợi ý trong vòng **7 ngày** tiếp theo.
* **Mô hình cốt lõi**: `NutritionPlan`, `MealSuggestion`, `FoodIngredient`, `MealHistory`, `TDEEFormula`.
