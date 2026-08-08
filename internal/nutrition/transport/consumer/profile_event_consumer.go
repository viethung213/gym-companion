package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	authv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/event"
	profilev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"google.golang.org/protobuf/encoding/protojson"
)

type CloudEventEnvelope struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

// ProfileEventConsumer listens to Profile & Auth events to initialize user meal schedules and generate daily plans.
type ProfileEventConsumer struct {
	reader              *kafka.Reader
	planRepo            repository.NutritionPlanRepository
	outboxLogRepo       port.OutboxLogRepository
	generatePlanHandler *command.GenerateDailyPlanHandler
}

func NewProfileEventConsumer(
	reader *kafka.Reader,
	planRepo repository.NutritionPlanRepository,
	outboxLogRepo port.OutboxLogRepository,
	generatePlanHandler *command.GenerateDailyPlanHandler,
) *ProfileEventConsumer {
	return &ProfileEventConsumer{
		reader:              reader,
		planRepo:            planRepo,
		outboxLogRepo:       outboxLogRepo,
		generatePlanHandler: generatePlanHandler,
	}
}

func (c *ProfileEventConsumer) Start(ctx context.Context) error {
	if c.reader == nil {
		log.Println("[Nutrition ProfileEventConsumer] Reader is nil, skipping consumer loop.")
		return nil
	}

	log.Println("[Nutrition ProfileEventConsumer] Started listening to Kafka events (UserRegistered & ProfileUpdated >= 80%)...")
	for {
		select {
		case <-ctx.Done():
			log.Println("[Nutrition ProfileEventConsumer] Stopping consumer due to context cancellation.")
			return nil
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				log.Printf("[Nutrition ProfileEventConsumer] Error reading Kafka message: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if err := c.ProcessMessage(ctx, msg); err != nil {
				log.Printf("[Nutrition ProfileEventConsumer] Error processing message: %v", err)
			}
		}
	}
}

