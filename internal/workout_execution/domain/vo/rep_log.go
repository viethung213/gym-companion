package vo

// RepLog represents the tracking metrics for an individual repetition during an AI camera set.
type RepLog struct {
	RepNumber     int
	ROMPercentage float32
	ErrorCodes    []string
	JointAngles   map[string]float32
}

// NewRepLog creates a new RepLog with slice and map defensive copying.
func NewRepLog(
	repNumber int,
	romPercentage float32,
	errorCodes []string,
	jointAngles map[string]float32,
) RepLog {
	var copiedErrors []string
	if len(errorCodes) > 0 {
		copiedErrors = make([]string, len(errorCodes))
		copy(copiedErrors, errorCodes)
	}

	var copiedAngles map[string]float32
	if jointAngles != nil {
		copiedAngles = make(map[string]float32, len(jointAngles))
		for k, v := range jointAngles {
			copiedAngles[k] = v
		}
	}

	return RepLog{
		RepNumber:     repNumber,
		ROMPercentage: romPercentage,
		ErrorCodes:    copiedErrors,
		JointAngles:   copiedAngles,
	}
}

// GetErrorCodes returns a defensive copy of ErrorCodes.
func (r RepLog) GetErrorCodes() []string {
	if len(r.ErrorCodes) == 0 {
		return nil
	}
	res := make([]string, len(r.ErrorCodes))
	copy(res, r.ErrorCodes)
	return res
}

// GetJointAngles returns a defensive copy of JointAngles.
func (r RepLog) GetJointAngles() map[string]float32 {
	if r.JointAngles == nil {
		return nil
	}
	res := make(map[string]float32, len(r.JointAngles))
	for k, v := range r.JointAngles {
		res[k] = v
	}
	return res
}
