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
3. System truy vấn danh sách nguyên liệu dị ứng của người dùng trực tiếp từ cơ sở dữ liệu `nutrition.user_allergies`, áp dụng bộ lọc dị ứng nghiêm ngặt (`ValidatePantryIngredientsAgainstAllergens` và `FilterAllergensOnly` - BR-NU-04), đồng thời truy vấn lịch sử tiêu thụ gần đây (`FindByUserIDAndDates`) để tính điểm mềm phân rã hàm mũ (`CandidateSelector` - BR-NU-02).
4. System sinh `DailyMealOption` cho 3 bữa chính + 1 bữa phụ theo `BudgetTier` user đã chọn. Đối với mỗi bữa ăn, System ưu tiên đính kèm đề xuất sản phẩm đối tác tiện lợi tương đương nếu có sẵn trong cơ sở dữ liệu; nếu không có, hệ thống hiển thị hướng dẫn định lượng để người dùng tự chuẩn bị hoặc mua ngoài tiệm bình thường (BR-NU-03). Client không cần truyền tham số dị ứng thủ công trong request.
5. System phát `NutritionPlanGenerated`.

**Alternative Flow**
- A1: Ngày tập nặng (`WorkoutSessionCompleted` → volume cao) → tăng calo target ~10%.
- A2: Ngày nghỉ → giảm calo target ~10%.

**Error / Edge Cases**
- E1: `CalorieAllocation.target` < 1200 kcal sau tính toán → buộc giữ nguyên 1200 kcal (BR-NU-01).
- E2: Tất cả nguyên liệu protein ăn gần đây → System áp dụng phân rã hàm mũ tự nhiên để ưu tiên món có khoảng cách xa nhất, không bao giờ bị nghẽn thực đơn.
- E3: `BiologicalMetrics` chưa cập nhật cân nặng > 7 ngày → dùng giá trị cuối cùng và hiển thị cảnh báo.
- E4: User có nhiều dị ứng nghiêm ngặt → System giữ nguyên việc lọc dị ứng bắt buộc, nếu danh mục an toàn còn ít vẫn sinh thực đơn từ các nguồn đạm an toàn khả dụng kèm cảnh báo.

**Postcondition**: `NutritionPlan` với `DailyMealOption` đầy đủ sẵn sàng hiển thị.  
> *`NutritionService.GenerateDailyPlan()` gọi `NutritionPlanRepository.Save()`, `UserAllergyRepository.FindAllergiesByUserID()`, và `NutritionPlanRepository.FindByUserIDAndDates()`.*

**Domain Events**: `NutritionPlanGenerated`

---

### UC-05.2 LogMeal

| | |
|---|---|
| **Actor** | User |
| **Precondition** | User đã đăng nhập. |

**Main Flow**
1. User tìm kiếm món ăn theo tên hoặc quét mã vạch (hoặc 1-tap quick log từ thực đơn gợi ý).
2. System trả về `FoodItem` phù hợp từ thư viện thực phẩm hoặc AI ước tính calo nếu nhập tự do ngoài danh mục.
3. User xác nhận khẩu phần (gram).
4. System tạo `MealLog`, ghi nhận thông tin tiêu thụ và danh sách nguyên liệu của bữa ăn.
5. System phát `MealLogged`.

**Alternative Flow**
- A1: Không tìm thấy món trong thư viện → User nhập thủ công tên + calo/macro ước tính (hoặc để AI ước tính tự động).

**Error / Edge Cases**
- E1: Quét mã vạch thất bại / sản phẩm không có trong database → fallback sang nhập thủ công.
- E2: User cố log cùng một món 2 lần trong ngày → cảnh báo trùng lặp, vẫn cho phép nếu user xác nhận.

**Postcondition**: `MealLog` được ghi nhận trong `MealHistory`.

**Domain Events**: `MealLogged`
