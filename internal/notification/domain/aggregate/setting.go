package aggregate

import (
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
)

// Setting represents user notification preferences.
type Setting struct {
	userID          string
	enablePush      bool
	enableEmail     bool
	enableSMS       bool
	quietHoursStart string
	quietHoursEnd   string
	createdAt       time.Time
	updatedAt       time.Time
}

func NewDefaultSetting(userID string) (*Setting, error) {
	if userID == "" {
		return nil, derror.ErrEmptyUserID
	}

	now := time.Now().UTC()
	return &Setting{
		userID:          userID,
		enablePush:      true,
		enableEmail:     true,
		enableSMS:       false,
		quietHoursStart: "",
		quietHoursEnd:   "",
		createdAt:       now,
		updatedAt:       now,
	}, nil
}

func ReconstituteSetting(
	userID string,
	enablePush, enableEmail, enableSMS bool,
	quietHoursStart, quietHoursEnd string,
	createdAt, updatedAt time.Time,
) *Setting {
	return &Setting{
		userID:          userID,
		enablePush:      enablePush,
		enableEmail:     enableEmail,
		enableSMS:       enableSMS,
		quietHoursStart: quietHoursStart,
		quietHoursEnd:   quietHoursEnd,
		createdAt:       createdAt,
		updatedAt:       updatedAt,
	}
}

func (s *Setting) UserID() string          { return s.userID }
func (s *Setting) EnablePush() bool        { return s.enablePush }
func (s *Setting) EnableEmail() bool       { return s.enableEmail }
func (s *Setting) EnableSMS() bool         { return s.enableSMS }
func (s *Setting) QuietHoursStart() string { return s.quietHoursStart }
func (s *Setting) QuietHoursEnd() string   { return s.quietHoursEnd }
func (s *Setting) CreatedAt() time.Time    { return s.createdAt }
func (s *Setting) UpdatedAt() time.Time    { return s.updatedAt }

// IsInQuietHours checks if the given timestamp falls within the user's quiet hours window.
func (s *Setting) IsInQuietHours(now time.Time) bool {
	if s.quietHoursStart == "" || s.quietHoursEnd == "" || s.quietHoursStart == s.quietHoursEnd {
		return false
	}

	current := now.Format("15:04")
	if s.quietHoursStart < s.quietHoursEnd {
		return current >= s.quietHoursStart && current < s.quietHoursEnd
	}

	// Crosses midnight (e.g. 22:00 -> 07:00)
	return current >= s.quietHoursStart || current < s.quietHoursEnd
}

// Update validates quietStart and quietEnd using vo.NewTimeOfDay and updates settings.
func (s *Setting) Update(enablePush, enableEmail, enableSMS bool, quietStart, quietEnd string) error {
	startVO, err := vo.NewTimeOfDay(quietStart)
	if err != nil {
		return fmt.Errorf("invalid quiet hours start: %w", err)
	}

	endVO, err := vo.NewTimeOfDay(quietEnd)
	if err != nil {
		return fmt.Errorf("invalid quiet hours end: %w", err)
	}

	s.enablePush = enablePush
	s.enableEmail = enableEmail
	s.enableSMS = enableSMS
	s.quietHoursStart = startVO.String()
	s.quietHoursEnd = endVO.String()
	s.updatedAt = time.Now().UTC()
	return nil
}
