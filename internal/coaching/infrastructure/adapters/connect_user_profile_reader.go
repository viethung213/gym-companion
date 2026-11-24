package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/protomap"
	profilemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/service/profilev1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

var _ port.UserProfileReader = (*ConnectUserProfileReader)(nil)

// ConnectUserProfileReader queries user profile from Profile Bounded Context via API Gateway.
type ConnectUserProfileReader struct {
	client         profilev1serviceconnect.ProfileServiceClient
	internalSecret string
}

// NewConnectUserProfileReader creates a new ConnectUserProfileReader communicating through API Gateway.
func NewConnectUserProfileReader(gatewayURL, internalSecret string, httpClient *http.Client) *ConnectUserProfileReader {
	if httpClient == nil {
		httpClient = newDefaultHTTPClient()
	}
	transport := connecthttp.NewTransport(httpClient, gatewayURL)
	client := profilev1serviceconnect.NewProfileServiceClient(connect.NewClient(transport))
	return &ConnectUserProfileReader{
		client:         client,
		internalSecret: internalSecret,
	}
}

// GetProfile retrieves user profile through ConnectRPC over API Gateway.
func (c *ConnectUserProfileReader) GetProfile(ctx context.Context, userID string) (port.Profile, error) {
	if userID == "" {
		return port.Profile{}, errors.New("user id cannot be empty")
	}

	tokenStr, err := internaltoken.Generate(userID, "Admin", "ai-coaching", time.Minute, c.internalSecret)
	if err != nil {
		return port.Profile{}, fmt.Errorf("generate internal token: %w", err)
	}

	req := &profilemsg.GetProfileRequest{UserId: userID}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set(middleware.HeaderInternalToken, tokenStr)

	res, err := c.client.GetProfile(ctx, req)
	if err != nil {
		return port.Profile{}, fmt.Errorf("gateway get profile for coaching user %s: %w", userID, err)
	}

	data := res

	profile := port.Profile{
		UserID:          data.GetUserId(),
		WeightKg:        float64(data.GetWeightKg()),
		HeightCm:        float64(data.GetHeightCm()),
		Age:             int(data.GetAge()),
		ExperienceLevel: strings.TrimSpace(data.GetExperienceLevel()),
	}

	if profile.UserID == "" {
		profile.UserID = userID
	}
	if profile.WeightKg <= 0 {
		profile.WeightKg = 75.0
	}
	if profile.HeightCm <= 0 {
		profile.HeightCm = 170.0
	}

	// 1. Calculate Age if missing but DateOfBirth provided
	if profile.Age <= 0 && data.GetDateOfBirth() != "" {
		if dob, parseErr := time.Parse("2006-01-02", data.GetDateOfBirth()); parseErr == nil {
			now := time.Now()
			age := now.Year() - dob.Year()
			if now.YearDay() < dob.YearDay() {
				age--
			}
			if age > 0 {
				profile.Age = age
			}
		}
	}

	// 2. Map Goals
	if len(data.GetGoals()) > 0 && strings.TrimSpace(data.GetGoals()[0]) != "" {
		profile.PrimaryGoal = data.GetGoals()[0]
	}
	if profile.PrimaryGoal == "" {
		profile.PrimaryGoal = "hypertrophy"
	}

	// 3. Map Available Equipment; an empty list means a regular gym (docs/05 §8.3).
	for _, e := range data.GetAvailableEquipment() {
		code, mapErr := protomap.Equipment(e)
		if mapErr != nil {
			warnUnknownProfileValue(ctx, "available_equipment", e.String(), mapErr)
			continue
		}
		profile.AvailableEquipment = append(profile.AvailableEquipment, code)
	}
	if len(profile.AvailableEquipment) == 0 {
		profile.AvailableEquipment = service.DefaultGymEquipment()
	}

	// 4. Map Preferred Muscle Groups; empty means no emphasis.
	for _, m := range data.GetPreferredMuscleGroups() {
		code, mapErr := protomap.MuscleGroup(m)
		if mapErr != nil {
			warnUnknownProfileValue(ctx, "preferred_muscle_groups", m.String(), mapErr)
			continue
		}
		profile.PreferredMuscleGroups = append(profile.PreferredMuscleGroups, code)
	}

	// Slot times are wall clock in the athlete's zone; an unknown zone falls back to UTC.
	profile.Location = roadmap.LoadLocation(data.GetTimezone())
	if tz := data.GetTimezone(); tz != "" && profile.Location.String() != tz {
		warnUnknownProfileValue(ctx, "timezone", tz, errors.New("unknown IANA zone"))
	}

	// 5. Parse Preferred Workout Times into AvailableSlots
	if len(data.GetPreferredWorkoutTimes()) > 0 {
		profile.AvailableSlots = parseWorkoutTimesList(data.GetPreferredWorkoutTimes())
	}
	if len(profile.AvailableSlots) == 0 {
		profile.AvailableSlots = []port.Slot{
			{DayOfWeek: time.Monday, StartTime: "06:00", EndTime: "07:30"},
			{DayOfWeek: time.Wednesday, StartTime: "06:00", EndTime: "07:30"},
			{DayOfWeek: time.Friday, StartTime: "06:00", EndTime: "07:30"},
		}
	}

	// 6. Map Active Injuries
	for _, inj := range data.GetInjuries() {
		if inj == nil || inj.GetIsRecovered() {
			continue
		}
		status, mapErr := mapInjury(inj)
		if mapErr != nil {
			warnUnknownProfileValue(ctx, "injuries", inj.GetInjuryId(), mapErr)
			continue
		}
		profile.ActiveInjuries = append(profile.ActiveInjuries, status)
	}

	// 7. Map UpdatedAt
	if data.GetUpdatedAt() != "" {
		if parsedUpdated, uErr := time.Parse(time.RFC3339, data.GetUpdatedAt()); uErr == nil {
			profile.UpdatedAt = parsedUpdated
		}
	}
	if profile.UpdatedAt.IsZero() {
		profile.UpdatedAt = time.Now()
	}

	return profile, nil
}

