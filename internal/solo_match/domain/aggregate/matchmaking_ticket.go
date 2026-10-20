package aggregate

import (
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
)

// MatchmakingTicket đại diện cho thông tin vé chờ ghép trận tạm thời trên Redis.
type MatchmakingTicket struct {
	id       string
	userID   string
	joinedAt time.Time
}

// NewMatchmakingTicket khởi tạo vé chờ ghép trận mới.
func NewMatchmakingTicket(id string, userID string, joinedAt time.Time) (*MatchmakingTicket, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, derror.ErrEmptyTicketID
	}
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return nil, derror.ErrEmptyUserID
	}
	if joinedAt.IsZero() {
		joinedAt = time.Now().UTC()
	}

	return &MatchmakingTicket{
		id:       trimmedID,
		userID:   trimmedUserID,
		joinedAt: joinedAt.UTC(),
	}, nil
}

func (t *MatchmakingTicket) ID() string {
	return t.id
}

func (t *MatchmakingTicket) UserID() string {
	return t.userID
}

func (t *MatchmakingTicket) JoinedAt() time.Time {
	return t.joinedAt
}
