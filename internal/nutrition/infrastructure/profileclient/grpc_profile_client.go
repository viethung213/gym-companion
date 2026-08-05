package profileclient

import (
	"context"
	"fmt"

	profilev1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/message"
	profilev1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ repository.ProfileClient = (*GRPCProfileClient)(nil)

// GRPCProfileClient là adapter triển khai repository.ProfileClient
// bằng cách gọi gRPC in-process đến ProfileService.
type GRPCProfileClient struct {
	client profilev1service.ProfileServiceClient
}

// NewGRPCProfileClient khởi tạo GRPCProfileClient.
func NewGRPCProfileClient(conn grpc.ClientConnInterface) *GRPCProfileClient {
	return &GRPCProfileClient{
		client: profilev1service.NewProfileServiceClient(conn),
	}
}

// GetBiologicalMetrics lấy dữ liệu sinh trắc học từ ProfileService.
// Trả về nil, nil nếu user chưa có profile.
func (c *GRPCProfileClient) GetBiologicalMetrics(
	ctx context.Context,
	userID string,
) (*repository.UserBiologicalMetrics, error) {
	resp, err := c.client.GetProfile(ctx, &profilev1message.GetProfileRequest{
		UserId: userID,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("profile client get biological metrics user %s: %w", userID, err)
	}

	return &repository.UserBiologicalMetrics{
		WeightKg:      float64(resp.GetWeightKg()),
		HeightCm:      float64(resp.GetHeightCm()),
		Age:           int(resp.GetAge()),
		Gender:        resp.GetGender(),
		ActivityLevel: mapExperienceLevelToActivityLevel(resp.GetExperienceLevel()),
	}, nil
}

// mapExperienceLevelToActivityLevel chuyển đổi trình độ tập luyện của Profile
// sang hệ số hoạt động dùng trong công thức tính TDEE (Mifflin-St Jeor).
//
// Mapping:
//   - BEGINNER    → LIGHTLY_ACTIVE    (×1.375)
//   - INTERMEDIATE → MODERATELY_ACTIVE (×1.55)
//   - ADVANCED    → VERY_ACTIVE       (×1.725)
//   - khác/rỗng   → MODERATELY_ACTIVE (mặc định an toàn)
func mapExperienceLevelToActivityLevel(experienceLevel string) string {
	switch experienceLevel {
	case "BEGINNER":
		return "LIGHTLY_ACTIVE"
	case "INTERMEDIATE":
		return "MODERATELY_ACTIVE"
	case "ADVANCED":
		return "VERY_ACTIVE"
	default:
		return "MODERATELY_ACTIVE"
	}
}
