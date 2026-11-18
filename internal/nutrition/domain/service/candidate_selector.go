package service

import (
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// Hạn mức số lượng nguyên liệu chuẩn theo nhóm dinh dưỡng (tổng cộng 21 nguyên liệu cho thực đơn 4 bữa).
const (
	DefaultTargetProteinCount  = 6
	DefaultTargetCarbCount     = 6
	DefaultTargetVeggieCount   = 6
	DefaultTargetSnackCount    = 3
	DefaultTotalCandidateCount = DefaultTargetProteinCount + DefaultTargetCarbCount + DefaultTargetVeggieCount + DefaultTargetSnackCount
)

// SelectionCriteria chứa toàn bộ tham số đầu vào cho việc chọn ứng viên nguyên liệu.
type SelectionCriteria struct {
	Catalog            []vo.FoodNutrient
	UserRestrictions   []string
	UserID             string
	PlanDate           time.Time
	NutritionGoal      vo.NutritionGoal
	UserPreferences    []string                   // Nguyên liệu, món ăn ưa thích từ insight (P_iu)
	TargetBudgetTier   string                     // BUDGET, STANDARD, PREMIUM (C_i)
	RecentConsumptions []vo.HistoricalConsumption // Lịch sử các bữa ăn kèm danh sách nguyên liệu và mốc thời gian (R_it)
}

// Trọng số chuẩn hóa cho công thức tính điểm ưu tiên:
// S(i, u, t) = w_n*N_i + w_p*P_iu - w_r*R_it - w_c*C_i
const (
	WeightNutrition  = 1.0 // w_n: Mức phù hợp nhu cầu dinh dưỡng
	WeightPreference = 0.8 // w_p: Mức phù hợp sở thích người dùng
	WeightRecency    = 1.0 // w_r: Trọng số phạt mức độ lặp lại gần đây
	WeightCost       = 0.8 // w_c: Mức bất lợi chi phí so với ngân sách
	TauDecayDays     = 2.0 // τ: Chu kỳ suy giảm ảnh hưởng lặp lại (ngày)
)

// CandidateSelector thực hiện chọn lọc ứng viên nguyên liệu dựa trên công thức tính điểm ưu tiên
// và phân rã lặp lại mềm (Soft Scoring & Exponential Decay):
// S(i, u, t) = w_n*N_i + w_p*P_iu - w_r*R_it - w_c*C_i
// R(i) = sum_{k=1}^n s(i, i_k) * e^{-Δt_k / τ}
type CandidateSelector struct{}

// NewCandidateSelector khởi tạo bộ chọn ứng viên nguyên liệu theo cơ chế chọn điểm mềm.
func NewCandidateSelector() *CandidateSelector {
	return &CandidateSelector{}
}

// SelectCandidates thực hiện chọn lọc nguyên liệu bằng công thức ưu tiên và phân tầng hạn mức.
func (s *CandidateSelector) SelectCandidates(criteria *SelectionCriteria) []vo.FoodNutrient {
	if criteria == nil || len(criteria.Catalog) == 0 {
		return nil
	}

	// BƯỚC 1: LỌC DỊ ỨNG TUYỆT ĐỐI (Hard exclusion duy nhất vì an toàn tính mạng)
	safeCatalog := FilterAllergensOnly(criteria.Catalog, criteria.UserRestrictions)
	if len(safeCatalog) == 0 {
		return nil
	}

	// BƯỚC 2: PHÂN NHÓM NGUYÊN LIỆU (Protein, Carb, Veggie, Snack, Others)
	proteins, carbs, veggies, snacks, others := CategorizeFoods(safeCatalog)

	// KIỂM TRA NGƯỠNG TỐI THIỂU THIẾT YẾU:
	// Để AI có thể lập thực đơn hợp lệ đủ macro, danh mục an toàn bắt buộc phải có đủ 3 nhóm dinh dưỡng chính
	if len(proteins) == 0 || len(carbs) == 0 || len(veggies) == 0 {
		return nil
	}

	if len(safeCatalog) <= DefaultTotalCandidateCount {
		return safeCatalog
	}

	consumptions := criteria.RecentConsumptions
	normalizedPrefs := normalizePreferences(criteria.UserPreferences)

	// BƯỚC 3: TÍNH ĐIỂM ƯU TIÊN VÀ XẾP HẠNG TỪNG NHÓM
	rng := NewDeterministicRNG(criteria.UserID, criteria.PlanDate)

	selectedProteins := s.rankAndSelect(proteins, DefaultTargetProteinCount, criteria, consumptions, normalizedPrefs, rng)
	selectedCarbs := s.rankAndSelect(carbs, DefaultTargetCarbCount, criteria, consumptions, normalizedPrefs, rng)
	selectedVeggies := s.rankAndSelect(veggies, DefaultTargetVeggieCount, criteria, consumptions, normalizedPrefs, rng)
	selectedSnacks := s.rankAndSelect(snacks, DefaultTargetSnackCount, criteria, consumptions, normalizedPrefs, rng)

	// Gom kết quả
	capacity := len(selectedProteins) + len(selectedCarbs) +
		len(selectedVeggies) + len(selectedSnacks) + len(others)
	result := make([]vo.FoodNutrient, 0, capacity)
	result = append(result, selectedProteins...)
	result = append(result, selectedCarbs...)
	result = append(result, selectedVeggies...)
	result = append(result, selectedSnacks...)

	// Bù nếu thiếu hụt từ nhóm others
	if len(result) < DefaultTotalCandidateCount && len(others) > 0 {
		needed := DefaultTotalCandidateCount - len(result)
		rankedOthers := s.rankAndSelect(others, needed, criteria, consumptions, normalizedPrefs, rng)
		result = append(result, rankedOthers...)
	}

	return result
}

// scoredFoodItem ghép thực phẩm với điểm số ưu tiên đã tính.
type scoredFoodItem struct {
	item  vo.FoodNutrient
	score float64
}

// rankAndSelect tính điểm cho toàn bộ thực phẩm trong nhóm và chọn top K theo điểm cao nhất.
func (s *CandidateSelector) rankAndSelect(
	items []vo.FoodNutrient,
	targetCount int,
	criteria *SelectionCriteria,
	consumptions []vo.HistoricalConsumption,
	normalizedPrefs []string,
	rng *rand.Rand,
) []vo.FoodNutrient {
	if len(items) <= targetCount {
		return items
	}

	scoredList := make([]scoredFoodItem, len(items))
	for i := range items {
		item := &items[i]
		score := s.CalculatePriorityScore(item, criteria, consumptions, normalizedPrefs)

		// Thêm một lượng jitter nhỏ ngẫu nhiên theo ngày (±0.05) để tránh đơn điệu
		// khi hai món có điểm bằng nhau hoặc sát sao
		jitter := (rng.Float64() - 0.5) * 0.1
		scoredList[i] = scoredFoodItem{
			item:  *item,
			score: score + jitter,
		}
	}

	// Sắp xếp giảm dần theo điểm số
	sort.SliceStable(scoredList, func(i, j int) bool {
		return scoredList[i].score > scoredList[j].score
	})

	selected := make([]vo.FoodNutrient, 0, targetCount)
	for i := 0; i < targetCount && i < len(scoredList); i++ {
		selected = append(selected, scoredList[i].item)
	}

	return selected
}

// CalculatePriorityScore tính điểm ưu tiên tổng hợp:
// S(i, u, t) = w_n*N_i + w_p*P_iu - w_r*R_it - w_c*C_i
func (s *CandidateSelector) CalculatePriorityScore(
	item *vo.FoodNutrient,
	criteria *SelectionCriteria,
	consumptions []vo.HistoricalConsumption,
	normalizedPrefs []string,
) float64 {
	ni := calculateNutritionFit(item, criteria.NutritionGoal)
	piu := calculatePreferenceFit(item.Name(), normalizedPrefs)
	rit := s.CalculateRecencyPenalty(item, consumptions, criteria.PlanDate)
	ci := calculateCostPenalty(item, criteria.TargetBudgetTier)

	return WeightNutrition*ni + WeightPreference*piu - WeightRecency*rit - WeightCost*ci
}

// CalculateRecencyPenalty tính mức phạt lặp lại bằng công thức phân rã hàm mũ:
// R(i) = sum_{k=1}^n s(i, i_k) * e^{-Δt_k / τ}
func (s *CandidateSelector) CalculateRecencyPenalty(
	item *vo.FoodNutrient,
	consumptions []vo.HistoricalConsumption,
	planDate time.Time,
) float64 {
	if len(consumptions) == 0 {
		return 0.0
	}

	var totalPenalty float64
	for i := range consumptions {
		c := &consumptions[i]

		// Tính khoảng cách ngày Δt_k
		diffHours := planDate.Sub(c.ConsumedAt).Hours()
		deltaDays := diffHours / 24.0
		if deltaDays < 0 {
			deltaDays = 0 // Cùng ngày hoặc ngày tương lai
		}

		similarity := calculateSimilarity(item, c)
		if similarity <= 0 {
			continue
		}

		// e^{-Δt_k / τ}
		decay := math.Exp(-deltaDays / TauDecayDays)
		totalPenalty += similarity * decay
	}

	return totalPenalty
}

// calculateSimilarity đánh giá mức độ tương đồng giữa nguyên liệu ứng viên và các nguyên liệu trong bữa ăn trước đó:
// - Nếu nguyên liệu ứng viên nằm trong danh sách nguyên liệu của bữa ăn: 1.0
// - Fallback nếu bữa ăn không có danh sách nguyên liệu chi tiết (món log ngoài): so khớp với tên món ăn: 1.0 nếu chứa nhau
// - Không trùng: 0.0
func calculateSimilarity(item *vo.FoodNutrient, prev *vo.HistoricalConsumption) float64 {
	itemNameUpper := strings.ToUpper(strings.TrimSpace(item.Name()))

	// 1. So khớp trực tiếp trong danh sách nguyên liệu của bữa ăn
	for _, ing := range prev.Ingredients {
		ingUpper := strings.ToUpper(strings.TrimSpace(ing))
		if ingUpper == itemNameUpper || strings.Contains(ingUpper, itemNameUpper) || strings.Contains(itemNameUpper, ingUpper) {
			return 1.0
		}
	}

	// 2. Fallback: Nếu không có danh sách nguyên liệu (món tự log ngoài plan), so khớp với tên món
	if len(prev.Ingredients) == 0 && prev.MealName != "" {
		mealNameUpper := strings.ToUpper(strings.TrimSpace(prev.MealName))
		if mealNameUpper == itemNameUpper || strings.Contains(mealNameUpper, itemNameUpper) || strings.Contains(itemNameUpper, mealNameUpper) {
			return 1.0
		}
	}

	return 0.0
}

// calculateNutritionFit (N_i) chuẩn hóa từ 0.0 đến 1.0 dựa trên mục tiêu dinh dưỡng.
func calculateNutritionFit(item *vo.FoodNutrient, goal vo.NutritionGoal) float64 {
	switch goal {
	case vo.GoalMuscleGain:
		// Ưu tiên mật độ Protein cao (>= 25g/100g đạt 1.0)
		return math.Min(1.0, item.ProteinPer100g()/25.0)

	case vo.GoalWeightLoss:
		// Ưu tiên Protein nạc và Calo thấp (Calo thấp + có đạm)
		calScore := math.Max(0.0, 1.0-item.CaloriesPer100g()/350.0)
		proScore := math.Min(1.0, item.ProteinPer100g()/20.0)
		return 0.5*calScore + 0.5*proScore

	default: // Maintenance
		// Cân đối: thực phẩm có macro đầy đủ được đánh giá tốt
		return 0.7
	}
}

// normalizePreferences chuẩn hóa danh sách từ khóa ưa thích một lần duy nhất.
func normalizePreferences(preferences []string) []string {
	if len(preferences) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(preferences))
	for _, p := range preferences {
		norm := strings.ToUpper(strings.TrimSpace(p))
		if norm != "" {
			normalized = append(normalized, norm)
		}
	}
	return normalized
}

