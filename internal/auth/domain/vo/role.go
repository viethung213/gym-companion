package vo

import (
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
)

// Role represents a validated user system role Value Object.
type Role struct {
	value string
}

// NewRole validates and creates a Role Value Object.
func NewRole(v string) (Role, error) {
	if v != "user" && v != "admin" {
		return Role{}, derror.ErrInvalidRole
	}
	return Role{value: v}, nil
}

// Value returns the raw string value of the Role.
func (r Role) Value() string {
	return r.value
}
