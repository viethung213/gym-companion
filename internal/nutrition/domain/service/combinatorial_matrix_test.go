package service_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
)

func TestCombinatorialMatrix_ComputeIngredientHash(t *testing.T) {
	t.Parallel()

	matrix := service.NewCombinatorialMatrix()

	// Hash giống nhau dù thứ tự khác nhau (vì sort trước khi hash).
	h1 := matrix.ComputeIngredientHash("Ức gà", "Khoai lang", "Bông cải xanh", "Áp chảo")
	h2 := matrix.ComputeIngredientHash("Bông cải xanh", "Ức gà", "Khoai lang", "Áp chảo")
	if h1 != h2 {
		t.Fatalf("hash phải giống nhau khi thứ tự nguyên liệu khác nhau: %s != %s", h1, h2)
	}

	// Hash khác nhau khi cooking style khác nhau.
	h3 := matrix.ComputeIngredientHash("Ức gà", "Khoai lang", "Bông cải xanh", "Hấp gừng")
	if h1 == h3 {
		t.Fatalf("hash phải khác nhau khi cooking style khác nhau")
	}

	// Hash có đúng 16 ký tự hex (8 bytes).
	if len(h1) != 16 {
		t.Fatalf("hash phải có 16 ký tự hex, got %d: %q", len(h1), h1)
	}
}
