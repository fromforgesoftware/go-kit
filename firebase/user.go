package firebase

import (
	"context"
)

// User represents a firebase user
type User interface {
	ID() string
	Email() string
	Password() string
	FirstName() string
	LastName() string
	EmailVerified() bool
}

// UserManagement defines user management operations
type UserManagement interface {
	Create(ctx context.Context, user User) (User, error)
	Get(ctx context.Context, id string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	Update(ctx context.Context, id string, user User) (User, error)
	Delete(ctx context.Context, id string) error
}

// User implementation
type user struct {
	id            string
	email         string
	password      string
	firstName     string
	lastName      string
	emailVerified bool
}

func (u *user) ID() string          { return u.id }
func (u *user) Email() string       { return u.email }
func (u *user) Password() string    { return u.password }
func (u *user) FirstName() string   { return u.firstName }
func (u *user) LastName() string    { return u.lastName }
func (u *user) EmailVerified() bool { return u.emailVerified }

type UserOption func(*user)

func WithID(id string) UserOption {
	return func(u *user) {
		u.id = id
	}
}

func WithUserLastName(lastName string) UserOption {
	return func(u *user) {
		u.lastName = lastName
	}
}

func WithUserEmailVerified(verified bool) UserOption {
	return func(u *user) {
		u.emailVerified = verified
	}
}

func NewUser(email, password, firstName string, opts ...UserOption) *user {
	u := &user{
		email:     email,
		password:  password,
		firstName: firstName,
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}
