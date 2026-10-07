package vo

import "fmt"

type TargetType string

const (
	TargetTypePost     TargetType = "POST"
	TargetTypeActivity TargetType = "ACTIVITY"
)

func NewTargetType(s string) (TargetType, error) {
	switch s {
	case string(TargetTypePost):
		return TargetTypePost, nil
	case string(TargetTypeActivity):
		return TargetTypeActivity, nil
	default:
		return "", fmt.Errorf("unknown target type: %s", s)
	}
}

func (t TargetType) String() string {
	return string(t)
}
