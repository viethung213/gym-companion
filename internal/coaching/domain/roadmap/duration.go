package roadmap

// Timing assumptions behind EstimatedMinutes.
const (
	secondsPerRep     = 4  // a controlled rep, lowering included
	secondsUnknownSet = 30 // a set that states neither reps nor a duration
	minutesRounding   = 5
)

// EstimatedMinutes is how long the prescription takes to perform: every set of
// every exercise (warm-ups and cool-downs included), the rest between its
// sets and the rest after it. It is rounded up to 5 minutes and kept within
// MinSessionMinutes..MaxSessionMinutes.
//
// The length of a session follows from what it prescribes, not from the slot
// it is booked in nor from the model's own estimate: a session shortened by
// the adaptive cycle must show its new length.
func (p WorkoutPrescription) EstimatedMinutes() int32 {
	minutes := (p.performSeconds() + 59) / 60
	minutes = (minutes + minutesRounding - 1) / minutesRounding * minutesRounding
	return int32(min(max(minutes, MinSessionMinutes), MaxSessionMinutes))
}

// performSeconds is the time every line takes, rests included, uncapped.
func (p WorkoutPrescription) performSeconds() int64 {
	var seconds int64
	for _, list := range [][]PrescribedExercise{p.WarmUps, p.MainExercises, p.CoolDowns} {
		for i := range list {
			seconds += exerciseSeconds(&list[i])
		}
	}
	return seconds
}

func exerciseSeconds(e *PrescribedExercise) int64 {
	sets := int64(max(e.TargetSets, 1))
	work := int64(e.DurationSeconds)
	if work <= 0 {
		work = int64(e.TargetReps) * secondsPerRep
	}
	if work <= 0 {
		work = secondsUnknownSet
	}
	return sets*work + (sets-1)*int64(max(e.RestSetSec, 0)) + int64(max(e.RestExerciseSec, 0))
}
