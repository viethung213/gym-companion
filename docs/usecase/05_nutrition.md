# UC-05 Nutrition

> Nguồn: [BRD](../NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](../02_bounded_context.md) · [Tactical Design](../03_ddd_tactical_design.md)

**Actor**: `User` (người tập) · `System` (AI Coach / AI Camera / AI Nutrition)  
**Format mỗi Use Case**: Precondition → Main Flow → Alternative Flow → Error / Edge Cases → Postcondition → Domain Events

---

### UC-05.1 GenerateDailyNutritionPlan

| | |
|---|---|
| **Actor** | System (AI Nutrition) |
| **Precondition** | `UserProfileCompleted`. Chưa có `NutritionPlan` cho ngày hôm nay. |

**Main Flow**
1. System đọc `BiologicalMetrics` (cân nặng, mục tiêu, mức vận động từ `WorkoutSession` hôm nay) từ `User`.
2. System tính TDEE theo công thức Mifflin-St Jeor, tính `CalorieAllocation` (Protein/Carb/Fat).
3. System đọc `ChatbotContext.food_restrictions` của `User` để lọc bỏ các thực phẩm gây dị ứng/ăn kiêng, đồng thời kiểm tra `LockoutRegistry` của `MealHistory` để lọc nguyên liệu bị khóa.
4. System sinh `DailyMealOption` cho 3 bữa chính + 1 bữa phụ theo `BudgetTier` user đã chọn.
5. System phát `NutritionPlanGenerated`.

**Alternative Flow**
- A1: Ngày tập nặng (`WorkoutSessionCompleted` → volume cao) → tăng calo target ~10%.
- A2: Ngày nghỉ → giảm calo target ~10%.

**Error / Edge Cases**
- E1: `CalorieAllocation.target` < 1200 kcal sau tính toán → buộc giữ nguyên 1200 kcal (BR-NU-01).
- E2: Tất cả nguyên liệu protein bị `LockoutRegistry` khóa → System tự giải phóng nguyên liệu ít bị khóa nhất (unlock sớm nhất) để đảm bảo có thực đơn.
- E3: `BiologicalMetrics` chưa cập nhật cân nặng > 7 ngày → dùng giá trị cuối cùng và hiển thị cảnh báo.
- E4: Tất cả các nguồn đạm (protein) đều bị loại bỏ do kết hợp giữa dị ứng (`food_restrictions`) và danh sách khóa (`LockoutRegistry`) → System giữ nguyên việc lọc dị ứng bắt buộc, tự động giải phóng sớm các nguyên liệu bị khóa trong `LockoutRegistry` để đảm bảo sinh được thực đơn.

**Postcondition**: `NutritionPlan` với `DailyMealOption` đầy đủ sẵn sàng hiển thị.  
> *`NutritionService.GenerateDailyPlan()` gọi `NutritionPlanRepository.Save()`, `UserRepository.GetChatbotContext()`, và `MealHistoryRepository.GetLockouts()`.*

**Domain Events**: `NutritionPlanGenerated`

---

### UC-05.2 LogMeal

| | |
|---|---|
| **Actor** | User |
| **Precondition** | User đã đăng nhập. |

**Main Flow**
1. User tìm kiếm món ăn theo tên hoặc quét mã vạch.
2. System trả về `FoodItem` phù hợp từ Catalog.
3. User xác nhận khẩu phần (gram).
4. System tạo `MealLog`, cập nhật `LockoutRegistry` (Protein 7 ngày, Carb 5 ngày, Chủ đề 3 ngày).
5. System phát `MealLogged` và `LockoutApplied`.

**Alternative Flow**
- A1: Không tìm thấy món trong Catalog → User nhập thủ công tên + calo/macro ước tính.

**Error / Edge Cases**
- E1: Quét mã vạch thất bại / sản phẩm không có trong database → fallback sang nhập thủ công.
- E2: User cố log cùng một món 2 lần trong ngày → cảnh báo trùng lặp, vẫn cho phép nếu user xác nhận.

**Postcondition**: `MealLog` được ghi, `LockoutRegistry` được cập nhật.

**Domain Events**: `MealLogged` · `LockoutApplied`
