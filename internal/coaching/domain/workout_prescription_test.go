package domain

import (
	"errors"
	"testing"
)

func TestNewWorkoutPrescription(t *testing.T) {
	t.Parallel()

	type giveStruct struct {
		sets        int
		reps        int
		weight      float64
		rpe         float64
		restSeconds int
	}

	tests := []struct {
		name    string
		give    giveStruct
		wantErr error
	}{
		{
			name: "valid prescription",
			give: giveStruct{
				sets:        3,
				reps:        10,
				weight:      60.5,
				rpe:         8.0,
				restSeconds: 90,
			},
			wantErr: nil,
		},
		{
			name: "sets zero",
			give: giveStruct{
				sets:        0,
				reps:        10,
				weight:      60,
				rpe:         8,
				restSeconds: 60,
			},
			wantErr: ErrInvalidPrescription,
		},
		{
			name: "reps zero",
			give: giveStruct{
				sets:        3,
				reps:        0,
				weight:      60,
				rpe:         8,
				restSeconds: 60,
			},
			wantErr: ErrInvalidPrescription,
		},
		{
			name: "negative weight",
			give: giveStruct{
				sets:        3,
				reps:        10,
				weight:      -5,
				rpe:         8,
				restSeconds: 60,
			},
			wantErr: ErrInvalidPrescription,
		},
		{
			name: "rpe negative",
			give: giveStruct{
				sets:        3,
				reps:        10,
				weight:      60,
				rpe:         -1,
				restSeconds: 60,
			},
			wantErr: ErrInvalidPrescription,
		},
		{
			name: "rpe greater than 10",
			give: giveStruct{
				sets:        3,
				reps:        10,
				weight:      60,
				rpe:         10.5,
				restSeconds: 60,
			},
			wantErr: ErrInvalidPrescription,
		},
		{
			name: "negative rest seconds",
			give: giveStruct{
				sets:        3,
				reps:        10,
				weight:      60,
				rpe:         8,
				restSeconds: -10,
			},
			wantErr: ErrInvalidPrescription,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewWorkoutPrescription(
				tt.give.sets,
				tt.give.reps,
				tt.give.weight,
				tt.give.rpe,
				tt.give.restSeconds,
			)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("NewWorkoutPrescription() got err nil, wantErr %v", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("NewWorkoutPrescription() got err %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewWorkoutPrescription() unexpected error = %v", err)
			}

			if got.Sets() != tt.give.sets {
				t.Errorf("got Sets %v, want %v", got.Sets(), tt.give.sets)
			}
			if got.Reps() != tt.give.reps {
				t.Errorf("got Reps %v, want %v", got.Reps(), tt.give.reps)
			}
			if got.Weight() != tt.give.weight {
				t.Errorf("got Weight %v, want %v", got.Weight(), tt.give.weight)
			}
			if got.RPE() != tt.give.rpe {
				t.Errorf("got RPE %v, want %v", got.RPE(), tt.give.rpe)
			}
			if got.RestSeconds() != tt.give.restSeconds {
				t.Errorf("got RestSeconds %v, want %v", got.RestSeconds(), tt.give.restSeconds)
			}
		})
	}
}
