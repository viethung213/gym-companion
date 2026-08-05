package repository

import "context"

// UserBiologicalMetrics chứa dữ liệu sinh trắc học cần thiết để tính TDEE.
type UserBiologicalMetrics struct {
	WeightKg      float64
	HeightCm      float64
	Age           int
	Gender        string // "MALE", "FEMALE"
	ActivityLevel string // "SEDENTARY", "LIGHTLY_ACTIVE", "MODERATELY_ACTIVE", "VERY_ACTIVE"
}

// ProfileClient là port để lấy dữ liệu sinh trắc học của người dùng từ Profile module.
// Interface được định nghĩa tại nơi sử dụng (Nutrition domain) theo nguyên tắc Hexagonal Architecture.
type ProfileClient interface {
	// GetBiologicalMetrics lấy chỉ số sinh trắc học của user phục vụ tính toán TDEE.
	// Trả về nil, nil nếu user chưa có profile.
	GetBiologicalMetrics(ctx context.Context, userID string) (*UserBiologicalMetrics, error)
}
