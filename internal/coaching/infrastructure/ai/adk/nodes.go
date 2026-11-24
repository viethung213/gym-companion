package adk

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
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

			// Fetch all sessions in the recent 5 days and encode them into compact, explicit strings.
			recent, getErr := c.sessionReader.GetRecentSessions(nodeCtx, userID, time.Now().AddDate(0, 0, -5))
			if getErr != nil {
				return CoachInput{}, fmt.Errorf("get recent sessions: %w", getErr)
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

			now := time.Now().UTC()
			adkSessions := make([]string, len(recent))
			for i := range recent {
				adkSessions[i] = formatRecentSession(&recent[i], now)
			}

			// Pre-fetch all PRs once (limit=0, no N+1) and group top 3 newest & strongest per muscle group.
			var prs []port.PersonalRecordSummary
			if c.sessionReader != nil {
				if fetchedPRs, prErr := c.sessionReader.GetPersonalRecords(nodeCtx, userID); prErr == nil {
					prs = fetchedPRs
				}
			}

			// Pre-select candidates with the shared eligibility rules (docs/05 §8.2-8.4):
			// hard filters, main work per primary muscle, stretching for warm-up / cool-down.
			athlete := profile.Athlete()
			var lists candidateLists
			var musclePRs map[string][]string
			if c.catalog != nil {
				exList, searchErr := c.catalog.SearchByFilter(nodeCtx, &port.ExerciseFilter{Limit: catalogPrefetchLimit})
				if searchErr != nil {
					return CoachInput{}, fmt.Errorf("search exercise catalog: %w", searchErr)
				}
				musclePRs = buildMusclePRs(exList, prs, 3)
				lists = buildCandidateLists(exList, &candidatePrefs{
					PreferredMuscles: profile.PreferredMuscleGroups,
					PrimaryGoal:      profile.PrimaryGoal,
				}, &athlete)
			}

			return CoachInput{
				Flow:        FlowInitiate4Week,
				CurrentTime: time.Now().In(profile.Zone()).Format(time.RFC3339),
				Timezone:    profile.Zone().String(),
				Profile: UserProfile{
					UserID:                userID,
					WeightKg:              profile.WeightKg,
					ExperienceLevel:       profile.ExperienceLevel,
					PrimaryGoal:           profile.PrimaryGoal,
					AvailableEquipment:    codeStrings(profile.AvailableEquipment),
					PreferredMuscleGroups: codeStrings(profile.PreferredMuscleGroups),
					PreferredWorkoutTimes: preferredTimes,
					AvailableSlots:        slots,
					ActiveInjuries:        injuryDTOs(athlete.Injuries),
				},
				RecentSessions:     adkSessions,
				MusclePRs:          musclePRs,
				AvailableExercises: lists.main,
				RecoveryExercises:  lists.recovery,
				InjuryLimits:       lists.limits,
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

			// Read the schedule in the athlete's current zone, as the handler does.
			rm.SetTimezone(in.Timezone)
			now := time.Now()
			pending := rm.UpcomingPendingSessions(now)
			if len(pending) == 0 {
				return in, nil
			}

			// A regenerate after the athlete changed their weekly slots moves the
			// remaining sessions onto the new slots instead of rewriting them in place.
			if in.Flow == FlowRegenerate {
				if targets := rescheduleTargets(rm, in.Profile.AvailableSlots, now); targets != nil {
					in.SessionsToRevise = rescheduledSessions(targets)
					in.Rescheduled = true
					in.CurrentRoadmap = &RoadmapSnapshot{RoadmapID: rm.ID(), Phase: currentPhase(rm, pending[0])}
					return in, nil
				}
			}

			in.SessionsToRevise = make([]SessionToRevise, 0, len(pending))
			for _, s := range pending {
				info := s.Info()
				in.SessionsToRevise = append(in.SessionsToRevise, SessionToRevise{
					ScheduledDate:            info.ScheduledDate.Format(scheduledDateISO),
					SlotTime:                 info.SlotTime,
					SessionKind:              string(info.Kind),
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
			cleaned := cleanJSONResponse(planText)
			switch {
			case cleaned == "":
				slog.Debug("[parse_to_schema] Received EMPTY planText", slog.Int("raw_len", len(planText)), slog.String("raw", planText))
			case len(cleaned) < 500:
				slog.Debug("[parse_to_schema] Received planText",
					slog.Int("raw_len", len(planText)), slog.Int("cleaned_len", len(cleaned)), slog.String("cleaned", cleaned))
			default:
				slog.Debug("[parse_to_schema] Received planText",
					slog.Int("raw_len", len(planText)), slog.Int("cleaned_len", len(cleaned)), slog.String("tail", cleaned[len(cleaned)-200:]))
			}
			var plan GeneratedPlan
			if parseErr := json.Unmarshal([]byte(cleaned), &plan); parseErr != nil {
				if repaired := repairJSONTail(cleaned); repaired != "" && repaired != cleaned {
					if retryErr := json.Unmarshal([]byte(repaired), &plan); retryErr == nil {
						slog.Warn("[parse_to_schema] Repaired malformed JSON tail", slog.Int("orig_len", len(cleaned)), slog.Int("repaired_len", len(repaired)))
						return &plan, nil
					}
				}
				return nil, fmt.Errorf("unmarshal plan: %w (cleaned tail: %s)", parseErr, cleaned[max(0, len(cleaned)-100):])
			}
			return &plan, nil
		},
		workflow.NodeConfig{},
	)
}

// repairJSONTail trims trailing garbage or mismatched brackets after the last
// complete JSON object and closes any remaining open '[' or '{' on the stack.
func repairJSONTail(s string) string {
	var stack []byte
	inString := false
	escaped := false
	lastObjEnd := -1

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}

		switch c {
		case '{', '[':
			stack = append(stack, c)
		case '}':
			if len(stack) == 0 || stack[len(stack)-1] != '{' {
				goto fix
			}
			stack = stack[:len(stack)-1]
			lastObjEnd = i + 1
		case ']':
			if len(stack) == 0 || stack[len(stack)-1] != '[' {
				goto fix
			}
			stack = stack[:len(stack)-1]
		}
	}

fix:
	if lastObjEnd <= 0 {
		return ""
	}
	// Recompute open stack up to lastObjEnd
	stack = stack[:0]
	inString = false
	escaped = false
	for i := 0; i < lastObjEnd; i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case '{', '[':
			stack = append(stack, c)
		case '}', ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}

	var b strings.Builder
	b.Grow(lastObjEnd + len(stack))
	b.WriteString(s[:lastObjEnd])
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == '{' {
			b.WriteByte('}')
		} else {
			b.WriteByte(']')
		}
	}
	return b.String()
}

