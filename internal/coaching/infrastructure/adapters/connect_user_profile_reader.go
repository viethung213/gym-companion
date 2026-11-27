package adapters

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
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
// It reports only what the profile holds; empty fields stay empty and
// port.WithProfileDefaults decides what coaching assumes for them.
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
	// 2. Map Goals
	if len(data.GetGoals()) > 0 && strings.TrimSpace(data.GetGoals()[0]) != "" {
		profile.PrimaryGoal = data.GetGoals()[0]
	}

	// 3. Map Available Equipment.
	for _, e := range data.GetAvailableEquipment() {
		code, mapErr := protomap.Equipment(e)
		if mapErr != nil {
			warnUnknownProfileValue(ctx, "available_equipment", e.String(), mapErr)
			continue
		}
		profile.AvailableEquipment = append(profile.AvailableEquipment, code)
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
		slots, unreadable := parseWorkoutTimesList(data.GetPreferredWorkoutTimes())
		profile.AvailableSlots = slots
		for _, item := range unreadable {
			warnUnknownProfileValue(ctx, "preferred_workout_times", item,
				errors.New("not a day and an HH:MM-HH:MM range"))
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
		Region:   region,
		Muscle:   muscle,
		Severity: severity,
		Notes:    inj.GetNotes(),
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

// parseWorkoutTimesList reads the profile's preferred workout times. Each item
// is one of: a JSON map {"mon": ["06:00-07:30"]}, a JSON list
// [{"day_of_week":1,"start_time":"06:00","end_time":"07:30"}], or a day and a
// range ("mon:06:00-07:30", the format the app sends, or "MON 06:00-07:30").
// Items that yield no valid slot are returned in unreadable so the caller can
// report them; planning then falls back to the default slots.
func parseWorkoutTimesList(rawList []string) (slots []port.Slot, unreadable []string) {
	for _, item := range rawList {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parsed := parseWorkoutTime(item)
		valid := slices.DeleteFunc(parsed, func(s port.Slot) bool { return !validSlot(s) })
		if len(valid) == 0 {
			unreadable = append(unreadable, item)
			continue
		}
		slots = append(slots, valid...)
	}
	return slots, unreadable
}

func parseWorkoutTime(item string) []port.Slot {
	var slots []port.Slot
	if strings.HasPrefix(item, "{") {
		var mapSlots map[string][]string
		if err := json.Unmarshal([]byte(item), &mapSlots); err == nil {
			for dayStr, slotStrs := range mapSlots {
				wd, ok := dayCodeToWeekday(dayStr)
				if !ok {
					continue
				}
				for _, slotStr := range slotStrs {
					start, end := splitSlotRange(slotStr)
					slots = append(slots, port.Slot{DayOfWeek: wd, StartTime: start, EndTime: end})
				}
			}
		}
		return slots
	}

	if strings.HasPrefix(item, "[") {
		var rawSlots []rawSlotItem
		if err := json.Unmarshal([]byte(item), &rawSlots); err == nil {
			for _, s := range rawSlots {
				day := s.DayOfWeek
				if day == 0 && s.Day != 0 {
					day = s.Day
				}
				startTime := cmp.Or(s.StartTime, s.Start)
				endTime := cmp.Or(s.EndTime, s.End)
				slots = append(slots, port.Slot{DayOfWeek: time.Weekday(day), StartTime: startTime, EndTime: endTime})
			}
		}
		return slots
	}

	// "mon:06:00-07:30" splits at the first colon; "MON 06:00-07:30" at the space.
	day, rng, found := strings.Cut(item, ":")
	if wd, ok := dayCodeToWeekday(day); found && ok {
		start, end := splitSlotRange(rng)
		return []port.Slot{{DayOfWeek: wd, StartTime: start, EndTime: end}}
	}
	if fields := strings.Fields(item); len(fields) == 2 {
		if wd, ok := dayCodeToWeekday(fields[0]); ok {
			start, end := splitSlotRange(fields[1])
			return []port.Slot{{DayOfWeek: wd, StartTime: start, EndTime: end}}
		}
	}
	return nil
}

// validSlot reports whether a slot names a weekday and an HH:MM start. The
// end is optional ("tue:18:00"): planning schedules by the start, and a
// dropped slot would put the athlete on default days they never chose.
func validSlot(s port.Slot) bool {
	if s.DayOfWeek < time.Sunday || s.DayOfWeek > time.Saturday {
		return false
	}
	if _, err := time.Parse("15:04", s.StartTime); err != nil {
		return false
	}
	if s.EndTime == "" {
		return true
	}
	_, err := time.Parse("15:04", s.EndTime)
	return err == nil
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
