package domain

// PrescriptionPolicy specifies default volume rules by experience and category.
type PrescriptionPolicy struct{}

type PrescriptionSpec struct {
	TargetSets int
	TargetReps int
}

func ResolvePrescriptionSpec(experienceLevel, category string) PrescriptionSpec {
	isCompound := category == "Compound"

	switch experienceLevel {
	case "intermediate":
		if isCompound {
			return PrescriptionSpec{TargetSets: 4, TargetReps: 8}
		}
		return PrescriptionSpec{TargetSets: 3, TargetReps: 12}
	case "advanced":
		if isCompound {
			return PrescriptionSpec{TargetSets: 5, TargetReps: 6}
		}
		return PrescriptionSpec{TargetSets: 4, TargetReps: 10}
	default: // "beginner" or fallback
		if isCompound {
			return PrescriptionSpec{TargetSets: 3, TargetReps: 10}
		}
		return PrescriptionSpec{TargetSets: 2, TargetReps: 12}
	}
}