// cleanJSONResponse strips markdown code fences and extraneous leading/trailing text.
func cleanJSONResponse(raw string) string {
	s := strings.TrimSpace(raw)

	startObj := strings.Index(s, "{")
	startArr := strings.Index(s, "[")
	start := -1
	switch {
	case startObj != -1 && startArr != -1:
		if startObj < startArr {
			start = startObj
		} else {
			start = startArr
		}
	case startObj != -1:
		start = startObj
	case startArr != -1:
		start = startArr
	}

	endObj := strings.LastIndex(s, "}")
	endArr := strings.LastIndex(s, "]")
	end := -1
	switch {
	case endObj != -1 && endArr != -1:
		if endObj > endArr {
			end = endObj
		} else {
			end = endArr
		}
	case endObj != -1:
		end = endObj
	case endArr != -1:
		end = endArr
	}

	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}

	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
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

func formatRecentSession(r *port.WorkoutSession, now time.Time) string {
	days := int(now.Sub(r.CompletedAt).Hours() / 24)
	when := "today"
	if days == 1 {
		when = "1 day ago"
	} else if days > 1 {
		when = fmt.Sprintf("%d days ago", days)
	}

	rpeLabel := "optimal effort"
	switch {
	case r.AverageRPE <= 0:
		rpeLabel = "unrated"
	case r.AverageRPE < 6.0:
		rpeLabel = "felt easy"
	case r.AverageRPE >= 9.0:
		rpeLabel = "felt overreached"
	case r.AverageRPE >= 8.0:
		rpeLabel = "felt hard"
	}

	status := "completed"
	if r.Aborted {
		status = "aborted"
	}

	return fmt.Sprintf("%s | sets: %d | RPE: %.1f/10 (%s) | status: %s",
		when, r.TotalSets, r.AverageRPE, rpeLabel, status)
}

func buildMusclePRs(exList []port.Exercise, prs []port.PersonalRecordSummary, maxPerMuscle int) map[string][]string {
	if len(prs) == 0 || len(exList) == 0 {
		return nil
	}
	exByID := make(map[string]*port.Exercise, len(exList))
	for i := range exList {
		exByID[exList[i].ExerciseID] = &exList[i]
	}

	type prEntry struct {
		pr   port.PersonalRecordSummary
		name string
	}
	grouped := make(map[string][]prEntry)
	for _, pr := range prs {
		ex, ok := exByID[pr.ExerciseID]
		if !ok {
			continue
		}
		muscle := string(ex.PrimaryMuscle)
		grouped[muscle] = append(grouped[muscle], prEntry{
			pr:   pr,
			name: ex.Name,
		})
	}
	if len(grouped) == 0 {
		return nil
	}

	out := make(map[string][]string, len(grouped))
	for muscle, entries := range grouped {
		sort.Slice(entries, func(i, j int) bool {
			if !entries[i].pr.AchievedAt.Equal(entries[j].pr.AchievedAt) {
				return entries[i].pr.AchievedAt.After(entries[j].pr.AchievedAt)
			}
			return entries[i].pr.OneRepMax > entries[j].pr.OneRepMax
		})
		if maxPerMuscle > 0 && len(entries) > maxPerMuscle {
			entries = entries[:maxPerMuscle]
		}
		lines := make([]string, len(entries))
		for i, e := range entries {
			when := ""
			if !e.pr.AchievedAt.IsZero() {
				days := int(time.Since(e.pr.AchievedAt).Hours() / 24)
				switch {
				case days <= 0:
					when = ", today"
				case days == 1:
					when = ", 1d ago"
				default:
					when = fmt.Sprintf(", %dd ago", days)
				}
			}
			lines[i] = fmt.Sprintf("%s: last %.1fkg x %d (1RM %.1fkg%s)",
				e.name, e.pr.Weight, e.pr.Reps, e.pr.OneRepMax, when)
		}
		out[muscle] = lines
	}
	return out
}
