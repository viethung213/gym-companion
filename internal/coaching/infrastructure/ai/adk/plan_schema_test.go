package adk

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"google.golang.org/adk/v2/workflow"
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
	root := buildGeneratedPlanSchema(false)
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

// Reproduces the live finding: asked for the 4-week roadmap, the model once
// returned weeks 1-2 only, costing a full regeneration. A whole roadmap's
// schema demands exactly weeks 1 to 4, each with a session; a rewrite's
// schema, which returns only the sessions it changes, demands none.
func TestGeneratedPlanSchema_WholeRoadmapHasFourWeeks(t *testing.T) {
	whole := buildGeneratedPlanSchema(true).Properties["weeks"]
	if whole.MinItems == nil || *whole.MinItems != 4 || whole.MaxItems == nil || *whole.MaxItems != 4 {
		t.Errorf("weeks items %v..%v, want exactly 4", whole.MinItems, whole.MaxItems)
	}
	week := whole.Items
	if n := week.Properties["week_number"]; n.Minimum == nil || *n.Minimum != 1 || n.Maximum == nil || *n.Maximum != 4 {
		t.Errorf("week_number range %v..%v, want 1..4", n.Minimum, n.Maximum)
	}
	if s := week.Properties["sessions"]; s.MinItems == nil || *s.MinItems != 1 {
		t.Errorf("sessions min items %v, want 1", s.MinItems)
	}

	rewrite := buildGeneratedPlanSchema(false).Properties["weeks"]
	if rewrite.MinItems != nil || rewrite.MaxItems != nil || rewrite.Items.Properties["week_number"].Maximum != nil {
		t.Error("a rewrite's schema bounds the weeks; it returns only the sessions it changes")
	}
}

// Only a whole roadmap uses the 4-week schema; every rewrite keeps the open one.
func TestGeneratorFor(t *testing.T) {
	whole, open, adhoc := &workflow.AgentNode{}, &workflow.AgentNode{}, &workflow.AgentNode{}
	c := &CoachingContextAgent{roadmapGeneratorNode: whole, generatorNode: open, adhocGeneratorNode: adhoc}
	for flow, want := range map[string]*workflow.AgentNode{
		FlowInitiate4Week: whole, FlowRegenerate: open, FlowAdaptiveCycle: open,
		FlowPostInjury: open, FlowSignalHandler: open, FlowSuggestAdHoc: adhoc,
	} {
		if got := c.generatorFor(&CoachInput{Flow: flow}); got != want {
			t.Errorf("%s: wrong generator", flow)
		}
	}
	hinted := &CoachInput{Flow: FlowSuggestAdHoc, AdHocHint: &AdHocHintInput{MuscleGroups: []string{"CHEST"}}}
	if c.generatorFor(hinted) != open {
		t.Error("a hinted ad-hoc session uses the structured generator")
	}
}
