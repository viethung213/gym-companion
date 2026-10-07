package vo

import (
	"database/sql/driver"
	"fmt"
)

type ReactionType string

const (
	ReactionTypeLike   ReactionType = "LIKE"
	ReactionTypeFire   ReactionType = "FIRE"
	ReactionTypeMuscle ReactionType = "MUSCLE"
	ReactionTypeClap   ReactionType = "CLAP"
)

func NewReactionType(s string) (ReactionType, error) {
	switch s {
	case string(ReactionTypeLike):
		return ReactionTypeLike, nil
	case string(ReactionTypeFire):
		return ReactionTypeFire, nil
	case string(ReactionTypeMuscle):
		return ReactionTypeMuscle, nil
	case string(ReactionTypeClap):
		return ReactionTypeClap, nil
	default:
		return "", fmt.Errorf("unknown reaction type: %s", s)
	}
}

func (r ReactionType) String() string {
	return string(r)
}

func (r ReactionType) Value() (driver.Value, error) {
	return string(r), nil
}

func (r *ReactionType) Scan(value any) error {
	if value == nil {
		*r = ""
		return nil
	}
	switch v := value.(type) {
	case string:
		*r = ReactionType(v)
	case []byte:
		*r = ReactionType(string(v))
	default:
		return fmt.Errorf("cannot scan %T into ReactionType", value)
	}
	return nil
}
