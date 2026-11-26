package adk

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"google.golang.org/genai"
)

// jsonFields lists the JSON names of a struct's exported fields.
func jsonFields(t reflect.Type) []string {
	var out []string
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name != "" && name != "-" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func schemaFields(s *genai.Schema) []string {
	out := make([]string, 0, len(s.Properties))
	for k := range s.Properties {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// The schema is what the model may write, so it must stay in step with the
// DTOs the backend parses: a field added to one and not the other is either
// never produced or silently dropped.
func TestGeneratedPlanSchemaMatchesTheDTOs(t *testing.T) {
	root := buildGeneratedPlanSchema()
	week := root.Properties["weeks"].Items
	session := week.Properties["sessions"].Items
	prescription := session.Properties["prescription"]
	exercise := prescription.Properties["main_exercises"].Items

	cases := []struct {
		name   string
		schema *genai.Schema
		dto    reflect.Type
		// Fields the backend sets itself and never asks the model for.
		backendOnly []string
	}{
		{"week", week, reflect.TypeOf(WeekPlan{}), []string{"phase", "target_rpe_max", "target_rpe_min"}},
		{"session", session, reflect.TypeOf(SessionPlan{}), nil},
		{"prescription", prescription, reflect.TypeOf(WorkoutPrescription{}), nil},
		{"exercise", exercise, reflect.TypeOf(PrescribedExercise{}), nil},
	}
	for _, tc := range cases {
		want := make([]string, 0)
		skip := make(map[string]bool)
		for _, f := range tc.backendOnly {
			skip[f] = true
		}
		for _, f := range jsonFields(tc.dto) {
			if !skip[f] {
				want = append(want, f)
			}
		}
		if got := schemaFields(tc.schema); !reflect.DeepEqual(got, want) {
			t.Errorf("%s schema fields %v, want the DTO's %v", tc.name, got, want)
		}
		for _, r := range tc.schema.Required {
			if tc.schema.Properties[r] == nil {
				t.Errorf("%s requires %q, which is not a property", tc.name, r)
			}
		}
	}
}
