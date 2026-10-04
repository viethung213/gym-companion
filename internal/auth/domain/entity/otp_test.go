package entity_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
)

func TestOTP_Invalidate(t *testing.T) {
	t.Parallel()

	now := time.Now()
	otp := entity.NewOTP(
		"otp-1",
		"user@example.com",
		"hashed",
		"register",
		3*time.Minute,
		60*time.Second,
		5,
	)

	// 1. Thử vô hiệu hóa khi chưa hết thời gian cooldown -> phải trả về ErrOTPResendCooldown
	beforeCooldown := now.Add(30 * time.Second)
	if err := otp.Invalidate(beforeCooldown); err != derror.ErrOTPResendCooldown {
		t.Fatalf("expected ErrOTPResendCooldown when before cooldown, got: %v", err)
	}
	if otp.IsUsed() {
		t.Fatalf("otp should NOT be marked as used if cooldown has not elapsed")
	}

	// 2. Vô hiệu hóa khi đã đạt chuẩn thời gian cooldown -> thành công và isUsed = true
	afterCooldown := now.Add(65 * time.Second)
	if err := otp.Invalidate(afterCooldown); err != nil {
		t.Fatalf("expected Invalidate to succeed after cooldown, got: %v", err)
	}
	if !otp.IsUsed() {
		t.Fatalf("expected otp to be marked as used after valid Invalidate")
	}
}
