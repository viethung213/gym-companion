package service

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// CombinatorialMatrix chỉ còn 1 nhiệm vụ: tạo ingredient_hash cho Recipe Cache.
// Toàn bộ việc chọn combo thực phẩm và tính Gram Macro đã chuyển sang AI Agent.
type CombinatorialMatrix struct{}

func NewCombinatorialMatrix() *CombinatorialMatrix {
	return &CombinatorialMatrix{}
}

// ComputeIngredientHash tạo SHA-256 hash 8 ký tự từ (protein, carb, veggie, style).
// Dùng để check Recipe Cache trước khi gọi AI.
func (c *CombinatorialMatrix) ComputeIngredientHash(protein, carb, veggie, style string) string {
	items := []string{
		strings.ToLower(strings.TrimSpace(protein)),
		strings.ToLower(strings.TrimSpace(carb)),
		strings.ToLower(strings.TrimSpace(veggie)),
		strings.ToLower(strings.TrimSpace(style)),
	}
	sort.Strings(items)
	rawKey := strings.Join(items, ":")
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:8])
}
