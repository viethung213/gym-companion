package service

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// ComputeIngredientHash tạo SHA-256 hash 16 ký tự hex (8 bytes) từ 4 thành phần (protein, carb, veggie, style).
// Các thành phần được chuẩn hóa (lowercase, trim whitespace) và sắp xếp trước khi băm,
// đảm bảo kết quả nhất quán bất kể thứ tự truyền vào.
// Dùng làm khóa tra cứu trong RecipeCacheRepository để tái sử dụng công thức giữa các lượt gọi AI.
func ComputeIngredientHash(protein, carb, veggie, style string) string {
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
