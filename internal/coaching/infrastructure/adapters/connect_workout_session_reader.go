package adapters

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	workoutexecutionmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/service/workoutexecutionv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

var _ port.WorkoutSessionReader = (*ConnectWorkoutSessionReader)(nil)

// ConnectWorkoutSessionReader queries workout execution history and PRs via API Gateway.
type ConnectWorkoutSessionReader struct {
	client         workoutexecutionv1serviceconnect.AdminWorkoutExecutionServiceClient
	internalSecret string
}

// NewConnectWorkoutSessionReader creates a new ConnectWorkoutSessionReader communicating through API Gateway.
func NewConnectWorkoutSessionReader(gatewayURL, internalSecret string, httpClient *http.Client) *ConnectWorkoutSessionReader {
	if httpClient == nil {
		httpClient = newDefaultHTTPClient()
	}
	transport := connecthttp.NewTransport(httpClient, gatewayURL)
	client := workoutexecutionv1serviceconnect.NewAdminWorkoutExecutionServiceClient(
		connect.NewClient(transport),
	)
	return &ConnectWorkoutSessionReader{
		client:         client,
		internalSecret: internalSecret,
	}
}

// GetRecentSessions fetches completed or aborted workout sessions for the user since given time.
func (c *ConnectWorkoutSessionReader) GetRecentSessions(ctx context.Context, userID string, since time.Time) ([]port.WorkoutSession, error) {
	if userID == "" {
		return nil, errors.New("user id cannot be empty")
	}

	tokenStr, err := internaltoken.Generate(userID, "Admin", "ai-coaching", time.Minute, c.internalSecret)
	if err != nil {
		return nil, fmt.Errorf("generate internal token: %w", err)
	}

	req := &workoutexecutionmsg.AdminGetWorkoutHistoryRequest{
		UserId: userID,
		Limit:  50,
		Offset: 0,
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set(middleware.HeaderInternalToken, tokenStr)

	res, err := c.client.AdminGetWorkoutHistory(ctx, req)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return []port.WorkoutSession{}, nil
		}
		return nil, fmt.Errorf("gateway get workout history for coaching user %s: %w", userID, err)
	}

	rawSessions := res.GetSessions()
	sessions := make([]port.WorkoutSession, 0, len(rawSessions))

	for _, sess := range rawSessions {
		if sess == nil {
			continue
		}
		var completedAt time.Time
		if sess.GetDate() != nil {
			completedAt = sess.GetDate().AsTime()
		} else {
			completedAt = time.Now()
		}

		if !since.IsZero() && completedAt.Before(since) {
			continue
		}

		avgForm := 0.0
		if sess.AverageFormScore != nil {
			avgForm = float64(sess.GetAverageFormScore())
		}

		sessions = append(sessions, port.WorkoutSession{
			SessionID:        sess.GetSessionId(),
			UserID:           userID,
			CompletedAt:      completedAt,
			TotalSets:        int(sess.GetTotalSets()),
			AverageFormScore: avgForm,
		})
	}

	return sessions, nil
}

// GetSetLogs fetches recent set logs for a user and exerciseID via personal records.
func (c *ConnectWorkoutSessionReader) GetSetLogs(ctx context.Context, userID, exerciseID string, _ int) ([]port.SetLog, error) {
	if userID == "" || exerciseID == "" {
		return []port.SetLog{}, nil
	}

	tokenStr, err := internaltoken.Generate(userID, "Admin", "ai-coaching", time.Minute, c.internalSecret)
	if err != nil {
		return nil, fmt.Errorf("generate internal token: %w", err)
	}

	req := &workoutexecutionmsg.AdminGetPersonalRecordsRequest{
		UserId:      userID,
		ExerciseIds: []string{exerciseID},
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set(middleware.HeaderInternalToken, tokenStr)

	res, err := c.client.AdminGetPersonalRecords(ctx, req)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return []port.SetLog{}, nil
		}
		return nil, fmt.Errorf("gateway get personal records for coaching user %s: %w", userID, err)
	}

	records := res.GetRecords()
	if len(records) == 0 {
		return []port.SetLog{}, nil
	}

	for _, pr := range records {
		if pr == nil || pr.GetExerciseId() != exerciseID {
			continue
		}
		if pr.GetWeight() > 0 {
			achievedAt := time.Now()
			if pr.GetAchievedAt() != nil {
				achievedAt = pr.GetAchievedAt().AsTime()
			}
			return []port.SetLog{
				{
					ExerciseID:  exerciseID,
					Weight:      float64(pr.GetWeight()),
					Reps:        int(pr.GetReps()),
					RPE:         8.0,
					CompletedAt: achievedAt,
				},
			}, nil
		}
	}

	return []port.SetLog{}, nil
}
