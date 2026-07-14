package aggregate

import (
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/event"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

// User represents the user aggregate in the authentication context.
type User struct {
	id           vo.UserID
	email        vo.Email
	googleID     string
	facebookID   string
	fullName     string
	role         vo.Role
	createdAt    time.Time
	updatedAt    time.Time
	domainEvents []event.DomainEvent
}

// NewUser creates a new User domain instance (typically used by Repositories for database mapping).
func NewUser(
	id vo.UserID,
	email vo.Email,
	googleID string,
	facebookID string,
	fullName string,
	role vo.Role,
	createdAt time.Time,
	updatedAt time.Time,
) *User {
	return &User{
		id:         id,
		email:      email,
		googleID:   googleID,
		facebookID: facebookID,
		fullName:   fullName,
		role:       role,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}

// RegisterUser is a factory method to register a new user in the domain, triggering the UserRegistered event.
func RegisterUser(
	idVal string,
	emailVal string,
	fullName string,
	roleVal string,
) (*User, error) {
	id, err := vo.NewUserID(idVal)
	if err != nil {
		return nil, err
	}

	email, err := vo.NewEmail(emailVal)
	if err != nil {
		return nil, err
	}

	role, err := vo.NewRole(roleVal)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &User{
		id:        id,
		email:     email,
		fullName:  fullName,
		role:      role,
		createdAt: now,
		updatedAt: now,
	}

	user.AddDomainEvent(event.UserRegisteredEvent{
		UserID:       id.Value(),
		Email:        email.Value(),
		FullName:     fullName,
		RegisteredAt: now,
	})

	return user, nil
}

// --- Getters ---
func (u *User) ID() string                        { return u.id.Value() }
func (u *User) Email() string                     { return u.email.Value() }
func (u *User) GoogleID() string                  { return u.googleID }
func (u *User) FacebookID() string                { return u.facebookID }
func (u *User) FullName() string                  { return u.fullName }
func (u *User) Role() string                      { return u.role.Value() }
func (u *User) CreatedAt() time.Time              { return u.createdAt }
func (u *User) UpdatedAt() time.Time              { return u.updatedAt }
func (u *User) DomainEvents() []event.DomainEvent { return u.domainEvents }

// --- Business Actions & Mutators ---

// LinkGoogle links a Google social ID to the user.
func (u *User) LinkGoogle(googleID string) {
	u.googleID = googleID
	u.updatedAt = time.Now()
}

// LinkFacebook links a Facebook social ID to the user.
func (u *User) LinkFacebook(facebookID string) {
	u.facebookID = facebookID
	u.updatedAt = time.Now()
}

// AddDomainEvent appends a new domain event.
func (u *User) AddDomainEvent(ev event.DomainEvent) {
	u.domainEvents = append(u.domainEvents, ev)
}

// ClearDomainEvents purges all domain events.
func (u *User) ClearDomainEvents() {
	u.domainEvents = nil
}
