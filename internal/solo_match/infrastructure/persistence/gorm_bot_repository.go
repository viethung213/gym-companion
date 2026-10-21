package persistence

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
	"gorm.io/gorm"
)

const defaultBotCacheTTL = 30 * 24 * time.Hour // TTL 1 tháng

// GORMBotRepository quản lý danh tính bot với bộ nhớ đệm In-Memory (TTL 1 tháng)
// Tránh truy vấn DB ngẫu nhiên O(N) và giảm tải tối đa cho PostgreSQL.
type GORMBotRepository struct {
	db        *gorm.DB
	mu        sync.RWMutex
	cachedIDs []string
	cachedAt  time.Time
	cacheTTL  time.Duration
}

var _ repository.BotRepository = (*GORMBotRepository)(nil)

func NewGORMBotRepository(db *gorm.DB) *GORMBotRepository {
	return &GORMBotRepository{
		db:       db,
		cacheTTL: defaultBotCacheTTL,
	}
}

// SetCacheTTL cho phép cấu hình thời hạn cache (dùng trong kiểm thử hoặc cấu hình riêng)
func (r *GORMBotRepository) SetCacheTTL(ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheTTL = ttl
}

// IsBot kiểm tra xem userID có phải tài khoản bot hệ thống không
func (r *GORMBotRepository) IsBot(ctx context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}

	// 1. Kiểm tra nhanh qua in-memory cache nếu còn hiệu lực
	r.mu.RLock()
	if len(r.cachedIDs) > 0 && time.Since(r.cachedAt) < r.cacheTTL {
		for _, id := range r.cachedIDs {
			if id == userID {
				r.mu.RUnlock()
				return true, nil
			}
		}
		r.mu.RUnlock()
		return false, nil
	}
	r.mu.RUnlock()

	// 2. Fallback query DB nếu cache chưa sẵn sàng
	var count int64
	err := r.db.WithContext(ctx).Model(&SystemBotModel{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check is bot: %w", err)
	}
	return count > 0, nil
}

// GetRandomBotUserID lấy ngẫu nhiên 1 bot ID từ memory cache (O(1)).
// Nếu memory trống hoặc hết hạn TTL (1 tháng), hàm sẽ nạp lại từ DB và lưu vào memory cache.
func (r *GORMBotRepository) GetRandomBotUserID(ctx context.Context) (string, error) {
	// 1. Đọc từ In-Memory Cache (O(1) - Không truy vấn DB)
	r.mu.RLock()
	if len(r.cachedIDs) > 0 && time.Since(r.cachedAt) < r.cacheTTL {
		chosen := r.cachedIDs[rand.IntN(len(r.cachedIDs))]
		r.mu.RUnlock()
		return chosen, nil
	}
	r.mu.RUnlock()

	// 2. Cache miss hoặc hết hạn TTL -> Nạp lại từ Database
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double check sau khi giữ Lock (tránh race condition khi nhiều goroutine cùng nạp)
	if len(r.cachedIDs) > 0 && time.Since(r.cachedAt) < r.cacheTTL {
		return r.cachedIDs[rand.IntN(len(r.cachedIDs))], nil
	}

	var botIDs []string
	err := r.db.WithContext(ctx).Model(&SystemBotModel{}).
		Where("is_active = ?", true).
		Pluck("user_id", &botIDs).Error
	if err != nil {
		return "", fmt.Errorf("reload active bot ids from db: %w", err)
	}

	if len(botIDs) == 0 {
		return "", derror.ErrNoBotAvailable
	}

	// Cập nhật bộ nhớ đệm server
	r.cachedIDs = botIDs
	r.cachedAt = time.Now().UTC()

	return r.cachedIDs[rand.IntN(len(r.cachedIDs))], nil
}
