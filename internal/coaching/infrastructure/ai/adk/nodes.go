package adk

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/workflow"
)

func (c *CoachingContextAgent) buildNodes() {
	c.fetchNode = workflow.NewFunctionNode(
		"fetch_profile_and_history",
		func(nodeCtx agent.Context, rawUserID string) (CoachInput, error) {
			userID := strings.TrimSpace(rawUserID)
			if userID == "" || strings.ContainsAny(userID, " \t\n") {
				return CoachInput{}, fmt.Errorf("%w: user id %q is not a valid identifier",
					ErrPlanGenerationFailed, rawUserID)
			}

			profile, getErr := c.profileReader.GetProfile(nodeCtx, userID)
			if getErr != nil {
				return CoachInput{}, fmt.Errorf("get profile: %w", getErr)
			}

			// Fetch only recent 7 days (max 3 items) to drastically reduce Prompt Tokens.
			recent, getErr := c.sessionReader.GetRecentSessions(nodeCtx, userID, time.Now().AddDate(0, 0, -7))
			if getErr != nil {
				return CoachInput{}, fmt.Errorf("get recent sessions: %w", getErr)
			}
			if len(recent) > 3 {
				recent = recent[:3]
			}

			slots := make([]WorkoutSlot, len(profile.AvailableSlots))
			preferredTimes := make(map[string][]string)
			for i, s := range profile.AvailableSlots {
				slots[i] = WorkoutSlot{
					DayOfWeek: int(s.DayOfWeek),
					StartTime: s.StartTime,
					EndTime:   s.EndTime,
				}
				dayKey := weekdayToCode(s.DayOfWeek)
				timeSlotStr := s.StartTime
				if s.EndTime != "" {
					timeSlotStr += "-" + s.EndTime
				}
				preferredTimes[dayKey] = append(preferredTimes[dayKey], timeSlotStr)
			}

			injuries := make([]InjuryStatus, len(profile.ActiveInjuries))
			for i, inj := range profile.ActiveInjuries {
				injuries[i] = InjuryStatus{
					MuscleGroup: inj.MuscleGroup,
					Severity:    "MODERATE",
				}
			}

			adkSessions := make([]WorkoutSession, len(recent))
			for i := range recent {
				r := &recent[i]
				adkSessions[i] = WorkoutSession{
					SessionID:    r.SessionID,
					CompletedAt:  r.CompletedAt,
					MuscleGroups: nil,
					AverageRPE:   r.AverageRPE,
					TotalSets:    r.TotalSets,
					Aborted:      r.Aborted,
				}
			}

			return CoachInput{
				Flow:        FlowInitiate4Week,
				CurrentTime: time.Now().UTC().Format(time.RFC3339),
				Profile: UserProfile{
					UserID:                userID,
					WeightKg:              profile.WeightKg,
					PrimaryGoal:           profile.PrimaryGoal,
					AvailableEquipment:    profile.AvailableEquipment,
					PreferredMuscleGroups: profile.PreferredMuscleGroups,
					PreferredWorkoutTimes: preferredTimes,
					AvailableSlots:        slots,
					ActiveInjuries:        injuries,
				},
				RecentSessions: adkSessions,
			}, nil
		},
		workflow.NodeConfig{},
	)

	// Only the rewrite flows run this; a fresh roadmap has nothing to revise.
	c.pendingNode = workflow.NewFunctionNode(
		"fetch_pending_sessions",
		func(nodeCtx agent.Context, in CoachInput) (CoachInput, error) {
			if c.roadmaps == nil {
				return in, nil
			}

			rm, findErr := c.roadmaps.FindActiveByUser(nodeCtx, in.Profile.UserID)
			if findErr != nil {
				if errors.Is(findErr, roadmap.ErrRoadmapNotFound) {
					return in, nil
				}
				return CoachInput{}, fmt.Errorf("find active roadmap: %w", findErr)
			}

			pending := rm.PendingSessionsFrom(time.Now().UTC().Truncate(24 * time.Hour))
			if len(pending) == 0 {
				return in, nil
			}

			in.SessionsToRevise = make([]SessionToRevise, 0, len(pending))
			for _, s := range pending {
				info := s.Info()
				in.SessionsToRevise = append(in.SessionsToRevise, SessionToRevise{
					ScheduledDate:            info.ScheduledDate.Format(scheduledDateISO),
					SlotTime:                 info.SlotTime,
					EstimatedDurationMinutes: info.EstimatedDurationMinutes,
					TargetMuscleGroups:       info.TargetMuscleGroups,
					CurrentPrescription:      prescriptionToDTO(info.Prescription),
					CurrentReasoning:         info.Reasoning,
				})
			}

			in.CurrentRoadmap = &RoadmapSnapshot{
				RoadmapID: rm.ID(),
				Phase:     currentPhase(rm, pending[0]),
			}

			return in, nil
		},
		workflow.NodeConfig{},
	)

	c.parseNode = workflow.NewFunctionNode(
		"parse_to_schema",
		func(_ agent.Context, planText string) (*GeneratedPlan, error) {
			var plan GeneratedPlan
			if parseErr := json.Unmarshal([]byte(planText), &plan); parseErr != nil {
				return nil, fmt.Errorf("unmarshal plan: %w", parseErr)
			}
			return &plan, nil
		},
		workflow.NodeConfig{},
	)
}

func weekdayToCode(w time.Weekday) string {
	switch w {
	case time.Monday:
		return "mon"
	case time.Tuesday:
		return "tue"
	case time.Wednesday:
		return "wed"
	case time.Thursday:
		return "thu"
	case time.Friday:
		return "fri"
	case time.Saturday:
		return "sat"
	case time.Sunday:
		return "sun"
	default:
		return "mon"
	}
}