// mapInjury converts one profile injury; region and severity are required.
func mapInjury(inj *profilemsg.Injury) (port.InjuryStatus, error) {
	region, err := protomap.JointRegion(inj.GetRegion())
	if err != nil {
		return port.InjuryStatus{}, fmt.Errorf("region: %w", err)
	}
	muscle, err := protomap.OptionalMuscleGroup(inj.GetMuscle())
	if err != nil {
		return port.InjuryStatus{}, fmt.Errorf("muscle: %w", err)
	}
	severity, err := protomap.InjurySeverity(inj.GetSeverity())
	if err != nil {
		return port.InjuryStatus{}, fmt.Errorf("severity: %w", err)
	}

	status := port.InjuryStatus{
		Region:     region,
		Muscle:     muscle,
		Severity:   severity,
		ReportedAt: time.Now(),
		Notes:      inj.GetNotes(),
	}
	if parsed, pErr := time.Parse(time.RFC3339, inj.GetReportedAt()); pErr == nil {
		status.ReportedAt = parsed
	}
	if parsed, rErr := time.Parse(time.RFC3339, inj.GetRecoveredAt()); rErr == nil {
		status.RecoveredAt = &parsed
	}
	return status, nil
}

// warnUnknownProfileValue logs a profile value outside the vocabulary; the
// value is dropped (docs/05 §7).
func warnUnknownProfileValue(ctx context.Context, field, value string, err error) {
	slog.WarnContext(ctx, "Dropping unknown profile value",
		slog.String("field", field), slog.String("value", value),
		slog.String("source", "profile"), slog.String("error", err.Error()))
}

type rawSlotItem struct {
	DayOfWeek int    `json:"day_of_week"`
	Day       int    `json:"day"`
	StartTime string `json:"start_time"`
	Start     string `json:"start"`
	EndTime   string `json:"end_time"`
	End       string `json:"end"`
}

func parseWorkoutTimesList(rawList []string) []port.Slot {
	var slots []port.Slot

	for _, item := range rawList {
		itemTrimmed := strings.TrimSpace(item)
		if itemTrimmed == "" {
			continue
		}

		// Check if item is a JSON object or array
		if strings.HasPrefix(itemTrimmed, "{") {
			// Try JSON Map: {"mon": ["06:00-07:30"]}
			var mapSlots map[string][]string
			if err := json.Unmarshal([]byte(itemTrimmed), &mapSlots); err == nil && len(mapSlots) > 0 {
				for dayStr, slotStrs := range mapSlots {
					if wd, ok := dayCodeToWeekday(dayStr); ok {
						for _, slotStr := range slotStrs {
							start, end := splitSlotRange(slotStr)
							slots = append(slots, port.Slot{
								DayOfWeek: wd,
								StartTime: start,
								EndTime:   end,
							})
						}
					}
				}
				continue
			}
		}

		if strings.HasPrefix(itemTrimmed, "[") {
			// Try rawSlot slice: [{"day_of_week":1, "start_time":"06:00", "end_time":"07:30"}]
			var rawSlots []rawSlotItem
			if err := json.Unmarshal([]byte(itemTrimmed), &rawSlots); err == nil && len(rawSlots) > 0 {
				for _, s := range rawSlots {
					day := s.DayOfWeek
					if day == 0 && s.Day != 0 {
						day = s.Day
					}
					startTime := s.StartTime
					if startTime == "" {
						startTime = s.Start
					}
					endTime := s.EndTime
					if endTime == "" {
						endTime = s.End
					}
					slots = append(slots, port.Slot{
						DayOfWeek: time.Weekday(day),
						StartTime: startTime,
						EndTime:   endTime,
					})
				}
				continue
			}
		}

		// Direct format: "mon: 06:00-07:30" or "Monday: 18:00-19:30"
		parts := strings.SplitN(itemTrimmed, ":", 2)
		if len(parts) == 2 {
			if wd, ok := dayCodeToWeekday(parts[0]); ok {
				start, end := splitSlotRange(parts[1])
				slots = append(slots, port.Slot{
					DayOfWeek: wd,
					StartTime: start,
					EndTime:   end,
				})
			}
		}
	}

	return slots
}

func dayCodeToWeekday(code string) (time.Weekday, bool) {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "mon", "monday", "1":
		return time.Monday, true
	case "tue", "tuesday", "2":
		return time.Tuesday, true
	case "wed", "wednesday", "3":
		return time.Wednesday, true
	case "thu", "thursday", "4":
		return time.Thursday, true
	case "fri", "friday", "5":
		return time.Friday, true
	case "sat", "saturday", "6":
		return time.Saturday, true
	case "sun", "sunday", "0", "7":
		return time.Sunday, true
	default:
		return time.Sunday, false
	}
}

func splitSlotRange(s string) (start, end string) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(s), ""
}
