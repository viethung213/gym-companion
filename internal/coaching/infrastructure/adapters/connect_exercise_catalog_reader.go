package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/protomap"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service/exercisev1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

const defaultSearchLimit = 100

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

// SearchByFilter queries active exercises matching the filter.
func (c *ConnectExerciseCatalogReader) SearchByFilter(ctx context.Context, filter *port.ExerciseFilter) ([]port.Exercise, error) {
	tokenStr, err := internaltoken.Generate("system", "Admin", "ai-coaching", time.Minute, c.internalSecret)
	if err != nil {
		return nil, fmt.Errorf("generate internal token: %w", err)
	}

	reqMsg := &exercisemsg.SearchExercisesRequest{
		Status: exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE,
		Limit:  defaultSearchLimit,
	}
	if filter != nil {
		applyFilter(reqMsg, filter)
	}

	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set(middleware.HeaderInternalToken, tokenStr)

	res, err := c.client.SearchExercises(ctx, reqMsg)
	if err != nil {
		return nil, fmt.Errorf("gateway search exercises: %w", err)
	}

	exercises := make([]port.Exercise, 0, len(res.GetExercises()))
	for _, item := range res.GetExercises() {
		if item == nil {
			continue
		}
		ex, mapErr := mapExerciseInfoToPort(item)
		if mapErr != nil {
			// Logged and skipped (docs/05 §7): one bad row must not hide the catalog.
			slog.WarnContext(ctx, "Skipping exercise with unknown taxonomy value",
				slog.String("field", "exercise"), slog.String("value", item.GetId()),
				slog.String("source", "exercise_catalog"), slog.String("error", mapErr.Error()))
			continue
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

	ex, err := mapExerciseInfoToPort(res.GetExercise())
	if err != nil {
		return port.Exercise{}, fmt.Errorf("map exercise %s: %w", exerciseID, err)
	}
	return ex, nil
}

func applyFilter(req *exercisemsg.SearchExercisesRequest, f *port.ExerciseFilter) {
	for _, m := range f.PrimaryMuscles {
		req.PrimaryMuscles = append(req.PrimaryMuscles, protomap.MuscleGroupToProto(m))
	}
	for _, m := range f.ExcludeMuscles {
		req.ExcludeMuscles = append(req.ExcludeMuscles, protomap.MuscleGroupToProto(m))
	}
	for _, e := range f.Equipment {
		req.Equipment = append(req.Equipment, protomap.EquipmentToProto(e))
	}
	for _, cat := range f.Categories {
		req.Categories = append(req.Categories, protomap.CategoryToProto(cat))
	}
	for _, l := range f.Levels {
		req.Levels = append(req.Levels, protomap.LevelToProto(l))
	}
	req.Keyword = f.Keyword
	if f.Limit > 0 {
		req.Limit = int32(f.Limit)
	}
	if f.Offset > 0 {
		req.Offset = int32(f.Offset)
	}
}

func mapExerciseInfoToPort(info *exercisemsg.ExerciseInfo) (port.Exercise, error) {
	primary, err := protomap.MuscleGroup(info.GetPrimaryMuscle())
	if err != nil {
		return port.Exercise{}, fmt.Errorf("primary muscle: %w", err)
	}
	secondary, err := protomap.MuscleGroups(info.GetSecondaryMuscles())
	if err != nil {
		return port.Exercise{}, fmt.Errorf("secondary muscles: %w", err)
	}
	equipment, err := protomap.Equipment(info.GetEquipment())
	if err != nil {
		return port.Exercise{}, fmt.Errorf("equipment: %w", err)
	}
	category, err := protomap.Category(info.GetCategory())
	if err != nil {
		return port.Exercise{}, fmt.Errorf("category: %w", err)
	}
	level, err := protomap.Level(info.GetLevel())
	if err != nil {
		return port.Exercise{}, fmt.Errorf("level: %w", err)
	}
	force, err := protomap.OptionalForce(info.GetForce())
	if err != nil {
		return port.Exercise{}, fmt.Errorf("force: %w", err)
	}
	mechanic, err := protomap.OptionalMechanic(info.GetMechanic())
	if err != nil {
		return port.Exercise{}, fmt.Errorf("mechanic: %w", err)
	}

	measurementType := "REPS"
	if info.GetMeasurementType() == exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION {
		measurementType = "DURATION"
	}

	return port.Exercise{
		ExerciseID:       info.GetId(),
		Name:             info.GetName(),
		PrimaryMuscle:    primary,
		SecondaryMuscles: secondary,
		Equipment:        equipment,
		Category:         category,
		Level:            level,
		Force:            force,
		Mechanic:         mechanic,
		MeasurementType:  measurementType,
	}, nil
}
