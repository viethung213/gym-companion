package vo

type WorkoutMetrics struct {
	sessionID       string
	workoutTitle    string
	durationSeconds int32
	totalVolumeKg   float64
	exerciseCount   int32
	totalSets       int32
	prCount         int32
}

func NewWorkoutMetrics(
	sessionID string,
	workoutTitle string,
	durationSeconds int32,
	totalVolumeKg float64,
	exerciseCount int32,
	totalSets int32,
	prCount int32,
) WorkoutMetrics {
	if durationSeconds < 0 {
		durationSeconds = 0
	}
	if totalVolumeKg < 0 {
		totalVolumeKg = 0
	}
	if exerciseCount < 0 {
		exerciseCount = 0
	}
	if totalSets < 0 {
		totalSets = 0
	}
	if prCount < 0 {
		prCount = 0
	}
	return WorkoutMetrics{
		sessionID:       sessionID,
		workoutTitle:    workoutTitle,
		durationSeconds: durationSeconds,
		totalVolumeKg:   totalVolumeKg,
		exerciseCount:   exerciseCount,
		totalSets:       totalSets,
		prCount:         prCount,
	}
}

func (m WorkoutMetrics) SessionID() string      { return m.sessionID }
func (m WorkoutMetrics) WorkoutTitle() string   { return m.workoutTitle }
func (m WorkoutMetrics) DurationSeconds() int32 { return m.durationSeconds }
func (m WorkoutMetrics) TotalVolumeKg() float64 { return m.totalVolumeKg }
func (m WorkoutMetrics) ExerciseCount() int32   { return m.exerciseCount }
func (m WorkoutMetrics) TotalSets() int32       { return m.totalSets }
func (m WorkoutMetrics) PRCount() int32         { return m.prCount }
func (m WorkoutMetrics) IsZero() bool {
	return m.sessionID == "" && m.totalVolumeKg == 0 && m.durationSeconds == 0
}
