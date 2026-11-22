package adk

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

func TestIsDifficultyAllowed(t *testing.T) {
	cases := []struct {
		name      string
		exDiff    string
		userLevel string
		want      bool
	}{
		{"beginner allows beginner", "Beginner", "BEGINNER", true},
		{"beginner rejects intermediate", "Intermediate", "BEGINNER", false},
		{"beginner rejects advanced", "Advanced", "BEGINNER", false},
		{"intermediate allows beginner", "Beginner", "INTERMEDIATE", true},
		{"intermediate allows intermediate", "Intermediate", "INTERMEDIATE", true},
		{"intermediate rejects advanced", "Advanced", "INTERMEDIATE", false},
		{"advanced allows beginner", "Beginner", "ADVANCED", true},
		{"advanced allows intermediate", "Intermediate", "ADVANCED", true},
		{"advanced allows advanced", "Advanced", "ADVANCED", true},
		{"empty user level allows any", "Advanced", "", true},
		{"empty exercise difficulty allowed", "", "BEGINNER", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDifficultyAllowed(tc.exDiff, tc.userLevel); got != tc.want {
				t.Errorf("isDifficultyAllowed(%q, %q) = %v, want %v", tc.exDiff, tc.userLevel, got, tc.want)
			}
		})
	}
}

type bodyPartFilterCatalog struct {
	exercises []port.Exercise
}

func (c *bodyPartFilterCatalog) SearchByFilter(_ context.Context, f *port.ExerciseFilter) ([]port.Exercise, error) {
	out := make([]port.Exercise, 0, len(c.exercises))
	for i := range c.exercises {
		ex := &c.exercises[i]
		if f.TargetMuscleID != "" && ex.MuscleGroup != f.TargetMuscleID {
			continue
		}
		if f.BodyPartID != "" && ex.BodyPart != f.BodyPartID {
			continue
		}
		out = append(out, *ex)
	}
	return out, nil
}

func (c *bodyPartFilterCatalog) GetByID(_ context.Context, id string) (port.Exercise, error) {
	for i := range c.exercises {
		if c.exercises[i].ExerciseID == id {
			return c.exercises[i], nil
		}
	}
	return port.Exercise{}, port.ErrExerciseNotFound
}

func TestPrefetchAdHocExercises_BodyPartFallbackAndDifficultyCap(t *testing.T) {
	cat := &bodyPartFilterCatalog{
		exercises: []port.Exercise{
			{
				ExerciseID:  "ex-beg",
				Name:        "Push-Up",
				MuscleGroup: "pectorals-id",
				BodyPart:    "bodypart-chest-id",
				Equipment:   "Body Weight",
				Difficulty:  "Beginner",
			},
			{
				ExerciseID:  "ex-adv",
				Name:        "Planche Push-Up",
				MuscleGroup: "pectorals-id",
				BodyPart:    "bodypart-chest-id",
				Equipment:   "Body Weight",
				Difficulty:  "Advanced",
			},
		},
	}

	agent := &CoachingContextAgent{
		catalog: cat,
		muscleNames: map[string]string{
			"muscle-chest-id": "Chest",
			"pectorals-id":    "Pectorals",
		},
		bodyPartNames: map[string]string{
			"bodypart-chest-id": "Chest",
		},
		equipmentNames: map[string]string{},
	}

	candidates := make(map[string][][]string)
	added := agent.prefetchAdHocExercises(
		context.Background(),
		[]string{"Chest"},
		nil,
		nil,
		"BEGINNER",
		candidates,
	)

	if added != 1 {
		t.Fatalf("got added=%d, want 1 (body_part fallback + beginner cap)", added)
	}
	if len(candidates["Chest"]) != 1 || candidates["Chest"][0][0] != "ex-beg" {
		t.Fatalf("got candidates[Chest]=%v, want only ex-beg", candidates["Chest"])
	}
}