// calculatePreferenceFit (P_iu) tính mức phù hợp sở thích bằng cách so khớp tên nguyên liệu (name).
func calculatePreferenceFit(itemName string, normalizedPrefs []string) float64 {
	if len(normalizedPrefs) == 0 {
		return 0.2 // Điểm cơ bản trung tính
	}
	nameUpper := strings.ToUpper(itemName)
	for _, pref := range normalizedPrefs {
		if strings.Contains(nameUpper, pref) {
			return 0.8 // Thưởng lớn nếu trùng tên món ưa thích
		}
	}
	return 0.2
}

// calculateCostPenalty (C_i) tính mức phạt chi phí so với ngân sách người dùng.
// Ngân sách: "BUDGET" (thấp), "STANDARD" (vừa), "PREMIUM" (cao / không giới hạn).
// Nếu người dùng không cấu hình ngân sách (targetBudget == ""), bỏ qua phạt chi phí (trả về 0.0).
func calculateCostPenalty(item *vo.FoodNutrient, targetBudget string) float64 {
	budget := strings.ToUpper(strings.TrimSpace(targetBudget))
	if budget == "" {
		return 0.0
	}

	tier := strings.ToUpper(strings.TrimSpace(item.CostTier()))
	if tier == "" {
		tier = "STANDARD"
	}

	switch budget {
	case "BUDGET", "LOW":
		switch tier {
		case "PREMIUM", "HIGH":
			return 1.0 // Phạt tối đa nếu dùng món đắt khi ngân sách eo hẹp
		case "STANDARD", "MEDIUM":
			return 0.4 // Phạt nhẹ
		default:
			return 0.0 // Món rẻ không bị phạt
		}

	case "STANDARD", "MEDIUM":
		if tier == "PREMIUM" || tier == "HIGH" {
			return 0.5 // Phạt vừa phải với món quá đắt
		}
		return 0.0

	case "PREMIUM", "HIGH":
		return 0.0 // Ngân sách dồi dào, không phạt chi phí

	default:
		return 0.0
	}
}
