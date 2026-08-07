package derror_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
)

func TestDomainErrors(t *testing.T) {
	t.Parallel()

	if derror.ErrDeviceNotFound == nil {
		t.Error("ErrDeviceNotFound should not be nil")
	}
	if derror.ErrSettingNotFound == nil {
		t.Error("ErrSettingNotFound should not be nil")
	}
	if derror.ErrNotificationNotFound == nil {
		t.Error("ErrNotificationNotFound should not be nil")
	}
	if derror.ErrEmptyUserID == nil {
		t.Error("ErrEmptyUserID should not be nil")
	}
	if derror.ErrEmptyDeviceToken == nil {
		t.Error("ErrEmptyDeviceToken should not be nil")
	}
	if derror.ErrEmptyTitle == nil {
		t.Error("ErrEmptyTitle should not be nil")
	}
}
