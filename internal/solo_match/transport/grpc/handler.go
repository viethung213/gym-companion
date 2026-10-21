package grpc

import (
	"context"
	"errors"
	"time"

	solomatchv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/solo_match/v1/message"
	solomatchv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/solo_match/v1/service"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/application/dto"
	"github.com/viethung213/gym-companion/internal/solo_match/application/query"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	solomatchv1service.UnimplementedSoloMatchServiceServer
	joinQueueHandler   *command.JoinMatchmakingQueueHandler
	cancelQueueHandler *command.CancelMatchmakingHandler
	getMatchHandler    *query.GetSoloMatchByIDHandler
	getHistoryHandler  *query.GetUserMatchHistoryHandler
	snapshotRepo       repository.UserSnapshotRepository
}

var _ solomatchv1service.SoloMatchServiceServer = (*Handler)(nil)

func NewHandler(
	joinQueueHandler *command.JoinMatchmakingQueueHandler,
	cancelQueueHandler *command.CancelMatchmakingHandler,
	getMatchHandler *query.GetSoloMatchByIDHandler,
	getHistoryHandler *query.GetUserMatchHistoryHandler,
	snapshotRepo repository.UserSnapshotRepository,
) *Handler {
	return &Handler{
		joinQueueHandler:   joinQueueHandler,
		cancelQueueHandler: cancelQueueHandler,
		getMatchHandler:    getMatchHandler,
		getHistoryHandler:  getHistoryHandler,
		snapshotRepo:       snapshotRepo,
	}
}

func (h *Handler) JoinQueue(
	req *solomatchv1message.JoinQueueRequest,
	stream grpc.ServerStreamingServer[solomatchv1message.JoinQueueResponse],
) error {
	return h.handleJoinQueue(stream.Context(), req, stream.Send)
}

func (h *Handler) handleJoinQueue(
	ctx context.Context,
	_ *solomatchv1message.JoinQueueRequest,
	send func(*solomatchv1message.JoinQueueResponse) error,
) error {
	userID, ok := ctx.Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return status.Error(codes.Unauthenticated, "user authentication required")
	}

	result, err := h.joinQueueHandler.Handle(ctx, command.JoinMatchmakingQueueCommand{
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, derror.ErrActiveMatchAlreadyExists) {
			return status.Error(codes.FailedPrecondition, err.Error())
		}
		return status.Errorf(codes.Internal, "join matchmaking queue: %v", err)
	}

	// Gửi sự kiện ban đầu
	err = send(&solomatchv1message.JoinQueueResponse{
		Type:           solomatchv1message.MatchStreamEventType_MATCH_STREAM_EVENT_TYPE_QUEUED,
		Message:        "Searching for an opponent...",
		ElapsedSeconds: 0,
	})
	if err != nil {
		return err
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var elapsedSeconds int32

	for {
		select {
		case <-ctx.Done():
			// Client ngắt mạng hoặc hủy -> Tự động cancel ticket
			_ = h.cancelQueueHandler.Handle(context.Background(), command.CancelMatchmakingCommand{
				TicketID: result.TicketID,
				UserID:   userID,
			})
			return ctx.Err()

		case event, ok := <-result.EventChan:
			if !ok {
				return status.Error(codes.Unavailable, "matchmaking connection interrupted, please retry")
			}
			if event != nil && event.Match != nil {
				// Lấy snapshot thông tin đối thủ để hiển thị
				userIDs := make([]string, 0, len(event.Match.Participants()))
				for _, p := range event.Match.Participants() {
					userIDs = append(userIDs, p.UserID())
				}
				snapshots, _ := h.snapshotRepo.FindByUserIDs(ctx, userIDs)
				if snapshots == nil {
					snapshots = make(map[string]*entity.UserSnapshot)
				}

				matchDTO := dto.ToSoloMatchDTO(event.Match, snapshots)
				matchView := ToSoloMatchView(&matchDTO)

				return send(&solomatchv1message.JoinQueueResponse{
					Type:           solomatchv1message.MatchStreamEventType_MATCH_STREAM_EVENT_TYPE_MATCHED,
					Message:        "Opponent found!",
					ElapsedSeconds: elapsedSeconds,
					Match:          matchView,
				})
			}

		case <-ticker.C:
			elapsedSeconds++
			err = send(&solomatchv1message.JoinQueueResponse{
				Type:           solomatchv1message.MatchStreamEventType_MATCH_STREAM_EVENT_TYPE_QUEUED,
				Message:        "Searching for an opponent...",
				ElapsedSeconds: elapsedSeconds,
			})
			if err != nil {
				return err
			}
		}
	}
}

func (h *Handler) CancelQueue(
	ctx context.Context,
	req *solomatchv1message.CancelQueueRequest,
) (*solomatchv1message.CancelQueueResponse, error) {
	userID, ok := ctx.Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return nil, status.Error(codes.Unauthenticated, "user authentication required")
	}

	err := h.cancelQueueHandler.Handle(ctx, command.CancelMatchmakingCommand{
		TicketID: req.TicketId,
		UserID:   userID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cancel matchmaking queue: %v", err)
	}

	return &solomatchv1message.CancelQueueResponse{Success: true}, nil
}

func (h *Handler) GetMatch(
	ctx context.Context,
	req *solomatchv1message.GetMatchRequest,
) (*solomatchv1message.GetMatchResponse, error) {
	matchDTO, err := h.getMatchHandler.Handle(ctx, query.GetSoloMatchByIDQuery{
		MatchID: req.MatchId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get match by id: %v", err)
	}
	if matchDTO == nil {
		return nil, status.Error(codes.NotFound, "match not found")
	}

	return &solomatchv1message.GetMatchResponse{
		Match: ToSoloMatchView(matchDTO),
	}, nil
}

func (h *Handler) GetMatchHistory(
	ctx context.Context,
	req *solomatchv1message.GetMatchHistoryRequest,
) (*solomatchv1message.GetMatchHistoryResponse, error) {
	userID, ok := ctx.Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return nil, status.Error(codes.Unauthenticated, "user authentication required")
	}

	historyDTOs, err := h.getHistoryHandler.Handle(ctx, query.GetUserMatchHistoryQuery{
		UserID:   userID,
		PageSize: int(req.PageSize),
		Offset:   0,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get match history: %v", err)
	}

	views := make([]*solomatchv1message.SoloMatchView, 0, len(historyDTOs))
	for i := range historyDTOs {
		views = append(views, ToSoloMatchView(&historyDTOs[i]))
	}

	return &solomatchv1message.GetMatchHistoryResponse{
		Matches: views,
	}, nil
}
