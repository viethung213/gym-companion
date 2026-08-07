package adk

import (
	"encoding/json"
	"testing"
)

// asMap parses a verdict exactly as ADK hands it back: an untyped map.
func asMap(t *testing.T, raw string) map[string]any {
	t.Helper()

	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return out
}

// TestDecodePlanReview_LiveShapes decodes payloads captured verbatim from real
// CoachReviewerAgent runs, so the decoder is pinned against what the model
// actually emits rather than against what the schema says it should.
func TestDecodePlanReview_LiveShapes(t *testing.T) {
	t.Run("approval with notes and no feedback", func(t *testing.T) {
		t.Parallel()

		got := decodePlanReview(asMap(t, `{
			"approved": true, "score": 90, "confidence": 1,
			"feedback": [],
			"notes": ["Volume progression scales cleanly", "Covers all preferred muscle groups"],
			"previous_feedback": []
		}`))

		if got == nil {
			t.Fatal("verdict = nil")
		}
		if !got.Approved || got.Score != 90 || got.Confidence != 1 {
			t.Errorf("verdict = %+v, want approved/90/1.0", got)
		}
		if len(got.Notes) != 2 {
			t.Errorf("notes = %v, want 2", got.Notes)
		}
		if len(got.Feedback) != 0 {
			t.Errorf("feedback = %+v, want empty", got.Feedback)
		}
	})

	t.Run("rejection accounting for the previous round", func(t *testing.T) {
		t.Parallel()

		got := decodePlanReview(asMap(t, `{
			"approved": false, "score": 60, "confidence": 0.9,
			"previous_feedback": [
				{"area": "goal_fit", "applied": true, "evidence": "all 12 sessions now carry 3 main exercises"},
				{"area": "warmup_specificity", "applied": false, "evidence": "week 1 back session still warms up with pull-up"}
			],
			"feedback": [
				{"area": "warmup_specificity", "detail": "back sessions reuse pull-up", "fix": "use band-pull-apart before rows"}
			]
		}`))

		if got == nil {
			t.Fatal("verdict = nil")
		}
		if got.Approved || got.Score != 60 {
			t.Errorf("verdict = %+v, want rejected/60", got)
		}
		if len(got.PreviousFeedback) != 2 {
			t.Fatalf("previous_feedback = %+v, want 2", got.PreviousFeedback)
		}
		if got.PreviousFeedback[0].Area != "goal_fit" || !got.PreviousFeedback[0].Applied {
			t.Errorf("previous_feedback[0] = %+v, want goal_fit applied", got.PreviousFeedback[0])
		}
		if got.PreviousFeedback[1].Applied {
			t.Errorf("previous_feedback[1] = %+v, want warmup_specificity NOT applied", got.PreviousFeedback[1])
		}
		if len(got.Feedback) != 1 || got.Feedback[0].Fix == "" {
			t.Errorf("feedback = %+v, want one entry carrying a fix", got.Feedback)
		}

		// The whole point of the compliance check: this verdict must survive
		// validation, and the same one flipped to approved must not.
		prior := []ReviewNote{{Area: "goal_fit"}, {Area: "warmup_specificity"}}
		if err := validateReview(got, prior); err != nil {
			t.Errorf("validateReview = %v, want usable", err)
		}

		got.Approved = true
		got.Score = 100
		if err := validateReview(got, prior); err == nil {
			t.Error("validateReview accepted an approval with warmup_specificity still unapplied")
		}
	})

	t.Run("score arrives as a JSON number, not an int", func(t *testing.T) {
		t.Parallel()

		// json.Unmarshal into any yields float64, which is what tripped the
		// hand-written decoder into needing a numeric conversion helper.
		got := decodePlanReview(asMap(t, `{"approved": true, "score": 85, "confidence": 0.75}`))

		if got == nil {
			t.Fatal("verdict = nil")
		}
		if got.Score != 85 {
			t.Errorf("score = %d, want 85", got.Score)
		}
		if got.Confidence != 0.75 {
			t.Errorf("confidence = %v, want 0.75", got.Confidence)
		}
	})
}

func TestDecodePlanReview_Rejects(t *testing.T) {
	tests := []struct {
		name string
		give map[string]any
	}{
		{name: "nil map"},
		{name: "score of the wrong type", give: map[string]any{"score": "ninety"}},
		{name: "feedback that is not a list", give: map[string]any{"feedback": "looks fine"}},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// A malformed verdict must decode to nil so validateReview reports it,
			// rather than to a half-filled struct that steers the loop.
			if got := decodePlanReview(tt.give); got != nil {
				t.Errorf("verdict = %+v, want nil", got)
			}
		})
	}
}
