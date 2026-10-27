package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/solo_match/application/dto"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
)

type GetActiveMatchQuery struct {
	UserID string
}

type GetActiveMatchHandler struct {
	matchRepo    repository.SoloMatchRepository
	snapshotRepo repository.UserSnapshotRepository
}

func NewGetActiveMatchHandler(
	matchRepo repository.SoloMatchRepository,
	snapshotRepo repository.UserSnapshotRepository,
) *GetActiveMatchHandler {
	return &GetActiveMatchHandler{
		matchRepo:    matchRepo,
		snapshotRepo: snapshotRepo,
	}
}

func (h *GetActiveMatchHandler) Handle(ctx context.Context, q GetActiveMatchQuery) (*dto.SoloMatchDTO, error) {
	match, err := h.matchRepo.FindActiveByUserID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("find active match for user %s: %w", q.UserID, err)
	}
	if match == nil {
		return nil, nil
	}

	userIDs := make([]string, 0, len(match.Participants()))
	for _, p := range match.Participants() {
		userIDs = append(userIDs, p.UserID())
	}

	snapshots, err := h.snapshotRepo.FindByUserIDs(ctx, userIDs)
	if err != nil {
		snapshots = make(map[string]*entity.UserSnapshot)
	}

	matchDTO := dto.ToSoloMatchDTO(match, snapshots)
	return &matchDTO, nil
}
