package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/event"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestAddCommentHandler(t *testing.T) {
	feedItemRepo := &mockFeedItemRepo{items: make(map[string]*aggregate.FeedItem)}
	p, _ := aggregate.NewPostItem("f-1", "u-author", "Test post", nil, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
	_ = feedItemRepo.Create(context.Background(), p)

	interactionRepo := &mockInteractionRepo{
		reactions: make(map[string]*entity.Reaction),
		comments:  make(map[string]*entity.Comment),
	}
	tx := &mockTxManager{}
	pub := &mockEventPublisher{}
	uInfo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"u-viewer": entity.NewUserSnapshot("u-viewer", "Commenter User", "https://example.com/commenter.png", "user", time.Time{}),
		},
	}

	handler := NewAddCommentHandler(interactionRepo, feedItemRepo, uInfo, pub, tx)

	res, err := handler.Handle(context.Background(), AddCommentCommand{
		UserID:     "u-viewer",
		FeedItemID: "f-1",
		Content:    "Awesome form!",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Comment.Content() != "Awesome form!" {
		t.Errorf("unexpected comment content")
	}
	if p.CommentCount() != 1 {
		t.Errorf("expected post comment count 1, got %d", p.CommentCount())
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 event published on comment, got %d", len(pub.published))
	}
	commentEv, ok := pub.published[0].(*event.PostCommentedEvent)
	if !ok {
		t.Fatalf("expected *event.PostCommentedEvent, got %T", pub.published[0])
	}
	if commentEv.PostOwnerID != "u-author" || commentEv.UserName != "Commenter User" || commentEv.Content != "Awesome form!" {
		t.Errorf("unexpected comment event content: %+v", commentEv)
	}

	// Validation tests:
	// 1. Empty UserID
	_, err = handler.Handle(context.Background(), AddCommentCommand{
		UserID:     "",
		FeedItemID: "f-1",
		Content:    "Hello",
	})
	if !errors.Is(err, derror.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for empty UserID, got %v", err)
	}

	// 2. Empty FeedItemID
	_, err = handler.Handle(context.Background(), AddCommentCommand{
		UserID:     "u-viewer",
		FeedItemID: "",
		Content:    "Hello",
	})
	if !errors.Is(err, derror.ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound for empty FeedItemID, got %v", err)
	}

	// 3. FeedItem not found
	_, err = handler.Handle(context.Background(), AddCommentCommand{
		UserID:     "u-viewer",
		FeedItemID: "non-existent-feed",
		Content:    "Hello",
	})
	if !errors.Is(err, derror.ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound for missing feed item, got %v", err)
	}

	// 4. User not found in snapshot repo
	_, err = handler.Handle(context.Background(), AddCommentCommand{
		UserID:     "non-existent-user",
		FeedItemID: "f-1",
		Content:    "Hello",
	})
	if !errors.Is(err, derror.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for missing user, got %v", err)
	}

	// 5. Parent comment not found
	badParent := "bad-parent-id"
	_, err = handler.Handle(context.Background(), AddCommentCommand{
		UserID:     "u-viewer",
		FeedItemID: "f-1",
		Content:    "Replying...",
		ParentID:   &badParent,
	})
	if !errors.Is(err, derror.ErrCommentNotFound) {
		t.Errorf("expected ErrCommentNotFound for missing parent, got %v", err)
	}

	// 6. Parent comment belongs to DIFFERENT feed item
	parentOnOtherPost, _ := entity.NewComment("c-other", "u-viewer", "other-feed", nil, "Other", time.Time{}, time.Time{})
	interactionRepo.comments["c-other"] = parentOnOtherPost
	parentOtherID := "c-other"
	_, err = handler.Handle(context.Background(), AddCommentCommand{
		UserID:     "u-viewer",
		FeedItemID: "f-1",
		Content:    "Replying across posts...",
		ParentID:   &parentOtherID,
	})
	if !errors.Is(err, derror.ErrCommentNotFound) {
		t.Errorf("expected ErrCommentNotFound for cross-post reply, got %v", err)
	}

	// 7. Successful reply comment populates ParentCommentOwnerID
	parentComment, _ := entity.NewComment("c-root", "u-parent-author", "f-1", nil, "Root comment", time.Time{}, time.Time{})
	interactionRepo.comments["c-root"] = parentComment
	parentID := "c-root"
	pub.published = nil

	replyRes, err := handler.Handle(context.Background(), AddCommentCommand{
		UserID:     "u-viewer",
		FeedItemID: "f-1",
		Content:    "This is a reply",
		ParentID:   &parentID,
	})
	if err != nil {
		t.Fatalf("unexpected error on reply comment: %v", err)
	}
	if replyRes.Comment.ParentID() == nil || *replyRes.Comment.ParentID() != "c-root" {
		t.Errorf("expected parent ID 'c-root'")
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 event published, got %d", len(pub.published))
	}
	replyEv, ok := pub.published[0].(*event.PostCommentedEvent)
	if !ok {
		t.Fatalf("expected *event.PostCommentedEvent, got %T", pub.published[0])
	}
	if replyEv.ParentCommentOwnerID == nil || *replyEv.ParentCommentOwnerID != "u-parent-author" {
		t.Errorf("expected ParentCommentOwnerID 'u-parent-author', got %v", replyEv.ParentCommentOwnerID)
	}
}
