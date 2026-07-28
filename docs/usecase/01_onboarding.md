# UC-01 Onboarding

> Nguồn: [BRD](../NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](../02_bounded_context.md) · [Tactical Design](../03_ddd_tactical_design.md)

**Actor**: `User` (người tập) · `System` (AI Coach / AI Camera / AI Nutrition)  
**Format mỗi Use Case**: Precondition → Main Flow → Alternative Flow → Error / Edge Cases → Postcondition → Domain Events

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
1. User nhập tuổi, giới tính, chiều cao, cân nặng, mục tiêu chính `primary_goal` (Tăng cơ / Giảm mỡ), danh sách dụng cụ tập luyện `available_equipment` và nhóm cơ ưu tiên `preferred_muscle_groups`.
2. User chọn khung giờ rảnh trong tuần `available_slots` (hệ thống hỗ trợ gán giá trị mặc định và cho phép thay đổi bất kỳ lúc nào).
3. System tính `ProfileCompletionRate` dựa trên các trường bắt buộc (các chỉ số sinh học và mục tiêu). `available_equipment` mặc định luôn bao gồm `"BODYWEIGHT"`.
4. Khi tỷ lệ ≥ 80%, System kích hoạt `ActiveCoachEnabled = true`.

**Alternative Flow**
- A1: User bỏ qua việc thiết lập khung giờ rảnh — System tự động áp dụng danh sách rỗng (hoặc mặc định) và cho phép cập nhật sau này trong Profile settings.
- A2: User khai báo chấn thương cũ hoặc bệnh lý mãn tính — Chuyển tiếp thực hiện **UC-01.3 ReportInjury** để ghi nhận vào hồ sơ.

**Error / Edge Cases**
- E1: Giá trị cân nặng / chiều cao không hợp lệ (≤ 0) → từ chối lưu, hiển thị lỗi inline.
- E2: Hoàn thiện < 80% (thiếu các trường chỉ số sinh học hoặc mục tiêu) → `ActiveCoachEnabled` giữ `false`, không sinh lộ trình.

**Postcondition**: `User.BiologicalMetrics`, `primary_goal`, `available_equipment`, `preferred_muscle_groups`, `available_slots` được cập nhật. Nếu đủ điều kiện (≥ 80%), `UserProfileCompleted` được phát.  
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

### UC-01.4 UpdateUserProfile

| | |
|---|---|
| **Actor** | User |
| **Precondition** | User đã đăng nhập. Hồ sơ đã hoàn thiện. |

**Main Flow**
1. User cập nhật các trường thông tin trong Profile (`primary_goal`, `available_equipment`, `preferred_muscle_groups`, `available_slots`).
2. System cập nhật thông tin và phát `ProfileUpdated`.
3. `Coaching Context` nhận `ProfileUpdated` và tự động kích hoạt Re-generation (FR-AC-06) cho các `SessionPlan` bắt đầu từ ngày tập tiếp theo.

**Domain Events**: `ProfileUpdated`
