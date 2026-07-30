package domain

import "errors"

var (
	ErrInvalidBodyPart    = errors.New("invalid body part")
	ErrBodyPartNotFound   = errors.New("body part not found")
	ErrBodyPartInUse      = errors.New("body part is in use and cannot be deleted")

	ErrInvalidEquipment   = errors.New("invalid equipment")
	ErrEquipmentNotFound  = errors.New("equipment not found")
	ErrEquipmentInUse     = errors.New("equipment is in use and cannot be deleted")

	ErrInvalidMuscle      = errors.New("invalid muscle")
	ErrMuscleNotFound     = errors.New("muscle not found")
	ErrMuscleInUse        = errors.New("muscle is in use and cannot be deleted")

	ErrInvalidTag         = errors.New("invalid tag")
	ErrTagNotFound        = errors.New("tag not found")
	ErrTagInUse           = errors.New("tag is in use and cannot be deleted")
)
