package repository

import "context"

// UserBiologicalMetrics chứa dữ liệu sinh trắc học cần thiết để tính TDEE, kèm múi giờ
// của người dùng (meal times và plan_date là giờ/ngày theo múi giờ này).
type UserBiologicalMetrics struct {
	WeightKg      float64
	HeightCm      float64
	Age           int
	Gender        string // "MALE", "FEMALE"
	ActivityLevel string // "SEDENTARY", "LIGHTLY_ACTIVE", "MODERATELY_ACTIVE", "VERY_ACTIVE"
	Timezone      string // IANA zone, e.g. "Asia/Ho_Chi_Minh"; empty = UTC
}

// ProfileClient là port để lấy dữ liệu sinh trắc học của người dùng từ Profile module.
// Interface được định nghĩa tại nơi sử dụng (Nutrition domain) theo nguyên tắc Hexagonal Architecture.
type ProfileClient interface {
	// GetBiologicalMetrics lấy chỉ số sinh trắc học của user phục vụ tính toán TDEE.
	// Trả về nil, nil nếu user chưa có profile.
	GetBiologicalMetrics(ctx context.Context, userID string) (*UserBiologicalMetrics, error)
}
