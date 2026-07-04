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
1. User nhập tuổi, giới tính, chiều cao, cân nặng, mục tiêu (Tăng cơ / Giảm mỡ), khung giờ tập cố định.
2. System tính `ProfileCompletionRate` dựa trên các trường bắt buộc của `BiologicalMetrics`.
3. Khi tỷ lệ ≥ 80%, System kích hoạt `ActiveCoachEnabled = true`.

**Alternative Flow**
- A1: User bỏ qua bước nhập — System lưu trạng thái hiện tại, nhắc lại ở lần mở app tiếp theo.

**Error / Edge Cases**
- E1: Giá trị cân nặng / chiều cao không hợp lệ (≤ 0) → từ chối lưu, hiển thị lỗi inline.
- E2: Hoàn thiện < 80% → `ActiveCoachEnabled` giữ `false`, không sinh lộ trình.

**Postcondition**: `User.BiologicalMetrics` được cập nhật. Nếu đủ điều kiện, `UserProfileCompleted` được phát.  
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
