package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
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
	client := profilev1serviceconnect.NewProfileServiceClient(httpClient, gatewayURL)
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

	req := connect.NewRequest(&profilemsg.GetProfileRequest{UserId: userID})
	req.Header().Set(middleware.HeaderInternalToken, tokenStr)

	res, err := c.client.GetProfile(ctx, req)
	if err != nil {
		return port.Profile{}, fmt.Errorf("gateway get profile for coaching user %s: %w", userID, err)
	}

	data := res.Msg

	profile := port.Profile{
		UserID:   data.GetUserId(),
		WeightKg: float64(data.GetWeightKg()),
		HeightCm: float64(data.GetHeightCm()),
		Age:      int(data.GetAge()),
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

	// 3. Map Available Equipment
	if len(data.GetAvailableEquipment()) > 0 {
		profile.AvailableEquipment = data.GetAvailableEquipment()
	}
	if len(profile.AvailableEquipment) == 0 {
		profile.AvailableEquipment = []string{"barbell", "bench", "cable", "dumbbell", "pull-up-bar", "rack"}
	}

	// 4. Map Preferred Muscle Groups
	if len(data.GetPreferredMuscleGroups()) > 0 {
		profile.PreferredMuscleGroups = data.GetPreferredMuscleGroups()
	}
	if len(profile.PreferredMuscleGroups) == 0 {
		profile.PreferredMuscleGroups = []string{"chest", "back", "legs", "shoulders"}
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
	if len(data.GetInjuries()) > 0 {
		injuries := make([]port.InjuryStatus, 0, len(data.GetInjuries()))
		for _, inj := range data.GetInjuries() {
			if inj == nil || inj.GetIsRecovered() {
				continue
			}
			repAt := time.Now()
			if inj.GetReportedAt() != "" {
				if parsedRep, pErr := time.Parse(time.RFC3339, inj.GetReportedAt()); pErr == nil {
					repAt = parsedRep
				}
			}
			var recAt *time.Time
			if inj.GetRecoveredAt() != "" {
				if parsedRec, rErr := time.Parse(time.RFC3339, inj.GetRecoveredAt()); rErr == nil {
					recAt = &parsedRec
				}
			}
			injuries = append(injuries, port.InjuryStatus{
				MuscleGroup: inj.GetMuscleGroup(),
				ReportedAt:  repAt,
				RecoveredAt: recAt,
				Notes:       inj.GetNotes(),
			})
		}
		profile.ActiveInjuries = injuries
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
