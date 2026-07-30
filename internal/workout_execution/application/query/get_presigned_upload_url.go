package query

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
)

// GetPresignedUploadURLQuery contains parameters to request a presigned upload URL.

type GetPresignedUploadURLQuery struct {
	FileName string

	ContentType string
}

// GetPresignedUploadURLQueryResult contains the upload URL and public file URL.

type GetPresignedUploadURLQueryResult struct {
	UploadURL string

	FileURL string

	FileName string
}

// GetPresignedUploadURLQueryHandler handles generating a presigned upload URL flexibly.

type GetPresignedUploadURLQueryHandler struct {
	storageProvider port.ObjectStorageProvider
}

// NewGetPresignedUploadURLQueryHandler constructs the query handler.

func NewGetPresignedUploadURLQueryHandler(storageProvider port.ObjectStorageProvider) *GetPresignedUploadURLQueryHandler {

	return &GetPresignedUploadURLQueryHandler{

		storageProvider: storageProvider,
	}

}

// Handle executes the GetPresignedUploadURL query in a completely dynamic and flexible way.

func (h *GetPresignedUploadURLQueryHandler) Handle(

	ctx context.Context,

	q GetPresignedUploadURLQuery,

) (*GetPresignedUploadURLQueryResult, error) {

	fileName := strings.TrimSpace(q.FileName)

	if fileName == "" {

		return nil, fmt.Errorf("get presigned upload url: %w: file_name is required", apperror.ErrInvalidInput)

	}

	// 1. Prevent Path Traversal attacks

	if strings.Contains(fileName, "..") {

		return nil, fmt.Errorf("get presigned upload url: %w: invalid file path containing '..'", apperror.ErrInvalidInput)

	}

	cleanPath := strings.TrimPrefix(path.Clean(fileName), "/")

	// 2. Dynamic extension resolution from contentType if fileName has no extension

	contentTypeInput := strings.TrimSpace(q.ContentType)

	if contentTypeInput != "" && path.Ext(cleanPath) == "" {

		if strings.HasPrefix(contentTypeInput, ".") {

			cleanPath += contentTypeInput

		} else if !strings.Contains(contentTypeInput, "/") {

			cleanPath += "." + contentTypeInput

		}

	}

	if h.storageProvider == nil {

		return nil, fmt.Errorf("get presigned upload url: %w: storage provider is not configured", apperror.ErrInternal)

	}

	uploadURL, publicFileURL, err := h.storageProvider.GeneratePresignedUploadURL(ctx, cleanPath, contentTypeInput)

	if err != nil {

		return nil, fmt.Errorf("get presigned upload url: %w", err)

	}

	return &GetPresignedUploadURLQueryResult{

		UploadURL: uploadURL,

		FileURL: publicFileURL,

		FileName: cleanPath,
	}, nil

}
