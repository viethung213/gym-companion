# Kế Hoạch Triển Khai Kỹ Thuật (Detailed Implementation Plan): Bảng Xếp Hạng Toàn Hệ Thống

## 1. Thiết Kế Mã Nguồn Chi Tiết (Low-Level Code Design)

### 1.1. Tầng Application (Query & Output Ports)

#### `internal/gamification/application/port/leaderboard_repository.go`
```go
package port

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type LeaderboardUserRecord struct {
	UserID    uuid.UUID
	XP        int64
	Level     int32
	UpdatedAt time.Time
}

// LeaderboardRepository định nghĩa các truy vấn đọc dữ liệu phục vụ xếp hạng
type LeaderboardRepository interface {
	// GetTopUsers lấy danh sách top N người dùng có điểm XP cao nhất
	GetTopUsers(ctx context.Context, limit int) ([]LeaderboardUserRecord, error)

	// GetUserRank tính toán thứ hạng chính xác của một người dùng bất kỳ
	GetUserRank(ctx context.Context, userID uuid.UUID, xp int64, updatedAt time.Time) (int64, error)

	// GetUserXpRecord lấy thông tin bản ghi XP của người dùng hiện tại
	GetUserXpRecord(ctx context.Context, userID uuid.UUID) (*LeaderboardUserRecord, error)
}
```

#### `internal/gamification/application/port/profile_reader.go`
```go
package port

import (
	"context"

	"github.com/google/uuid"
)

type ProfileSummary struct {
	DisplayName string
	AvatarURL   string
}

// ProfileReaderPort định nghĩa cổng giao tiếp in-process sang module profile
type ProfileReaderPort interface {
	BatchGetSummaries(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]ProfileSummary, error)
}
```

#### `internal/gamification/application/query/get_global_leaderboard.go`
```go
package query

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
)

type GetGlobalLeaderboardQuery struct {
	CurrentUserID uuid.UUID
	Limit         int
}

type LeaderboardEntryDTO struct {
	Rank        int64
	UserID      uuid.UUID
	DisplayName string
	AvatarURL   string
	XP          int64
	Level       int32
}

type GetGlobalLeaderboardResult struct {
	Items      []LeaderboardEntryDTO
	MyStanding LeaderboardEntryDTO
}

type GetGlobalLeaderboardHandler struct {
	repo          port.LeaderboardRepository
	profileReader port.ProfileReaderPort
}

func NewGetGlobalLeaderboardHandler(
	repo port.LeaderboardRepository,
	profileReader port.ProfileReaderPort,
) *GetGlobalLeaderboardHandler {
	return &GetGlobalLeaderboardHandler{
		repo:          repo,
		profileReader: profileReader,
	}
}

func (h *GetGlobalLeaderboardHandler) Handle(
	ctx context.Context,
	q GetGlobalLeaderboardQuery,
) (*GetGlobalLeaderboardResult, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	// 1. Lấy danh sách Top N
	topUsers, err := h.repo.GetTopUsers(ctx, limit)
	if err != nil {
		return nil, err
	}

	// 2. Xác định thứ hạng và thông tin của người dùng hiện tại
	var myStandingRecord *port.LeaderboardUserRecord
	var myRank int64 = -1

	for idx, u := range topUsers {
		if u.UserID == q.CurrentUserID {
			myRank = int64(idx + 1)
			recordCopy := u
			myStandingRecord = &recordCopy
			break
		}
	}

	if myRank == -1 {
		record, err := h.repo.GetUserXpRecord(ctx, q.CurrentUserID)
		if err != nil {
			return nil, err
		}
		if record == nil {
			// Người dùng mới hoàn toàn chưa có XP
			myStandingRecord = &port.LeaderboardUserRecord{
				UserID:    q.CurrentUserID,
				XP:        0,
				Level:     1,
				UpdatedAt: time.Now(),
			}
			// Tính hạng cuối
			rank, err := h.repo.GetUserRank(ctx, q.CurrentUserID, 0, myStandingRecord.UpdatedAt)
			if err != nil {
				return nil, err
			}
			myRank = rank
		} else {
			myStandingRecord = record
			rank, err := h.repo.GetUserRank(ctx, q.CurrentUserID, record.XP, record.UpdatedAt)
			if err != nil {
				return nil, err
			}
			myRank = rank
		}
	}

	// 3. Gom danh sách UserIDs để nạp profile qua Port nội bộ
	uniqueIDs := make([]uuid.UUID, 0, len(topUsers)+1)
	idMap := make(map[uuid.UUID]struct{})
	for _, u := range topUsers {
		if _, exists := idMap[u.UserID]; !exists {
			idMap[u.UserID] = struct{}{}
			uniqueIDs = append(uniqueIDs, u.UserID)
		}
	}
	if _, exists := idMap[q.CurrentUserID]; !exists {
		uniqueIDs = append(uniqueIDs, q.CurrentUserID)
	}

	// 4. Gọi ProfileReaderPort có timeout 500ms và fallback an toàn
	profileMap := make(map[uuid.UUID]port.ProfileSummary)
	if len(uniqueIDs) > 0 && h.profileReader != nil {
		pCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		summaries, pErr := h.profileReader.BatchGetSummaries(pCtx, uniqueIDs)
		cancel()
		if pErr == nil && summaries != nil {
			profileMap = summaries
		}
		// Nếu lỗi hoặc timeout, profileMap để trống -> fallback tự động
	}

	// Helper gán fallback
	getProfile := func(uid uuid.UUID) (string, string) {
		if p, ok := profileMap[uid]; ok && p.DisplayName != "" {
			return p.DisplayName, p.AvatarURL
		}
		return "Gymer", ""
	}

	// 5. Đóng gói kết quả DTO
	items := make([]LeaderboardEntryDTO, len(topUsers))
	for idx, u := range topUsers {
		name, avatar := getProfile(u.UserID)
		items[idx] = LeaderboardEntryDTO{
			Rank:        int64(idx + 1),
			UserID:      u.UserID,
			DisplayName: name,
			AvatarURL:   avatar,
			XP:          u.XP,
			Level:       u.Level,
		}
	}

	myName, myAvatar := getProfile(q.CurrentUserID)
	myStanding := LeaderboardEntryDTO{
		Rank:        myRank,
		UserID:      q.CurrentUserID,
		DisplayName: myName,
		AvatarURL:   myAvatar,
		XP:          myStandingRecord.XP,
		Level:       myStandingRecord.Level,
	}

	return &GetGlobalLeaderboardResult{
		Items:      items,
		MyStanding: myStanding,
	}, nil
}
```

