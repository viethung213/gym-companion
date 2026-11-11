package adapters

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service/exercisev1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

var _ port.ExerciseCatalogReader = (*ConnectExerciseCatalogReader)(nil)

// ConnectExerciseCatalogReader queries exercise catalog via API Gateway over ConnectRPC.
type ConnectExerciseCatalogReader struct {
	client         exercisev1serviceconnect.ExerciseServiceClient
	internalSecret string
}

// NewConnectExerciseCatalogReader creates a new ConnectExerciseCatalogReader communicating through API Gateway.
func NewConnectExerciseCatalogReader(gatewayURL, internalSecret string, httpClient *http.Client) *ConnectExerciseCatalogReader {
	if httpClient == nil {
		httpClient = newDefaultHTTPClient()
	}
	transport := connecthttp.NewTransport(httpClient, gatewayURL)
	client := exercisev1serviceconnect.NewExerciseServiceClient(connect.NewClient(transport))
	return &ConnectExerciseCatalogReader{
		client:         client,
		internalSecret: internalSecret,
	}
}

// SearchByFilter queries active exercises matching the filter options.
func (c *ConnectExerciseCatalogReader) SearchByFilter(ctx context.Context, filter *port.ExerciseFilter) ([]port.Exercise, error) {
	tokenStr, err := internaltoken.Generate("system", "Admin", "ai-coaching", time.Minute, c.internalSecret)
	if err != nil {
		return nil, fmt.Errorf("generate internal token: %w", err)
	}

	reqMsg := &exercisemsg.SearchExercisesRequest{
		Status: exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE,
	}

	if filter != nil {
		reqMsg.BodyPartId = filter.BodyPartID
		reqMsg.TargetMuscleId = filter.TargetMuscleID
		reqMsg.SecondaryMuscleIds = filter.SecondaryMuscleIDs
		reqMsg.AvoidInjuryAreas = filter.AvoidInjuryAreas
		reqMsg.TagIds = filter.TagIDs
		reqMsg.Keyword = filter.Keyword
		reqMsg.Difficulty = filter.Difficulty
		if filter.Limit > 0 {
			reqMsg.Limit = int32(filter.Limit)
		} else {
			reqMsg.Limit = 100
		}
		if filter.Offset > 0 {
			reqMsg.Offset = int32(filter.Offset)
		}
		if len(filter.EquipmentIDs) > 0 {
			reqMsg.EquipmentId = filter.EquipmentIDs[0]
		}
	} else {
		reqMsg.Limit = 100
	}

	req := reqMsg
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set(middleware.HeaderInternalToken, tokenStr)

	res, err := c.client.SearchExercises(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gateway search exercises: %w", err)
	}

	items := res.GetExercises()
	exercises := make([]port.Exercise, 0, len(items))

	for _, item := range items {
		if item == nil {
			continue
		}
		ex := mapExerciseInfoToPort(item)

		// If caller provided multiple EquipmentIDs and we could only send 1 via proto EquipmentId,
		// filter in-memory if needed.
		if filter != nil && len(filter.EquipmentIDs) > 1 {
			matched := false
			for _, eq := range filter.EquipmentIDs {
				if strings.EqualFold(ex.Equipment, eq) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		exercises = append(exercises, ex)
	}

	return exercises, nil
}

// GetByID returns an exercise by ID or port.ErrExerciseNotFound if not found.
func (c *ConnectExerciseCatalogReader) GetByID(ctx context.Context, exerciseID string) (port.Exercise, error) {
	if exerciseID == "" {
		return port.Exercise{}, port.ErrExerciseNotFound
	}

	tokenStr, err := internaltoken.Generate("system", "Admin", "ai-coaching", time.Minute, c.internalSecret)
	if err != nil {
		return port.Exercise{}, fmt.Errorf("generate internal token: %w", err)
	}

	req := &exercisemsg.GetExerciseRequest{Id: exerciseID}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set(middleware.HeaderInternalToken, tokenStr)

	res, err := c.client.GetExercise(ctx, req)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return port.Exercise{}, fmt.Errorf("%w: %s", port.ErrExerciseNotFound, exerciseID)
		}
		return port.Exercise{}, fmt.Errorf("gateway get exercise by id %s: %w", exerciseID, err)
	}

	if res.GetExercise() == nil {
		return port.Exercise{}, fmt.Errorf("%w: %s", port.ErrExerciseNotFound, exerciseID)
	}

	return mapExerciseInfoToPort(res.GetExercise()), nil
}

func mapExerciseInfoToPort(info *exercisemsg.ExerciseInfo) port.Exercise {
	muscleGroup := info.GetTargetMuscleId()
	if muscleGroup == "" {
		muscleGroup = info.GetBodyPartId()
	}

	eqName := info.GetEquipmentId()
	eqLower := strings.ToLower(eqName)

	isBodyweight := eqLower == "bodyweight" || eqLower == "body weight" ||
		eqLower == "none" || eqLower == "" ||
		eqLower == "pull-up-bar" || eqLower == "body_weight"
	isMachineOrCable := strings.Contains(eqLower, "cable") ||
		strings.Contains(eqLower, "machine") ||
		strings.Contains(eqLower, "pulldown") ||
		strings.Contains(eqLower, "lever")

	return port.Exercise{
		ExerciseID:       info.GetId(),
		Name:             info.GetName(),
		MuscleGroup:      muscleGroup,
		Equipment:        eqName,
		Difficulty:       info.GetDifficulty(),
		IsBodyweight:     isBodyweight,
		IsMachineOrCable: isMachineOrCable,
	}
}
