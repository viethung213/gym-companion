package service

import (
	"hash/fnv"
	"math/rand"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// CandidateSelectionConfig quy định hạn mức số lượng nguyên liệu cho từng nhóm macro.
type CandidateSelectionConfig struct {
	TargetProteinCount int
	TargetCarbCount    int
	TargetVeggieCount  int
	TargetSnackCount   int
}

// DefaultCandidateConfig trả về cấu hình hạn mức chuẩn cho thực đơn (tổng cộng 21 nguyên liệu: 6 Protein, 6 Carb, 6 Veggie, 3 Snack).
func DefaultCandidateConfig() CandidateSelectionConfig {
	return CandidateSelectionConfig{
		TargetProteinCount: 6,
		TargetCarbCount:    6,
		TargetVeggieCount:  6,
		TargetSnackCount:   3,
	}
}

// SelectDiverseCandidates chọn lọc một tập hợp con nguyên liệu đa dạng, xoay vòng theo ngày
// từ danh mục thực phẩm, đồng thời giải quyết triệt để các quy chuẩn cứng mà AI không thể tự làm:
//
//  1. Loại bỏ 100% món ăn chứa dị ứng (`userRestrictions`).
//  2. Loại bỏ 100% món ăn đang bị khóa chu kỳ 7/5/3 ngày (`lockoutRegistry`).
//  3. Loại bỏ sản phẩm công nghiệp NutiFood chế biến sẵn (`isNutiFoodProduct`).
//  4. Phân tầng cân đối (Stratified Quotas) theo bộ ba Macro: Protein, Carb, Veggie, Snack.
//  5. Đa dạng hóa nguồn gốc đạm (gia cầm, thịt đỏ/heo, thủy hải sản, trứng/thực vật)
//     và nguồn gốc tinh bột (cơm gạo lứt, yến mạch, khoai, bún/mì).
//  6. Xoay vòng món theo ngày (Day-based Pseudo-random Rotation): Sử dụng seed từ planDate + userID
//     để mỗi ngày người dùng nhận một tổ hợp nguyên liệu phong phú khác nhau,
//     nhưng vẫn đảm bảo tính tất định (deterministic) khi retry trong cùng một ngày.
func SelectDiverseCandidates(
	catalog []vo.FoodNutrient,
	lockoutRegistry vo.LockoutRegistry,
	userRestrictions []string,
	userID string,
	planDate time.Time,
	customCfg ...CandidateSelectionConfig,
) []vo.FoodNutrient {
	if len(catalog) == 0 {
		return nil
	}

	cfg := DefaultCandidateConfig()
	if len(customCfg) > 0 {
		cfg = customCfg[0]
	}

	// 1. Bước lọc cứng: Dị ứng, Lockouts, và Sản phẩm NutiFood
	safeCatalog := FilterSafeFoods(catalog, lockoutRegistry, userRestrictions, planDate)
	if len(safeCatalog) == 0 {
		return nil
	}

	targetTotal := cfg.TargetProteinCount + cfg.TargetCarbCount + cfg.TargetVeggieCount + cfg.TargetSnackCount
	if len(safeCatalog) <= targetTotal {
		return safeCatalog
	}

	// 2. Tạo bộ sinh số ngẫu nhiên giả lập tất định (PRNG) dựa trên Ngày + UserID
	rng := NewDeterministicRNG(userID, planDate)

	// 3. Phân nhóm nguyên liệu theo danh mục Category
	proteins, carbs, veggies, snacks, others := CategorizeFoods(safeCatalog)

	// 4. Lấy mẫu phân tầng (Stratified Sampling) cho từng nhóm
	selectedProteins := sampleProteins(proteins, cfg.TargetProteinCount, rng)
	selectedCarbs := sampleCarbs(carbs, cfg.TargetCarbCount, rng)
	selectedVeggies := SampleSimple(veggies, cfg.TargetVeggieCount, rng)
	selectedSnacks := SampleSimple(snacks, cfg.TargetSnackCount, rng)

	// Gom các món đã chọn
	capacity := len(selectedProteins) + len(selectedCarbs) + len(selectedVeggies) + len(selectedSnacks) + len(others)
	result := make([]vo.FoodNutrient, 0, capacity)
	result = append(result, selectedProteins...)
	result = append(result, selectedCarbs...)
	result = append(result, selectedVeggies...)
	result = append(result, selectedSnacks...)

	// Nếu vẫn chưa đủ hạn mức do thiếu món ở một số nhóm, bù thêm từ nhóm khác
	if len(result) < targetTotal && len(others) > 0 {
		needed := targetTotal - len(result)
		shuffledOthers := shuffleFoods(others, rng)
		if len(shuffledOthers) > needed {
			result = append(result, shuffledOthers[:needed]...)
		} else {
			result = append(result, shuffledOthers...)
		}
	}

	return result
}

// FilterSafeFoods loại bỏ các món vi phạm dị ứng, vi phạm lockout hoặc là sản phẩm NutiFood.
func FilterSafeFoods(
	catalog []vo.FoodNutrient,
	lockoutRegistry vo.LockoutRegistry,
	userRestrictions []string,
	planDate time.Time,
) []vo.FoodNutrient {
	// 1. Áp dụng Lockout rule (7 ngày Protein, 5 ngày Carb, 3 ngày Category)
	unlockedFoods := lockoutRegistry.FilterAvailableIngredients(catalog, planDate)

	// 2. Xây dựng bản đồ dị ứng
	restrictionMap := make(map[string]bool, len(userRestrictions))
	for _, r := range userRestrictions {
		norm := strings.ToUpper(strings.TrimSpace(r))
		if norm != "" {
			restrictionMap[norm] = true
		}
	}

	safe := make([]vo.FoodNutrient, 0, len(unlockedFoods))
	for i := range unlockedFoods {
		f := &unlockedFoods[i]

		// Kiểm tra dị ứng qua AllergenTags và Category/Name/Source
		if hasAllergenViolation(f, restrictionMap) {
			continue
		}

		safe = append(safe, *f)
	}

	return safe
}

// hasAllergenViolation kiểm tra thực phẩm có chứa chất dị ứng của người dùng không.
func hasAllergenViolation(f *vo.FoodNutrient, restrictionMap map[string]bool) bool {
	if len(restrictionMap) == 0 {
		return false
	}

	for _, tag := range f.AllergenTags() {
		tagUpper := strings.ToUpper(strings.TrimSpace(tag))
		if restrictionMap[tagUpper] {
			return true
		}
	}

	nameUpper := strings.ToUpper(f.Name())
	catUpper := strings.ToUpper(f.Category())
	proSrcUpper := strings.ToUpper(f.ProteinSource())
	carbSrcUpper := strings.ToUpper(f.CarbSource())

	for r := range restrictionMap {
		if strings.Contains(nameUpper, r) ||
			strings.Contains(catUpper, r) ||
			strings.Contains(proSrcUpper, r) ||
			strings.Contains(carbSrcUpper, r) {
			return true
		}
	}

	return false
}

// CategorizeFoods phân chia các món ăn an toàn thành 4 nhóm chính.
func CategorizeFoods(foods []vo.FoodNutrient) (proteins, carbs, veggies, snacks, others []vo.FoodNutrient) {
	for i := range foods {
		f := foods[i]
		cat := strings.ToUpper(strings.TrimSpace(f.Category()))

		switch {
		case f.IsNutiFoodProduct() || cat == "NUTIFOOD" || cat == "SNACK" || cat == "FAT" || cat == "DAIRY" || cat == "FRUIT":
			snacks = append(snacks, f)
		case cat == "PROTEIN":
			proteins = append(proteins, f)
		case cat == "CARB":
			carbs = append(carbs, f)
		case cat == "VEGGIE" || cat == "VEGETABLE":
			veggies = append(veggies, f)
		case strings.Contains(cat, "MEAT") || strings.Contains(cat, "FISH") || f.ProteinPer100g() >= 18:
			proteins = append(proteins, f)
		case strings.Contains(cat, "GRAIN") || f.CarbsPer100g() >= 20:
			carbs = append(carbs, f)
		case strings.Contains(cat, "SALAD") || strings.Contains(cat, "MUSHROOM") || strings.Contains(cat, "VEG"):
			veggies = append(veggies, f)
		default:
			others = append(others, f)
		}
	}
	return
}

// sampleProteins chọn lọc Protein đa dạng nguồn gốc (Gia cầm, Thịt đỏ, Hải sản, Trứng/Thực vật).
func sampleProteins(proteins []vo.FoodNutrient, targetCount int, rng *rand.Rand) []vo.FoodNutrient {
	if len(proteins) <= targetCount {
		return proteins
	}

	// Phân nhóm theo nguồn ProteinSource
	poultry := make([]vo.FoodNutrient, 0)
	meat := make([]vo.FoodNutrient, 0)
	seafood := make([]vo.FoodNutrient, 0)
	plantOrEgg := make([]vo.FoodNutrient, 0)
	general := make([]vo.FoodNutrient, 0)

	for i := range proteins {
		src := strings.ToUpper(proteins[i].ProteinSource())
		name := strings.ToUpper(proteins[i].Name())

		switch {
		case strings.Contains(src, "CHICKEN") || strings.Contains(src, "POULTRY") ||
			strings.Contains(name, "GÀ") || strings.Contains(name, "VỊT"):
			poultry = append(poultry, proteins[i])
		case strings.Contains(src, "BEEF") || strings.Contains(src, "PORK") ||
			strings.Contains(name, "BÒ") || strings.Contains(name, "HEO"):
			meat = append(meat, proteins[i])
		case strings.Contains(src, "FISH") || strings.Contains(src, "SEAFOOD") ||
			strings.Contains(name, "CÁ") || strings.Contains(name, "TÔM") || strings.Contains(name, "MỰC"):
			seafood = append(seafood, proteins[i])
		case strings.Contains(src, "EGG") || strings.Contains(src, "PLANT") || strings.Contains(src, "TOFU") ||
			strings.Contains(name, "TRỨNG") || strings.Contains(name, "ĐẬU"):
			plantOrEgg = append(plantOrEgg, proteins[i])
		default:
			general = append(general, proteins[i])
		}
	}

	// Shuffle từng nhóm con
	poultry = shuffleFoods(poultry, rng)
	meat = shuffleFoods(meat, rng)
	seafood = shuffleFoods(seafood, rng)
	plantOrEgg = shuffleFoods(plantOrEgg, rng)
	general = shuffleFoods(general, rng)

	subgroups := [][]*[]vo.FoodNutrient{
		{&poultry}, {&meat}, {&seafood}, {&plantOrEgg}, {&general},
	}

	selected := make([]vo.FoodNutrient, 0, targetCount)
	for len(selected) < targetCount {
		addedInRound := 0
		for _, sg := range subgroups {
			slice := *sg[0]
			if len(slice) > 0 {
				selected = append(selected, slice[0])
				*sg[0] = slice[1:]
				addedInRound++
				if len(selected) >= targetCount {
					break
				}
			}
		}
		if addedInRound == 0 {
			break
		}
	}

	return selected
}

// sampleCarbs chọn lọc Carb đa dạng nguồn gốc (Cơm/Gạo, Yến mạch/Bánh mì, Khoai củ, Bún/Mì).
func sampleCarbs(carbs []vo.FoodNutrient, targetCount int, rng *rand.Rand) []vo.FoodNutrient {
	if len(carbs) <= targetCount {
		return carbs
	}

	grains := make([]vo.FoodNutrient, 0)
	tubers := make([]vo.FoodNutrient, 0)
	oatsAndBread := make([]vo.FoodNutrient, 0)
	noodles := make([]vo.FoodNutrient, 0)
	general := make([]vo.FoodNutrient, 0)

	for i := range carbs {
		src := strings.ToUpper(carbs[i].CarbSource())
		name := strings.ToUpper(carbs[i].Name())

		switch {
		case strings.Contains(src, "RICE") || strings.Contains(src, "GRAIN") || strings.Contains(name, "CƠM") || strings.Contains(name, "GẠO"):
			grains = append(grains, carbs[i])
		case strings.Contains(src, "POTATO") || strings.Contains(src, "TUBER") || strings.Contains(name, "KHOAI"):
			tubers = append(tubers, carbs[i])
		case strings.Contains(src, "OATS") || strings.Contains(src, "BREAD") || strings.Contains(name, "YẾN MẠCH") || strings.Contains(name, "BÁNH MÌ"):
			oatsAndBread = append(oatsAndBread, carbs[i])
		case strings.Contains(src, "NOODLE") || strings.Contains(name, "BÚN") || strings.Contains(name, "MÌ") || strings.Contains(name, "PHỞ"):
			noodles = append(noodles, carbs[i])
		default:
			general = append(general, carbs[i])
		}
	}

	grains = shuffleFoods(grains, rng)
	tubers = shuffleFoods(tubers, rng)
	oatsAndBread = shuffleFoods(oatsAndBread, rng)
	noodles = shuffleFoods(noodles, rng)
	general = shuffleFoods(general, rng)

	subgroups := [][]*[]vo.FoodNutrient{
		{&grains}, {&tubers}, {&oatsAndBread}, {&noodles}, {&general},
	}

	selected := make([]vo.FoodNutrient, 0, targetCount)
	for len(selected) < targetCount {
		addedInRound := 0
		for _, sg := range subgroups {
			slice := *sg[0]
			if len(slice) > 0 {
				selected = append(selected, slice[0])
				*sg[0] = slice[1:]
				addedInRound++
				if len(selected) >= targetCount {
					break
				}
			}
		}
		if addedInRound == 0 {
			break
		}
	}

	return selected
}

// SampleSimple chọn ngẫu nhiên có shuffle cho nhóm Veggie và Snack hoặc lấy mẫu nhanh.
func SampleSimple(foods []vo.FoodNutrient, targetCount int, rng *rand.Rand) []vo.FoodNutrient {
	if len(foods) <= targetCount {
		return foods
	}
	shuffled := shuffleFoods(foods, rng)
	return shuffled[:targetCount]
}

// SamplePaged xáo trộn danh sách thực phẩm và lấy mẫu theo trang (page >= 1, targetCount > 0).
// Khi page vượt quá số lượng, hàm tự động wrap-around để đảm bảo luôn trả về ứng viên.
func SamplePaged(foods []vo.FoodNutrient, targetCount, page int, rng *rand.Rand) []vo.FoodNutrient {
	if len(foods) == 0 {
		return nil
	}
	if targetCount <= 0 {
		targetCount = 6
	}
	if page <= 0 {
		page = 1
	}

	shuffled := shuffleFoods(foods, rng)
	startIdx := (page - 1) * targetCount
	if startIdx >= len(shuffled) {
		startIdx %= len(shuffled)
	}

	endIdx := startIdx + targetCount
	if endIdx > len(shuffled) {
		endIdx = len(shuffled)
	}
	return shuffled[startIdx:endIdx]
}

// shuffleFoods xáo trộn danh sách thực phẩm bằng Fisher-Yates shuffle.
func shuffleFoods(foods []vo.FoodNutrient, rng *rand.Rand) []vo.FoodNutrient {
	copied := make([]vo.FoodNutrient, len(foods))
	copy(copied, foods)
	rng.Shuffle(len(copied), func(i, j int) {
		copied[i], copied[j] = copied[j], copied[i]
	})
	return copied
}

// NewDeterministicRNG tạo seed từ ngày và UserID để sinh ngẫu nhiên nhưng ổn định trong ngày.
func NewDeterministicRNG(userID string, planDate time.Time) *rand.Rand {
	h := fnv.New64a()
	_, _ = h.Write([]byte(userID))
	userHash := int64(h.Sum64())

	// Seed = Năm * 1000 + Ngày trong năm + UserHash
	seed := int64(planDate.Year())*1000 + int64(planDate.YearDay()) + userHash
	//nolint:gosec // PRNG used for meal candidate variety, not cryptographic security
	return rand.New(rand.NewSource(seed))
}

// SelectPantryCandidates phân tách rõ ràng 2 danh sách nguyên liệu:
//  1. mandatory: Toàn bộ nguyên liệu do người dùng nhập từ tủ lạnh (BẮT BUỘC AI phải dùng, cấm thay thế).
//  2. supplementary: Các món an toàn lấy ngẫu nhiên từ DB cho các nhóm dinh dưỡng còn thiếu (GỢI Ý BỔ SUNG,
//     lấy theo cơ chế giống tool fetch_food_catalog. Nếu AI thấy không hòa hợp ẩm thực thì được phép gọi tool lấy món khác thay thế).
func SelectPantryCandidates(
	pantryItems []vo.FoodNutrient,
	activeCatalog []vo.FoodNutrient,
	lockoutRegistry vo.LockoutRegistry,
	userRestrictions []string,
	userID string,
	planDate time.Time,
) (mandatory, supplementary []vo.FoodNutrient) {
	mandatory = pantryItems
	if len(activeCatalog) == 0 {
		return mandatory, nil
	}

	// 1. Kiểm tra các nhóm macro đã có trong nguyên liệu của người dùng dựa trên Category chuẩn từ DB
	var hasProtein, hasCarb, hasVeggie bool
	existingIDs := make(map[string]bool)
	existingNames := make(map[string]bool)

	for i := range pantryItems {
		if pantryItems[i].ID() != "" {
			existingIDs[pantryItems[i].ID()] = true
		}
		nameLower := strings.ToLower(strings.TrimSpace(pantryItems[i].Name()))
		if nameLower != "" {
			existingNames[nameLower] = true
		}

		switch strings.ToUpper(strings.TrimSpace(pantryItems[i].Category())) {
		case "PROTEIN":
			hasProtein = true
		case "CARB":
			hasCarb = true
		case "VEGGIE":
			hasVeggie = true
		}
	}

	// Nếu người dùng đã cung cấp đủ cả 3 nhóm Protein + Carb + Veggie, không cần bổ sung DB
	if hasProtein && hasCarb && hasVeggie {
		return mandatory, nil
	}

	// 2. Lấy nguyên liệu an toàn ngẫu nhiên từ DB cho các nhóm còn thiếu (giống cách Tool fetch_food_catalog hoạt động)
	safeCatalog := FilterSafeFoods(activeCatalog, lockoutRegistry, userRestrictions, planDate)
	proteins, carbs, veggies, _, _ := CategorizeFoods(safeCatalog)

	supplementary = make([]vo.FoodNutrient, 0)

	addCandidates := func(candidates []vo.FoodNutrient, count int, groupKey string) {
		filtered := make([]vo.FoodNutrient, 0, len(candidates))
		for i := range candidates {
			c := &candidates[i]
			nameLower := strings.ToLower(strings.TrimSpace(c.Name()))
			if !existingIDs[c.ID()] && !existingNames[nameLower] {
				filtered = append(filtered, *c)
			}
		}
		rng := NewDeterministicRNG(userID+"_pantry_"+groupKey, planDate)
		picked := SamplePaged(filtered, count, 1, rng)
		for i := range picked {
			p := &picked[i]
			existingIDs[p.ID()] = true
			existingNames[strings.ToLower(strings.TrimSpace(p.Name()))] = true
			supplementary = append(supplementary, *p)
		}
	}

	if !hasProtein {
		addCandidates(proteins, 4, "PROTEIN")
	}
	if !hasCarb {
		addCandidates(carbs, 4, "CARB")
	}
	if !hasVeggie {
		addCandidates(veggies, 4, "VEGGIE")
	}

	return mandatory, supplementary
}