//nolint:gocritic // kafka.Message is passed by value per kafka consumer library design
func (c *ProfileEventConsumer) ProcessMessage(ctx context.Context, msg kafka.Message) error {
	var env CloudEventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		return fmt.Errorf("unmarshal cloudevent envelope: %w", err)
	}

	// 1. Check Outbox Log Idempotency: skip if already processed
	if c.outboxLogRepo != nil && env.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, env.ID)
		if err != nil {
			log.Printf("[Nutrition ProfileEventConsumer] Warning checking outbox log idempotency: %v", err)
		} else if processed {
			log.Printf("[Nutrition ProfileEventConsumer] Event %s already processed in outbox_log, skipping", env.ID)
			return nil
		}
	}

	defaultSchedules := map[string]string{
		"BREAKFAST": "07:00",
		"LUNCH":     "12:00",
		"SNACK":     "15:30",
		"DINNER":    "19:00",
	}

	var processErr error

	switch env.Type {
	case "contracts.generic.auth.v1.userRegistered", "UserRegistered":
		var data authv1event.UserRegistered
		if err := protojson.Unmarshal(env.Data, &data); err != nil {
			if errLegacy := json.Unmarshal(env.Data, &data); errLegacy != nil {
				processErr = fmt.Errorf("unmarshal UserRegistered data payload: %w", err)
				c.saveLog(ctx, env, "", "FAILED", processErr)
				return processErr
			}
		}

		userID := data.GetUserId()
		if userID == "" {
			return nil
		}

		if err := c.planRepo.SaveUserMealSchedules(ctx, userID, defaultSchedules); err != nil {
			processErr = fmt.Errorf("save default user meal schedules: %w", err)
			c.saveLog(ctx, env, userID, "FAILED", processErr)
			return processErr
		}
		log.Printf("[Nutrition ProfileEventConsumer] Seeded default meal schedule SQL records for new user: %s", userID)
		c.saveLog(ctx, env, userID, "PROCESSED", nil)

	case "contracts.supporting.profile.v1.event.ProfileUpdated", "ProfileUpdated":
		var data profilev1event.ProfileUpdated
		if err := protojson.Unmarshal(env.Data, &data); err != nil {
			if errLegacy := json.Unmarshal(env.Data, &data); errLegacy != nil {
				processErr = fmt.Errorf("unmarshal ProfileUpdated payload: %w", err)
				c.saveLog(ctx, env, "", "FAILED", processErr)
				return processErr
			}
		}

		userID := data.GetUserId()
		if userID == "" {
			return nil
		}

		rate := data.GetCompletionRate()
		// Tỉ lệ hoàn thiện profile >= 80% (thang 0-100 hoặc 0-1)
		isAbove80 := (rate >= 80.0) || (rate >= 0.80 && rate <= 1.0)
		if !isAbove80 {
			log.Printf("[Nutrition ProfileEventConsumer] Profile completion rate %.1f for user %s is < 80%%, skipping auto plan generation.", rate, userID)
			c.saveLog(ctx, env, userID, "SKIPPED", nil)
			return nil
		}

		if err := c.planRepo.SaveUserMealSchedules(ctx, userID, defaultSchedules); err != nil {
			log.Printf("[Nutrition ProfileEventConsumer] SaveUserMealSchedules warning: %v", err)
		}

		bioMetrics := service.BiologicalMetrics{
			WeightKg:      float64(data.GetWeightKg()),
			HeightCm:      float64(data.GetHeightCm()),
			Age:           25,
			Gender:        data.GetGender(),
			ActivityLevel: "MODERATELY_ACTIVE",
		}
		if bioMetrics.WeightKg <= 0 {
			bioMetrics.WeightKg = 70.0
		}
		if bioMetrics.HeightCm <= 0 {
			bioMetrics.HeightCm = 170.0
		}

		if c.generatePlanHandler != nil {
			_, genErr := c.generatePlanHandler.Handle(ctx, command.GenerateDailyPlanCommand{
				UserID:            userID,
				PlanDate:          time.Now(),
				BiologicalMetrics: bioMetrics,
				UserRestrictions:  data.GetGoals(),
			})
			if genErr != nil {
				log.Printf("[Nutrition ProfileEventConsumer] GenerateDailyPlan error for user %s: %v", userID, genErr)
				c.saveLog(ctx, env, userID, "FAILED", genErr)
				return genErr
			}
			log.Printf("[Nutrition ProfileEventConsumer] Successfully generated daily meals for user %s (profile completion >= 80%%)", userID)
		}
		c.saveLog(ctx, env, userID, "PROCESSED", nil)

	case "contracts.supporting.profile.v1.event.ProfileCompleted", "ProfileCompleted":
		var data profilev1event.ProfileCompleted
		if err := protojson.Unmarshal(env.Data, &data); err != nil {
			if errLegacy := json.Unmarshal(env.Data, &data); errLegacy != nil {
				processErr = fmt.Errorf("unmarshal ProfileCompleted payload: %w", err)
				c.saveLog(ctx, env, "", "FAILED", processErr)
				return processErr
			}
		}

		userID := data.GetUserId()
		if userID == "" {
			return nil
		}

		if err := c.planRepo.SaveUserMealSchedules(ctx, userID, defaultSchedules); err != nil {
			log.Printf("[Nutrition ProfileEventConsumer] SaveUserMealSchedules warning: %v", err)
		}

		bio := data.GetBiologicalMetrics()
		bioMetrics := service.BiologicalMetrics{
			WeightKg:      70.0,
			HeightCm:      170.0,
			Age:           25,
			Gender:        "MALE",
			ActivityLevel: "MODERATELY_ACTIVE",
		}
		if bio != nil {
			if bio.GetWeightKg() > 0 {
				bioMetrics.WeightKg = float64(bio.GetWeightKg())
			}
			if bio.GetHeightCm() > 0 {
				bioMetrics.HeightCm = float64(bio.GetHeightCm())
			}
			if bio.GetAge() > 0 {
				bioMetrics.Age = int(bio.GetAge())
			}
			if bio.GetGender() != "" {
				bioMetrics.Gender = bio.GetGender()
			}
		}

		if c.generatePlanHandler != nil {
			_, genErr := c.generatePlanHandler.Handle(ctx, command.GenerateDailyPlanCommand{
				UserID:            userID,
				PlanDate:          time.Now(),
				BiologicalMetrics: bioMetrics,
				UserRestrictions:  data.GetGoals(),
			})
			if genErr != nil {
				log.Printf("[Nutrition ProfileEventConsumer] GenerateDailyPlan error for user %s: %v", userID, genErr)
				c.saveLog(ctx, env, userID, "FAILED", genErr)
				return genErr
			}
			log.Printf("[Nutrition ProfileEventConsumer] Successfully generated daily meals for user %s (ProfileCompleted 100%%)", userID)
		}
		c.saveLog(ctx, env, userID, "PROCESSED", nil)
	}

	return nil
}

//nolint:gocritic // env envelope value object is passed by value per helper design
func (c *ProfileEventConsumer) saveLog(ctx context.Context, env CloudEventEnvelope, userID, status string, err error) {
	if c.outboxLogRepo == nil || env.ID == "" {
		return
	}
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	logRecord := &port.OutboxLogRecord{
		ID:           env.ID,
		EventID:      env.ID,
		EventType:    env.Type,
		Payload:      env.Data,
		PartitionKey: userID,
		Status:       status,
		ErrorMessage: errMsg,
	}
	if saveErr := c.outboxLogRepo.SaveLog(ctx, logRecord); saveErr != nil {
		log.Printf("[Nutrition ProfileEventConsumer] Warning: failed to save outbox log for event %s: %v", env.ID, saveErr)
	}
}
