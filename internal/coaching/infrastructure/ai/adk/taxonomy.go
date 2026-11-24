package adk

import (
	"context"
	"log/slog"
	"strings"

	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// catalogPrefetchLimit covers the whole free-exercise-db catalog (876 exercises).
const catalogPrefetchLimit = 1000

// codeStrings renders taxonomy codes for the prompt.
func codeStrings[T ~string](codes []T) []string {
	if len(codes) == 0 {
		return nil
	}
	out := make([]string, len(codes))
	for i, c := range codes {
		out[i] = string(c)
	}
	return out
}

// injuryDTOs renders injuries for the prompt.
func injuryDTOs(injuries []service.Injury) []InjuryStatus {
	if len(injuries) == 0 {
		return nil
	}
	out := make([]InjuryStatus, len(injuries))
	for i, inj := range injuries {
		out[i] = InjuryStatus{Region: string(inj.Region), Muscle: string(inj.Muscle), Severity: string(inj.Severity)}
	}
	return out
}

// domainInjuries parses the prompt injuries back for the eligibility rules.
// They are built by the backend, so an unknown code is a bug: it is logged and
// the field dropped, an injury left with neither region nor muscle is skipped.
func domainInjuries(ctx context.Context, dtos []InjuryStatus) []service.Injury {
	out := make([]service.Injury, 0, len(dtos))
	for _, dto := range dtos {
		var inj service.Injury
		if dto.Region != "" {
			inj.Region, _ = parseLLMValue(ctx, "injury.region", dto.Region, "coach_input", taxonomy.ParseJointRegion)
		}
		if dto.Muscle != "" {
			inj.Muscle, _ = parseLLMValue(ctx, "injury.muscle", dto.Muscle, "coach_input", taxonomy.ParseMuscleGroup)
		}
		severity, ok := parseLLMValue(ctx, "injury.severity", dto.Severity, "coach_input", taxonomy.ParseInjurySeverity)
		if !ok || (inj.Region == "" && inj.Muscle == "") {
			continue
		}
		inj.Severity = severity
		out = append(out, inj)
	}
	return out
}

// parseLLMValue parses one taxonomy code from model or tool input, accepting
// case and spacing variants ("lower back" -> LOWER_BACK). An unknown value is
// dropped and logged with its field, value and source (docs/05 §7).
func parseLLMValue[T ~string](ctx context.Context, field, value, source string, parse func(string) (T, error)) (T, bool) {
	code := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", "_"))
	v, err := parse(code)
	if err != nil {
		slog.WarnContext(ctx, "Dropping unknown taxonomy value",
			slog.String("field", field), slog.String("value", value), slog.String("source", source))
		return v, false
	}
	return v, true
}

// parseLLMValues parses a list, dropping unknown values.
func parseLLMValues[T ~string](ctx context.Context, field string, values []string, source string, parse func(string) (T, error)) []T {
	var out []T
	for _, value := range values {
		if v, ok := parseLLMValue(ctx, field, value, source, parse); ok {
			out = append(out, v)
		}
	}
	return out
}

// athleteFromInput rebuilds the eligibility view of the athlete from the
// prompt input, which is what tools and later stages can see.
func athleteFromInput(ctx context.Context, in *CoachInput) service.Athlete {
	return service.Athlete{
		Equipment: parseLLMValues(ctx, "available_equipment", in.Profile.AvailableEquipment, "coach_input", taxonomy.ParseEquipment),
		Level:     service.UserLevel(in.Profile.ExperienceLevel),
		Injuries:  domainInjuries(ctx, in.Profile.ActiveInjuries),
	}
}
