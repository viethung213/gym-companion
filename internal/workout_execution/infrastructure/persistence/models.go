package persistence

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// WorkoutSessionModel maps to workout_execution.workout_sessions table.
type WorkoutSessionModel struct {
	ID               string               `gorm:"primaryKey;column:id"`
	UserID           string               `gorm:"column:user_id"`
	PlanID           string               `gorm:"column:plan_id"`
	Status           string               `gorm:"column:status"`
	TotalSets        int                  `gorm:"column:total_sets"`
	TotalVolume      float32              `gorm:"column:total_volume"`
	AverageFormScore *float32             `gorm:"column:average_form_score"`
	AverageRPE       float32              `gorm:"column:average_rpe"`
	ScheduledAt      *time.Time           `gorm:"column:scheduled_at"`
	StartedAt        *time.Time           `gorm:"column:started_at"`
	EndedAt          *time.Time           `gorm:"column:ended_at"`
	CreatedAt        time.Time            `gorm:"column:created_at"`
	UpdatedAt        time.Time            `gorm:"column:updated_at"`
	Sets             []WorkoutSetLogModel `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE"`
	Errors           []SessionErrorModel  `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE"`
}

// TableName returns table name for GORM.
func (WorkoutSessionModel) TableName() string {
	return "workout_execution.workout_sessions"
}

// WorkoutSetLogModel maps to workout_execution.workout_set_logs table.
type WorkoutSetLogModel struct {
	ID          string        `gorm:"primaryKey;column:id"`
	SessionID   string        `gorm:"column:session_id"`
	SetNumber   int           `gorm:"column:set_number"`
	ExerciseID  string        `gorm:"column:exercise_id"`
	TargetReps  int           `gorm:"column:target_reps"`
	ActualReps  int           `gorm:"column:actual_reps"`
	Weight      float32       `gorm:"column:weight"`
	FormScore   *float32      `gorm:"column:form_score"`
	RPE         float32       `gorm:"column:rpe"`
	CameraAngle string        `gorm:"column:camera_angle"`
	CreatedAt   time.Time     `gorm:"column:created_at"`
	Reps        []RepLogModel `gorm:"foreignKey:SetLogID;constraint:OnDelete:CASCADE"`
}

// TableName returns table name for GORM.
func (WorkoutSetLogModel) TableName() string {
	return "workout_execution.workout_set_logs"
}

