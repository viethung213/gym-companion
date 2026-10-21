package derror

import "errors"

var (
	ErrSelfFollow          = errors.New("cannot follow yourself")
	ErrPostNotFound        = errors.New("post not found")
	ErrActivityNotFound    = errors.New("activity not found")
	ErrCommentNotFound     = errors.New("comment not found")
	ErrUnauthorized        = errors.New("unauthorized action on social resource")
	ErrEmptyContent        = errors.New("content cannot be empty")
	ErrEmptySearchQuery    = errors.New("search query or author_id is required")
	ErrSearchQueryTooShort = errors.New("search query must be at least 2 characters")
	ErrInvalidUserID       = errors.New("invalid user id format: must be a valid UUID")
	ErrInvalidTargetType   = errors.New("invalid target type: must be POST or ACTIVITY")
	ErrInvalidReactionType = errors.New("invalid reaction type")
)
