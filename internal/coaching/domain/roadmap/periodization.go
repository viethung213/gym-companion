package roadmap

// The 4-week block is fixed: the backend decides each week's phase and its
// intensity window, the model only fills the sessions in.

// PhaseForWeek returns the phase of a 1-based week of the block, or "" for a
// week number outside 1..WeeksPerRoadmap.
func PhaseForWeek(weekNumber int) Phase {
	switch weekNumber {
	case 1:
		return PhaseAccumulation
	case 2:
		return PhaseOverload
	case 3:
		return PhasePeak
	case 4:
		return PhaseDeload
	default:
		return ""
	}
}

// RPEBand is an inclusive target-RPE window.
type RPEBand struct {
	Min float32
	Max float32
}

// Contains reports whether rpe lies inside the band.
func (b RPEBand) Contains(rpe float32) bool { return rpe >= b.Min && rpe <= b.Max }

// Clamp moves rpe into the band.
func (b RPEBand) Clamp(rpe float32) float32 { return min(max(rpe, b.Min), b.Max) }

// Capped lowers the band to an injury RPE cap; a cap below the band's floor
// accepts any RPE up to the cap. A cap <= 0 means no cap.
func (b RPEBand) Capped(maxRPE float32) RPEBand {
	if maxRPE <= 0 || maxRPE >= b.Max {
		return b
	}
	b.Max = maxRPE
	if b.Min > maxRPE {
		b.Min = 0
	}
	return b
}

// PhaseRPEBand is the target-RPE window of a phase's main work (BR-AC-10).
func PhaseRPEBand(p Phase) (RPEBand, bool) {
	switch p {
	case PhaseAccumulation:
		return RPEBand{Min: 6, Max: 7}, true
	case PhaseOverload:
		return RPEBand{Min: 7, Max: 8}, true
	case PhasePeak:
		return RPEBand{Min: 8, Max: 9}, true
	case PhaseDeload:
		return RPEBand{Min: 5, Max: 6}, true
	default:
		return RPEBand{}, false
	}
}

// RecoveryRPEBand is the target-RPE window of an active-recovery session.
func RecoveryRPEBand() RPEBand { return RPEBand{Min: 1, Max: RecoveryMaxRPE} }

// DeloadVolumeRatio is the share of the PEAK week's working sets the DELOAD
// week may keep (BR-AC-11).
const DeloadVolumeRatio = 0.7
