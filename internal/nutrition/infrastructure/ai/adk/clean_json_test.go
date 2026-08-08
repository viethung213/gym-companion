package adk

import "testing"

func TestCleanJSONResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Pure JSON Object",
			input:    `{"options":[{"protein_food_name":"Ức gà"}]}`,
			expected: `{"options":[{"protein_food_name":"Ức gà"}]}`,
		},
		{
			name:     "Markdown ```json wrapper",
			input:    "```json\n{\"options\":[{\"protein_food_name\":\"Ức gà\"}]}\n```",
			expected: `{"options":[{"protein_food_name":"Ức gà"}]}`,
		},
		{
			name:     "Markdown with leading text prose",
			input:    "Chắc chắn rồi! Dưới đây là thực đơn JSON:\n```json\n{\"options\":[]}\n```\nChúc bạn ngon miệng!",
			expected: `{"options":[]}`,
		},
		{
			name:     "JSON Array with leading spaces",
			input:    "   [{\"id\": 1}]   ",
			expected: `[{"id": 1}]`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := cleanJSONResponse(tt.input)
			if got != tt.expected {
				t.Errorf("cleanJSONResponse(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
