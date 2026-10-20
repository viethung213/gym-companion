package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/solo_match/application/dto"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
)

type GetSoloMatchByIDQuery struct {
	MatchID string
}

type GetSoloMatchByIDHandler struct {
	matchRepo    repository.SoloMatchRepository
	snapshotRepo repository.UserSnapshotRepository
}

func NewGetSoloMatchByIDHandler(
	matchRepo repository.SoloMatchRepository,
	snapshotRepo repository.UserSnapshotRepository,
) *GetSoloMatchByIDHandler {
	return &GetSoloMatchByIDHandler{
		matchRepo:    matchRepo,
		snapshotRepo: snapshotRepo,
	}
}

func (h *GetSoloMatchByIDHandler) Handle(ctx context.Context, q GetSoloMatchByIDQuery) (*dto.SoloMatchDTO, error) {
	match, err := h.matchRepo.FindByID(ctx, q.MatchID)
	if err != nil {
		return nil, fmt.Errorf("find match %s: %w", q.MatchID, err)
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
