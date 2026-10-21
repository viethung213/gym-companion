package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/solo_match/application/dto"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
)

type GetUserMatchHistoryQuery struct {
	UserID   string
	PageSize int
	Offset   int
}

type GetUserMatchHistoryHandler struct {
	matchRepo    repository.SoloMatchRepository
	snapshotRepo repository.UserSnapshotRepository
}

func NewGetUserMatchHistoryHandler(
	matchRepo repository.SoloMatchRepository,
	snapshotRepo repository.UserSnapshotRepository,
) *GetUserMatchHistoryHandler {
	return &GetUserMatchHistoryHandler{
		matchRepo:    matchRepo,
		snapshotRepo: snapshotRepo,
	}
}

func (h *GetUserMatchHistoryHandler) Handle(ctx context.Context, q GetUserMatchHistoryQuery) ([]dto.SoloMatchDTO, error) {
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 50 {
		pageSize = 50
	}

	matches, err := h.matchRepo.ListByUserID(ctx, q.UserID, q.Offset, pageSize)
	if err != nil {
		return nil, fmt.Errorf("list matches for user %s: %w", q.UserID, err)
	}

	if len(matches) == 0 {
		return []dto.SoloMatchDTO{}, nil
	}

	// Gom toàn bộ userIDs để fetch snapshot một lần (tránh N+1)
	userIDMap := make(map[string]struct{})
	for _, m := range matches {
		for _, p := range m.Participants() {
			userIDMap[p.UserID()] = struct{}{}
		}
	}

	userIDs := make([]string, 0, len(userIDMap))
	for uid := range userIDMap {
		userIDs = append(userIDs, uid)
	}

	snapshots, err := h.snapshotRepo.FindByUserIDs(ctx, userIDs)
	if err != nil {
		snapshots = make(map[string]*entity.UserSnapshot)
	}

	dtos := make([]dto.SoloMatchDTO, 0, len(matches))
	for _, m := range matches {
		dtos = append(dtos, dto.ToSoloMatchDTO(m, snapshots))
	}

	return dtos, nil
}