---

### 1.2. Tầng Infrastructure (Persistence & Adapter)

#### `internal/gamification/infrastructure/persistence/leaderboard_repository.go`
```go
package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"gorm.io/gorm"
)

type PostgresLeaderboardRepository struct {
	db *gorm.DB
}

func NewPostgresLeaderboardRepository(db *gorm.DB) *PostgresLeaderboardRepository {
	return &PostgresLeaderboardRepository{db: db}
}

func (r *PostgresLeaderboardRepository) GetTopUsers(ctx context.Context, limit int) ([]port.LeaderboardUserRecord, error) {
	var rows []struct {
		UserID    uuid.UUID `gorm:"column:user_id"`
		XP        int64     `gorm:"column:xp"`
		Level     int32     `gorm:"column:level"`
		UpdatedAt time.Time `gorm:"column:updated_at"`
	}

	err := r.db.WithContext(ctx).
		Table("gamification.user_xp").
		Select("user_id, xp, level, updated_at").
		Order("xp DESC, updated_at ASC, user_id ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	results := make([]port.LeaderboardUserRecord, len(rows))
	for i, row := range rows {
		results[i] = port.LeaderboardUserRecord{
			UserID:    row.UserID,
			XP:        row.XP,
			Level:     row.Level,
			UpdatedAt: row.UpdatedAt,
		}
	}
	return results, nil
}

func (r *PostgresLeaderboardRepository) GetUserRank(ctx context.Context, userID uuid.UUID, xp int64, updatedAt time.Time) (int64, error) {
	var count int64
	// Đếm số người có điểm cao hơn, hoặc bằng điểm nhưng đạt sớm hơn (updated_at nhỏ hơn)
	query := `
		SELECT COUNT(*) 
		FROM gamification.user_xp 
		WHERE xp > ? 
		   OR (xp = ? AND updated_at < ?)
		   OR (xp = ? AND updated_at = ? AND user_id < ?)
	`
	err := r.db.WithContext(ctx).
		Raw(query, xp, xp, updatedAt, xp, updatedAt, userID).
		Scan(&count).Error
	if err != nil {
		return 0, err
	}
	return count + 1, nil
}

func (r *PostgresLeaderboardRepository) GetUserXpRecord(ctx context.Context, userID uuid.UUID) (*port.LeaderboardUserRecord, error) {
	var row struct {
		UserID    uuid.UUID `gorm:"column:user_id"`
		XP        int64     `gorm:"column:xp"`
		Level     int32     `gorm:"column:level"`
		UpdatedAt time.Time `gorm:"column:updated_at"`
	}

	err := r.db.WithContext(ctx).
		Table("gamification.user_xp").
		Select("user_id, xp, level, updated_at").
		Where("user_id = ?", userID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &port.LeaderboardUserRecord{
		UserID:    row.UserID,
		XP:        row.XP,
		Level:     row.Level,
		UpdatedAt: row.UpdatedAt,
	}, nil
}
```

