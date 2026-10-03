package vo

import (
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
	RoleBrand = "brand"
)

// Role represents a validated user system role Value Object.
type Role struct {
	value string
}

// NewRole validates and creates a Role Value Object.
func NewRole(v string) (Role, error) {
	switch v {
	case RoleAdmin, RoleUser, RoleBrand:
		return Role{value: v}, nil
	default:
		return Role{}, derror.ErrInvalidRole
	}
}

// Value returns the raw string value of the Role.
func (r Role) Value() string {
	return r.value
}
