package transport

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
	coachingmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	datepb "google.golang.org/genproto/googleapis/type/date"
)

func planningInput(payload *coachingmsg.InitiateRoadmapPayload) (domain.PlanningInput, error) {
	if payload == nil {
		return domain.PlanningInput{}, invalidRequest("payload is required")
	}
	if payload.GetPlanningProfile() == nil {
		if payload.GetProfileSnapshotId() != "" {
			return domain.PlanningInput{}, errProfileReaderUnavailable
		}
		return domain.PlanningInput{}, invalidRequest("planning_profile is required")
	}
	profile := payload.GetPlanningProfile()
	startDate, err := requiredDate(profile.GetStartDate())
	if err != nil {
		return domain.PlanningInput{}, err
	}
	weekdays := make([]time.Weekday, 0, len(profile.GetPreferredWeekdays()))
	for _, value := range profile.GetPreferredWeekdays() {
		weekday, parseErr := parseWeekday(value)
		if parseErr != nil {
			return domain.PlanningInput{}, parseErr
		}
		weekdays = append(weekdays, weekday)
	}
	return domain.PlanningInput{
		ProfileSnapshotID:   payload.GetProfileSnapshotId(),
		Goal:                toDomainGoal(profile.GetGoal()),
		ExperienceLevel:     toDomainExperience(profile.GetExperienceLevel()),
		TrainingDaysPerWeek: int(profile.GetTrainingDaysPerWeek()),
		PreferredWeekdays:   weekdays,
		MaxSessionMinutes:   int(profile.GetMaxSessionMinutes()),
		EquipmentIDs:        append([]string(nil), profile.GetEquipmentIds()...),
		ActiveInjuryAreas:   append([]string(nil), profile.GetActiveInjuryAreas()...),
		Timezone:            profile.GetTimezone(),
		StartDate:           startDate,
	}, nil
}

func authorizeRequest(ctx context.Context, requestedUserID string) error {
	if strings.TrimSpace(requestedUserID) == "" {
		return invalidRequest("user_id is required")
	}
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return err
	}
	if actor.UserID != requestedUserID && !actor.IsAdmin() {
		return middleware.ErrForbidden
	}
	return nil
}

func requiredDate(value *datepb.Date) (time.Time, error) {
	result, err := optionalDate(value)
	if err != nil {
		return time.Time{}, err
	}
	if result.IsZero() {
		return time.Time{}, invalidRequest("date is required")
	}
	return result, nil
}

func optionalDate(value *datepb.Date) (time.Time, error) {
	if value == nil {
		return time.Time{}, nil
	}
	result := time.Date(int(value.GetYear()), time.Month(value.GetMonth()), int(value.GetDay()), 0, 0, 0, 0, time.UTC)
	if result.Year() != int(value.GetYear()) || int(result.Month()) != int(value.GetMonth()) || result.Day() != int(value.GetDay()) {
		return time.Time{}, invalidRequest("date is invalid")
	}
	return result, nil
}

func parseWeekday(value string) (time.Weekday, error) {
	weekdays := map[string]time.Weekday{
		"sunday":    time.Sunday,
		"monday":    time.Monday,
		"tuesday":   time.Tuesday,
		"wednesday": time.Wednesday,
		"thursday":  time.Thursday,
		"friday":    time.Friday,
		"saturday":  time.Saturday,
	}
	weekday, ok := weekdays[strings.ToLower(strings.TrimSpace(value))]
	if !ok {
		return 0, invalidRequest(fmt.Sprintf("unsupported preferred weekday %q", value))
	}
	return weekday, nil
}

func pageItems[T any](items []T, pageSize int32, token string) ([]T, string, error) {
	offset := 0
	if token != "" {
		parsed, err := strconv.Atoi(token)
		if err != nil || parsed < 0 {
			return nil, "", invalidRequest("page_token is invalid")
		}
		offset = parsed
	}
	if offset > len(items) {
		return nil, "", invalidRequest("page_token is outside the result set")
	}
	limit := int(pageSize)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	nextToken := ""
	if end < len(items) {
		nextToken = strconv.Itoa(end)
	}
	return items[offset:end], nextToken, nil
}
