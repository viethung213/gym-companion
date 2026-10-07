package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/event"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type CreatePostCommand struct {
	UserID     string
	Caption    string
	MediaURLs  []string
	Visibility string
}

func (c CreatePostCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return derror.ErrUnauthorized
	}
	if strings.TrimSpace(c.Caption) == "" && len(c.MediaURLs) == 0 {
		return derror.ErrEmptyContent
	}
	return nil
}

type CreatePostResult struct {
	Item *aggregate.FeedItem
}

type CreatePostHandler struct {
	feedItemRepo     repository.FeedItemRepository
	followRepo       repository.FollowRepository
	userSnapshotRepo repository.UserSnapshotRepository
	eventPub         port.EventPublisher
	txManager        port.TransactionManager
}

type CreatePostHandlerOption func(*CreatePostHandler)

func WithCreatePostFollowRepo(r repository.FollowRepository) CreatePostHandlerOption {
	return func(h *CreatePostHandler) { h.followRepo = r }
}

func WithCreatePostUserSnapshotRepo(r repository.UserSnapshotRepository) CreatePostHandlerOption {
	return func(h *CreatePostHandler) { h.userSnapshotRepo = r }
}

func NewCreatePostHandler(
	feedItemRepo repository.FeedItemRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
	opts ...CreatePostHandlerOption,
) *CreatePostHandler {
	h := &CreatePostHandler{
		feedItemRepo: feedItemRepo,
		eventPub:     eventPub,
		txManager:    txManager,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *CreatePostHandler) Handle(ctx context.Context, cmd CreatePostCommand) (*CreatePostResult, error) {
	if err := cmd.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	item, createErr := aggregate.NewPostItem(
		uuid.New().String(),
		cmd.UserID,
		cmd.Caption,
		cmd.MediaURLs,
		vo.NewVisibility(cmd.Visibility),
		0,
		0,
		now,
		now,
	)
	if createErr != nil {
		return nil, fmt.Errorf("create post item aggregate: %w", createErr)
	}

	txErr := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.feedItemRepo.Create(txCtx, item); err != nil {
			return fmt.Errorf("persist post item: %w", err)
		}

		var authorName, authorAvatarURL string
		if h.userSnapshotRepo != nil {
			if u, getErr := h.userSnapshotRepo.GetByID(txCtx, cmd.UserID); getErr == nil && u != nil {
				authorName = u.FullName()
				authorAvatarURL = u.AvatarURL()
			}
		}

		var followerUserIDs []string
		if h.followRepo != nil && item.Visibility() != vo.VisibilityPrivate {
			followers, _, _, getFollowersErr := h.followRepo.GetFollowers(txCtx, cmd.UserID, 500, "")
			if getFollowersErr == nil {
				followerUserIDs = make([]string, 0, len(followers))
				for _, f := range followers {
					followerUserIDs = append(followerUserIDs, f.FollowerID())
				}
			}
		}

		if h.eventPub != nil {
			ev := &event.PostCreatedEvent{
				PostID:          item.ID(),
				UserID:          cmd.UserID,
				UserName:        authorName,
				UserAvatarURL:   authorAvatarURL,
				Content:         cmd.Caption,
				MediaURLs:       cmd.MediaURLs,
				Visibility:      string(item.Visibility()),
				FollowerUserIDs: followerUserIDs,
				CreatedAt:       item.CreatedAt(),
			}
			if err := h.eventPub.PublishEvents(txCtx, []any{ev}); err != nil {
				return fmt.Errorf("publish post created event: %w", err)
			}
		}

		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	return &CreatePostResult{Item: item}, nil
}
