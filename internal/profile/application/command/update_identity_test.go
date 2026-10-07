//go:build unit

package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/profile/application/command"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type failEventPub struct{}

func (f *failEventPub) PublishEvents(ctx context.Context, events []any) error {
	return errors.New("publish events failed")
}

func TestUpdateIdentityHandler_Success(t *testing.T) {
	repo := newMockRepo()
	eventPub := &mockEventPub{}
	txManager := &mockTxManager{}

	bio, _ := vo.NewBiologicalMetrics(70, 170, 25, "MALE")
	profile, err := aggregate.NewUserProfile("user-identity-1", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), profile))

	handler := command.NewUpdateIdentityHandler(repo, eventPub, txManager)

	newName := "Nguyen Van B"
	newAvatar := "https://example.com/avatar2.jpg"
	cmd := command.UpdateIdentityCommand{
		UserID:    "user-identity-1",
		FullName:  &newName,
		AvatarURL: &newAvatar,
	}

	res, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	assert.Equal(t, "user-identity-1", res.UserID)
	assert.Equal(t, newName, res.FullName)
	assert.Equal(t, newAvatar, res.AvatarURL)

	// Verify profile in repo was updated
	saved, err := repo.FindByUserID(context.Background(), "user-identity-1")
	require.NoError(t, err)
	assert.Equal(t, newName, saved.FullName())
	assert.Equal(t, newAvatar, saved.AvatarURL())

	// Verify event was published
	assert.NotEmpty(t, eventPub.published)
}

func TestUpdateIdentityHandler_Errors(t *testing.T) {
	repo := newMockRepo()
	eventPub := &mockEventPub{}
	txManager := &mockTxManager{}

	bio, _ := vo.NewBiologicalMetrics(70, 170, 25, "MALE")
	profile, err := aggregate.NewUserProfile("user-identity-err", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), profile))

	handler := command.NewUpdateIdentityHandler(repo, eventPub, txManager)

	t.Run("Profile not found", func(t *testing.T) {
		name := "Sample Name"
		cmd := command.UpdateIdentityCommand{
			UserID:   "non-existent-user",
			FullName: &name,
		}
		_, err := handler.Handle(context.Background(), cmd)
		assert.ErrorIs(t, err, derror.ErrProfileNotFound)
	})

	t.Run("Empty update inputs", func(t *testing.T) {
		cmd := command.UpdateIdentityCommand{
			UserID: "user-identity-err",
		}
		_, err := handler.Handle(context.Background(), cmd)
		assert.Error(t, err)
	})

	t.Run("Transaction failure", func(t *testing.T) {
		failHandler := command.NewUpdateIdentityHandler(repo, eventPub, &failTxManager{})
		name := "Valid Name"
		cmd := command.UpdateIdentityCommand{
			UserID:   "user-identity-err",
			FullName: &name,
		}
		_, err := failHandler.Handle(context.Background(), cmd)
		assert.Error(t, err)
	})

	t.Run("Publish events failure", func(t *testing.T) {
		failPubHandler := command.NewUpdateIdentityHandler(repo, &failEventPub{}, txManager)
		name := "Valid New Name"
		cmd := command.UpdateIdentityCommand{
			UserID:   "user-identity-err",
			FullName: &name,
		}
		_, err := failPubHandler.Handle(context.Background(), cmd)
		assert.Error(t, err)
	})
}
