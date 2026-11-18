package service

import (
	"hash/fnv"
	"math/rand"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// NewDeterministicRNG tạo PRNG có seed từ UserID và ngày để sinh số ngẫu nhiên nhưng ổn định trong cùng một ngày.
func NewDeterministicRNG(userID string, planDate time.Time) *rand.Rand {
	h := fnv.New64a()
	_, _ = h.Write([]byte(userID))
	userHash := int64(h.Sum64())

	seed := int64(planDate.Year())*1000 + int64(planDate.YearDay()) + userHash
	//nolint:gosec // PRNG used for meal candidate variety, not cryptographic security
	return rand.New(rand.NewSource(seed))
}

// SamplePaged xáo trộn danh sách thực phẩm và lấy mẫu theo trang (page >= 1, targetCount > 0).
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
