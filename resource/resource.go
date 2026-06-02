package resource

import (
	"strconv"
	"time"
)

type (
	Resource interface {
		Identifier
		Timestamps
	}

	Identifier interface {
		ID() string
		LID() string
		Type() Type
	}

	Timestamps interface {
		CreatedAt() time.Time
		UpdatedAt() time.Time
		DeletedAt() *time.Time
	}

	Type string
)

func (t Type) String() string {
	return string(t)
}

type (
	resource struct {
		id        string
		lid       string
		createdAt time.Time
		updatedAt time.Time
		deletedAt *time.Time
		kind      Type
	}

	resourceOption func(*resource)
)

func New(opts ...resourceOption) Resource {
	res := &resource{}
	for _, opt := range opts {
		opt(res)
	}
	return res
}

func Update(res Resource, opts ...resourceOption) *resource {
	update := &resource{
		id:        res.ID(),
		lid:       res.LID(),
		createdAt: res.CreatedAt(),
		updatedAt: res.UpdatedAt(),
		deletedAt: res.DeletedAt(),
		kind:      res.Type(),
	}
	for _, opt := range opts {
		opt(update)
	}
	return update
}

func NewIdentifier(id string, kind Type) Identifier {
	return &resource{
		id:   id,
		kind: kind,
	}
}

func WithID(id string) resourceOption {
	return func(r *resource) {
		r.id = id
	}
}

func WithLID(lid string) resourceOption {
	return func(r *resource) {
		r.lid = lid
	}
}

func WithType(kind Type) resourceOption {
	return func(r *resource) {
		r.kind = kind
	}
}

func WithCreatedAt(createdAt time.Time) resourceOption {
	return func(r *resource) {
		r.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) resourceOption {
	return func(r *resource) {
		r.updatedAt = updatedAt
	}
}

func WithDeletedAt(deletedAt *time.Time) resourceOption {
	return func(r *resource) {
		r.deletedAt = deletedAt
	}
}

func (r *resource) ID() string {
	return r.id
}

func (r *resource) LID() string {
	return r.lid
}

func (r *resource) Type() Type {
	return r.kind
}

func (r *resource) CreatedAt() time.Time {
	return r.createdAt
}

func (r *resource) UpdatedAt() time.Time {
	return r.updatedAt
}

func (r *resource) DeletedAt() *time.Time {
	return r.deletedAt
}

// Helpers

func IDToUint64(id string) uint64 {
	i, _ := strconv.ParseUint(id, 10, 64)
	return i
}

func IDToInt(id string) int {
	i, _ := strconv.Atoi(id)
	return i
}
