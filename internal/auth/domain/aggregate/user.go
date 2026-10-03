package aggregate

import (
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/event"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

// User status enum matching auth.user_status PostgreSQL enum
const (
	UserStatusActive    = "active"
	UserStatusInactive  = "inactive"
	UserStatusLocked    = "locked"
	UserStatusSuspended = "suspended"
)

// Supported login identity types
const (
	IdentityTypeEmail    = "email"
	IdentityTypePhone    = "phone"
	IdentityTypeGoogle   = "google"
	IdentityTypeFacebook = "facebook"
)

// IsSupportedOAuthProvider checks if the provider is a supported OAuth provider.
func IsSupportedOAuthProvider(provider string) bool {
	return provider == IdentityTypeGoogle || provider == IdentityTypeFacebook
}

// Identity represents a credential/login identity associated with a User.
type Identity struct {
	id             string
	identityType   string
	identifier     string
	credentialData string
	metadata       []byte
	createdAt      time.Time
	updatedAt      time.Time
}

// NewIdentity creates a new Identity value within the User aggregate.
func NewIdentity(
	id string,
	identityType string,
	identifier string,
	credentialData string,
	metadata []byte,
	createdAt time.Time,
	updatedAt time.Time,
) Identity {
	var copiedMetadata []byte
	if metadata != nil {
		copiedMetadata = make([]byte, len(metadata))
		copy(copiedMetadata, metadata)
	}

	return Identity{
		id:             id,
		identityType:   identityType,
		identifier:     identifier,
		credentialData: credentialData,
		metadata:       copiedMetadata,
		createdAt:      createdAt,
		updatedAt:      updatedAt,
	}
}

// Identity Getters
func (i Identity) ID() string             { return i.id }
func (i Identity) IdentityType() string   { return i.identityType }
func (i Identity) Identifier() string     { return i.identifier }
func (i Identity) CredentialData() string { return i.credentialData }
func (i Identity) Metadata() []byte {
	if i.metadata == nil {
		return nil
	}
	res := make([]byte, len(i.metadata))
	copy(res, i.metadata)
	return res
}
func (i Identity) CreatedAt() time.Time { return i.createdAt }
func (i Identity) UpdatedAt() time.Time { return i.updatedAt }

// User represents the user aggregate root in the authentication context.
type User struct {
	id           string
	fullName     string
	role         vo.Role
	status       string
	identity     Identity
	createdAt    time.Time
	updatedAt    time.Time
	domainEvents []event.DomainEvent
}

// NewUser creates a User domain aggregate instance (used by Repositories for persistence mapping).
func NewUser(
	id string,
	fullName string,
	role vo.Role,
	status string,
	identity Identity,
	createdAt time.Time,
	updatedAt time.Time,
) *User {
	if status == "" {
		status = UserStatusActive
	}

	return &User{
		id:        id,
		fullName:  fullName,
		role:      role,
		status:    status,
		identity:  identity,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

// RegisterNewUser is a factory method to register a new user aggregate with full profile metadata, recording the UserRegistered event.
func RegisterNewUser(
	id string,
	fullName string,
	gender string,
	dateOfBirth string,
	identity Identity,
	avatarURL string,
) *User {
	role, _ := vo.NewRole(vo.RoleUser)

	now := time.Now()
	user := &User{
		id:        id,
		fullName:  fullName,
		role:      role,
		status:    UserStatusActive,
		identity:  identity,
		createdAt: now,
		updatedAt: now,
	}

	user.AddDomainEvent(event.UserRegisteredEvent{
		UserID:       id,
		IdentityType: identity.IdentityType(),
		Identifier:   identity.Identifier(),
		FullName:     fullName,
		Gender:       gender,
		DateOfBirth:  dateOfBirth,
		AvatarURL:    avatarURL,
		RegisteredAt: now,
	})

	return user
}

// RegisterUser is a factory method to register a new user aggregate with default "user" role, recording the UserRegistered event.
func RegisterUser(
	id string,
	fullName string,
	identity Identity,
	avatarURL string,
) *User {
	return RegisterNewUser(id, fullName, "", "", identity, avatarURL)
}

// --- Getters ---
func (u *User) ID() string           { return u.id }
func (u *User) FullName() string     { return u.fullName }
func (u *User) Role() string         { return u.role.Value() }
func (u *User) RoleVO() vo.Role      { return u.role }
func (u *User) Status() string       { return u.status }
func (u *User) CreatedAt() time.Time { return u.createdAt }
func (u *User) UpdatedAt() time.Time { return u.updatedAt }

func (u *User) Identity() Identity {
	return u.identity
}

func (u *User) DomainEvents() []event.DomainEvent {
	if u.domainEvents == nil {
		return nil
	}
	res := make([]event.DomainEvent, len(u.domainEvents))
	copy(res, u.domainEvents)
	return res
}

// PrimaryEmail retrieves the email identifier if the identity is of type email.
func (u *User) PrimaryEmail() string {
	if u.identity.IdentityType() == IdentityTypeEmail {
		return u.identity.Identifier()
	}
	return ""
}

// --- Business Actions & State Mutators ---

// SetIdentity updates the single identity of the user.
func (u *User) SetIdentity(identity Identity) {
	u.identity = identity
	u.updatedAt = time.Now()
}

// FindIdentity finds the identity if it matches the type and identifier.
func (u *User) FindIdentity(identityType, identifier string) (Identity, bool) {
	if u.identity.IdentityType() == identityType && u.identity.Identifier() == identifier {
		return u.identity, true
	}
	return Identity{}, false
}

// FindIdentityByType finds the identity if it matches the specified type.
func (u *User) FindIdentityByType(identityType string) (Identity, bool) {
	if u.identity.IdentityType() == identityType {
		return u.identity, true
	}
	return Identity{}, false
}

// Activate transitions the user state to active.
func (u *User) Activate() {
	u.status = UserStatusActive
	u.updatedAt = time.Now()
}

// Lock transitions the user state to locked.
func (u *User) Lock() {
	u.status = UserStatusLocked
	u.updatedAt = time.Now()
}

// Suspend transitions the user state to suspended.
func (u *User) Suspend() {
	u.status = UserStatusSuspended
	u.updatedAt = time.Now()
}

// ChangeRole updates the user's role.
func (u *User) ChangeRole(newRole vo.Role) {
	u.role = newRole
	u.updatedAt = time.Now()
}

// UpdateFullName updates the user's full name.
func (u *User) UpdateFullName(name string) {
	u.fullName = name
	u.updatedAt = time.Now()
}

// UpdatePassword updates the password credential of the user's identity.
func (u *User) UpdatePassword(newHashedPassword string) {
	now := time.Now()
	u.identity.credentialData = newHashedPassword
	u.identity.updatedAt = now
	u.updatedAt = now
}

// AddDomainEvent appends a new domain event to the aggregate.
func (u *User) AddDomainEvent(ev event.DomainEvent) {
	u.domainEvents = append(u.domainEvents, ev)
}

// ClearDomainEvents purges all domain events.
func (u *User) ClearDomainEvents() {
	u.domainEvents = nil
}