#### `internal/gamification/infrastructure/adapter/profile_adapter.go`
```go
package adapter

import (
	"context"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
)

// ProfileModuleFacade định nghĩa interface công khai mà module profile cung cấp
type ProfileModuleFacade interface {
	BatchGetProfiles(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]port.ProfileSummary, error)
}

type InProcessProfileReaderAdapter struct {
	facade ProfileModuleFacade
}

func NewInProcessProfileReaderAdapter(facade ProfileModuleFacade) *InProcessProfileReaderAdapter {
	return &InProcessProfileReaderAdapter{facade: facade}
}

func (a *InProcessProfileReaderAdapter) BatchGetSummaries(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]port.ProfileSummary, error) {
	if a.facade == nil {
		return nil, nil
	}
	return a.facade.BatchGetProfiles(ctx, userIDs)
}
```

---

## 2. Kế Hoạch Kiểm Thử TDD (Testing Strategy)

### 2.1. Unit Test (`get_global_leaderboard_test.go`)
* **Test Case 1: Happy Path - Người dùng nằm trong Top 50**:
  - Mock repo trả về 10 người dùng, trong đó người dùng gọi API nằm ở vị trí thứ 3.
  - Kỳ vọng: `my_standing.rank = 3`, `items[2].user_id == current_user_id`. Không gọi `GetUserRank` (tối ưu hóa).
* **Test Case 2: Người dùng nằm ngoài Top 50**:
  - Mock repo trả về Top 50 (không có `current_user_id`).
  - Mock `GetUserRank` trả về 75.
  - Kỳ vọng: `len(items) == 50`, `my_standing.rank = 75`.
* **Test Case 3: Người dùng mới chưa có bản ghi (0 XP)**:
  - Mock `GetUserXpRecord` trả về `nil`.
  - Kỳ vọng: `my_standing.XP = 0`, `my_standing.Level = 1`, tính hạng chính xác mà không báo lỗi `NotFound`.
* **Test Case 4: Module Profile bị lỗi / Timeout**:
  - Mock `profileReader` trả về lỗi hoặc timeout.
  - Kỳ vọng: Request vẫn trả về `200 OK`, tên hiển thị fallback về `"Gymer"`, `avatar_url = ""`.
* **Test Case 5: Giới hạn Limit (Clamp)**:
  - Request `limit = 0` $\rightarrow$ Tự động dùng `50`.
  - Request `limit = 200` $\rightarrow$ Tự động cắt về `100`.

### 2.2. Integration Test (`leaderboard_repository_test.go`)
* Chạy với PostgreSQL thật (qua Docker / Testcontainers):
  - Chèn 100 bản ghi `user_xp` với điểm số khác nhau.
  - Kiểm tra `GetTopUsers(ctx, 50)` trả về đúng 50 người dẫn đầu theo thứ tự `xp DESC`.
  - Kiểm tra quy tắc hòa điểm: 2 user cùng $1,000$ XP, user có `updated_at` sớm hơn phải đứng trước.
  - Kiểm tra `GetUserRank`: User ở hạng 75 phải trả về chính xác `rank = 75`.
