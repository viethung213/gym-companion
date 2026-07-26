package coaching

import (
	"gorm.io/gorm"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
	coachingai "github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai"
	coachingpersistence "github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	coachinggrpc "github.com/viethung213/gym-companion/internal/coaching/infrastructure/transport/grpc"
	coachingv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service"
)

type Module struct {
	GRPCHandler coachingv1service.CoachingServiceServer
}

func NewModule(db *gorm.DB, geminiAPIKey string, publisher port.OutboxPublisher, exerciseProvider port.ExerciseProvider) *Module {
	roadmapRepo := coachingpersistence.NewGormWorkoutRoadmapRepository(db)
	scheduleRepo := coachingpersistence.NewGormWeeklyScheduleRepository(db)
	dailyPlanRepo := coachingpersistence.NewGormDailyWorkoutPlanRepository(db)

	validator := domain.NewUpperSafetyEnvelopeValidator()
	agent := coachingai.NewGeminiCoachAgent(geminiAPIKey)

	initiateUC := command.NewInitiateRoadmapHandler(roadmapRepo, scheduleRepo, agent, publisher, validator)
	genDailyUC := command.NewGenerateDailyPlanHandler(roadmapRepo, scheduleRepo, dailyPlanRepo, agent, exerciseProvider, publisher, validator)
	processPostUC := command.NewProcessPostWorkoutHandler(dailyPlanRepo, scheduleRepo, agent, exerciseProvider, validator)

	grpcHandler := coachinggrpc.NewCoachingGRPCHandler(initiateUC, genDailyUC, processPostUC, roadmapRepo, scheduleRepo, dailyPlanRepo)

	return &Module{
		GRPCHandler: grpcHandler,
	}
}