// RepLogModel maps to workout_execution.rep_logs table.
type RepLogModel struct {
	ID            string    `gorm:"primaryKey;column:id"`
	SetLogID      string    `gorm:"column:set_log_id"`
	RepNumber     int       `gorm:"column:rep_number"`
	ROMPercentage float32   `gorm:"column:rom_percentage"`
	ErrorCodes    []byte    `gorm:"column:error_codes;type:jsonb"`
	JointAngles   []byte    `gorm:"column:joint_angles;type:jsonb"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

// TableName returns table name for GORM.
func (RepLogModel) TableName() string {
	return "workout_execution.rep_logs"
}

// SessionErrorModel maps to workout_execution.session_errors table.
type SessionErrorModel struct {
	ID         string    `gorm:"primaryKey;column:id"`
	SessionID  string    `gorm:"column:session_id"`
	SetNumber  int       `gorm:"column:set_number"`
	RepNumber  int       `gorm:"column:rep_number"`
	ExerciseID string    `gorm:"column:exercise_id"`
	ErrorCode  string    `gorm:"column:error_code"`
	Severity   string    `gorm:"column:severity"`
	Timestamp  time.Time `gorm:"column:timestamp"`
}

// TableName returns table name for GORM.
func (SessionErrorModel) TableName() string {
	return "workout_execution.session_errors"
}

// PersonalRecordModel maps to workout_execution.personal_records table.
type PersonalRecordModel struct {
	ID           string    `gorm:"primaryKey;column:id"`
	UserID       string    `gorm:"column:user_id"`
	ExerciseID   string    `gorm:"column:exercise_id"`
	OneRepMax    float32   `gorm:"column:one_rep_max"`
	Weight       float32   `gorm:"column:weight"`
	Reps         int       `gorm:"column:reps"`
	FormVerified bool      `gorm:"column:form_verified"`
	AchievedAt   time.Time `gorm:"column:achieved_at"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

// TableName returns table name for GORM.
func (PersonalRecordModel) TableName() string {
	return "workout_execution.personal_records"
}

// MotionSpecificationModel maps to workout_execution.motion_specifications table.
type MotionSpecificationModel struct {
	ExerciseID             string    `gorm:"primaryKey;column:exercise_id"`
	OnnxModelURL           string    `gorm:"column:onnx_model_url"`
	LocalRulesURL          string    `gorm:"column:local_rules_url"`
	DialogueEngineURL      string    `gorm:"column:dialogue_engine_url"`
	RecommendedCameraAngle string    `gorm:"column:recommended_camera_angle"`
	IsReady                bool      `gorm:"column:is_ready;default:false"`
	CreatedAt              time.Time `gorm:"column:created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at"`
}

// TableName returns table name for GORM.
func (MotionSpecificationModel) TableName() string {
	return "workout_execution.motion_specifications"
}

// OutboxLogModel maps to workout_execution.outbox_log table.
type OutboxLogModel struct {
	ID           string    `gorm:"primaryKey;type:uuid"`
	EventID      string    `gorm:"column:event_id;type:uuid;not null"`
	EventType    string    `gorm:"column:event_type;not null"`
	Payload      []byte    `gorm:"column:payload;type:jsonb;not null"`
	PartitionKey string    `gorm:"column:partition_key;not null"`
	ProcessedAt  time.Time `gorm:"column:processed_at;autoCreateTime"`
	Status       string    `gorm:"column:status;not null"`
	ErrorMessage string    `gorm:"column:error_message"`
}

// TableName returns table name for GORM.
func (OutboxLogModel) TableName() string {
	return "workout_execution.outbox_log"
}

// OutboxModel maps to workout_execution.outbox table.
type OutboxModel struct {
	ID            string       `gorm:"primaryKey;column:id"`
	EventID       string       `gorm:"column:event_id"`
	AggregateType string       `gorm:"column:aggregate_type"`
	AggregateID   string       `gorm:"column:aggregate_id"`
	EventType     string       `gorm:"column:event_type"`
	Payload       []byte       `gorm:"column:payload"`
	PartitionKey  string       `gorm:"column:partition_key"`
	Published     bool         `gorm:"column:published;default:false"`
	PublishedAt   sql.NullTime `gorm:"column:published_at"`
	CreatedAt     time.Time    `gorm:"column:created_at"`
}

// TableName returns table name for GORM.
func (OutboxModel) TableName() string {
	return "workout_execution.outbox"
}

// Mappers to / from Domain

func SessionToPersistence(session *aggregate.WorkoutSession) *WorkoutSessionModel {
	summary := session.CalculateSummary()
	model := &WorkoutSessionModel{
		ID:               session.ID(),
		UserID:           session.UserID(),
		PlanID:           session.PlanID(),
		Status:           session.Status().String(),
		TotalSets:        summary.TotalSets,
		TotalVolume:      summary.TotalVolume,
		AverageFormScore: summary.AverageFormScore,
		AverageRPE:       summary.AverageRPE,
		ScheduledAt:      session.ScheduledAt(),
		StartedAt:        session.StartedAt(),
		EndedAt:          session.EndedAt(),
		CreatedAt:        session.CreatedAt(),
		UpdatedAt:        session.UpdatedAt(),
		Sets:             make([]WorkoutSetLogModel, 0, len(session.Sets())),
		Errors:           make([]SessionErrorModel, 0, len(session.Errors())),
	}

	for _, set := range session.Sets() {
		setModel := WorkoutSetLogModel{
			ID:          set.ID,
			SessionID:   session.ID(),
			SetNumber:   set.SetNumber,
			ExerciseID:  set.ExerciseID,
			TargetReps:  set.TargetReps,
			ActualReps:  set.ActualReps,
			Weight:      set.Weight,
			FormScore:   set.FormScore,
			RPE:         set.RPE,
			CameraAngle: set.CameraAngle,
			CreatedAt:   set.CreatedAt,
			Reps:        make([]RepLogModel, 0, len(set.Reps)),
		}

		for _, r := range set.Reps {
			errBytes, _ := json.Marshal(r.GetErrorCodes())
			anglesBytes, _ := json.Marshal(r.GetJointAngles())
			setModel.Reps = append(setModel.Reps, RepLogModel{
				ID:            session.ID() + "-" + set.ID,
				SetLogID:      set.ID,
				RepNumber:     r.RepNumber,
				ROMPercentage: r.ROMPercentage,
				ErrorCodes:    errBytes,
				JointAngles:   anglesBytes,
			})
		}
		model.Sets = append(model.Sets, setModel)
	}

	for _, e := range session.Errors() {
		model.Errors = append(model.Errors, SessionErrorModel{
			ID:         e.ID,
			SessionID:  session.ID(),
			SetNumber:  e.SetNumber,
			RepNumber:  e.RepNumber,
			ExerciseID: e.ExerciseID,
			ErrorCode:  e.ErrorCode,
			Severity:   e.Severity,
			Timestamp:  e.Timestamp,
		})
	}

	return model
}

func SessionToDomain(m *WorkoutSessionModel) *aggregate.WorkoutSession {
	sets := make([]aggregate.WorkoutSetLog, 0, len(m.Sets))
	for _, sm := range m.Sets {
		reps := make([]vo.RepLog, 0, len(sm.Reps))
		for _, rm := range sm.Reps {
			var errs []string
			var angles map[string]float32
			if len(rm.ErrorCodes) > 0 {
				_ = json.Unmarshal(rm.ErrorCodes, &errs)
			}
			if len(rm.JointAngles) > 0 {
				_ = json.Unmarshal(rm.JointAngles, &angles)
			}
			reps = append(reps, vo.NewRepLog(rm.RepNumber, rm.ROMPercentage, errs, angles))
		}

		sets = append(sets, aggregate.WorkoutSetLog{
			ID:          sm.ID,
			SessionID:   sm.SessionID,
			SetNumber:   sm.SetNumber,
			ExerciseID:  sm.ExerciseID,
			TargetReps:  sm.TargetReps,
			ActualReps:  sm.ActualReps,
			Weight:      sm.Weight,
			FormScore:   sm.FormScore,
			RPE:         sm.RPE,
			CameraAngle: sm.CameraAngle,
			Reps:        reps,
			CreatedAt:   sm.CreatedAt,
		})
	}

	errs := make([]aggregate.SessionError, 0, len(m.Errors))
	for _, em := range m.Errors {
		errs = append(errs, aggregate.SessionError{
			ID:         em.ID,
			SessionID:  em.SessionID,
			SetNumber:  em.SetNumber,
			RepNumber:  em.RepNumber,
			ExerciseID: em.ExerciseID,
			ErrorCode:  em.ErrorCode,
			Severity:   em.Severity,
			Timestamp:  em.Timestamp,
		})
	}

	status := aggregate.ParseSessionStatus(m.Status)
	return aggregate.ReconstituteWorkoutSession(
		m.ID, m.UserID, m.PlanID, status, sets, errs,
		m.ScheduledAt, m.StartedAt, m.EndedAt, m.CreatedAt, m.UpdatedAt,
	)
}

func PersonalRecordToPersistence(pr *aggregate.PersonalRecord) *PersonalRecordModel {
	return &PersonalRecordModel{
		ID:           pr.ID(),
		UserID:       pr.UserID(),
		ExerciseID:   pr.ExerciseID(),
		OneRepMax:    pr.OneRepMax(),
		Weight:       pr.Weight(),
		Reps:         pr.Reps(),
		FormVerified: pr.FormVerified(),
		AchievedAt:   pr.AchievedAt(),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
}

func PersonalRecordToDomain(m *PersonalRecordModel) *aggregate.PersonalRecord {
	return aggregate.ReconstitutePersonalRecord(
		m.ID, m.UserID, m.ExerciseID, m.OneRepMax, m.Weight, m.Reps,
		m.FormVerified, m.AchievedAt, m.CreatedAt, m.UpdatedAt,
	)
}

func OutboxToDomain(m *OutboxModel) *port.OutboxRecord {
	var pubAt *time.Time
	if m.PublishedAt.Valid {
		pubAt = &m.PublishedAt.Time
	}
	return &port.OutboxRecord{
		ID:            m.ID,
		EventID:       m.EventID,
		AggregateType: m.AggregateType,
		AggregateID:   m.AggregateID,
		EventType:     m.EventType,
		Payload:       m.Payload,
		PartitionKey:  m.PartitionKey,
		Published:     m.Published,
		CreatedAt:     m.CreatedAt,
		PublishedAt:   pubAt,
	}
}
