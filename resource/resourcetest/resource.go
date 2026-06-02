// Package resourcetest provides test helpers for resource package
package resourcetest

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/google/uuid"

	res "github.com/fromforgesoftware/go-kit/resource"
)

const (
	ResourceTypeStub = res.Type("test")
	Serial4Limit     = 2147483647 - 1
)

type resourceStub struct {
	id        string
	lid       string
	kind      res.Type
	createdAt time.Time
	updatedAt time.Time
	deletedAt *time.Time
}

// ResourceStub is an exported alias for the test resource stub
type ResourceStub = resourceStub

type Option func(*resourceStub)

func defaultOpts() []Option {
	return []Option{
		WithID(uuid.NewString()),
		WithType(ResourceTypeStub),
		WithCreatedAt(time.Now().UTC()),
		WithUpdatedAt(time.Now().UTC()),
	}
}

func New(opts ...Option) *resourceStub {
	res := &resourceStub{}
	for _, opt := range append(defaultOpts(), opts...) {
		opt(res)
	}
	return res
}

// NewStub is an exported alias for New
func NewStub(opts ...Option) *ResourceStub {
	return New(opts...)
}

func Update(rs *resourceStub, opts ...Option) *resourceStub {
	updated := &resourceStub{
		id:        rs.id,
		lid:       rs.lid,
		kind:      rs.kind,
		createdAt: rs.createdAt,
		updatedAt: rs.updatedAt,
		deletedAt: rs.deletedAt,
	}
	for _, opt := range opts {
		opt(updated)
	}

	return updated
}

func WithID(id string) Option {
	return func(rs *resourceStub) {
		rs.id = id
	}
}

func WithLID(lid string) Option {
	return func(rs *resourceStub) {
		rs.lid = lid
	}
}

func WithRandomSerialID() Option {
	return func(rs *resourceStub) {
		rs.id = RandomSerialID()
	}
}

func WithRandomSerialLID() Option {
	return func(rs *resourceStub) {
		rs.lid = RandomSerialID()
	}
}

func WithType(kind res.Type) Option {
	return func(rs *resourceStub) {
		rs.kind = kind
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(rs *resourceStub) {
		rs.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(rs *resourceStub) {
		rs.updatedAt = updatedAt
	}
}

func WithDeletedAt(deletedAt *time.Time) Option {
	return func(rs *resourceStub) {
		rs.deletedAt = deletedAt
	}
}

func WithEmptyResource() Option {
	return func(rs *resourceStub) {
		rs.id = ""
		rs.createdAt = time.Time{}
		rs.updatedAt = time.Time{}
		rs.deletedAt = nil
	}
}

func WithRef(id string, kind res.Type) Option {
	return func(rs *resourceStub) {
		rs.id = id
		rs.createdAt = time.Time{}
		rs.updatedAt = time.Time{}
		rs.deletedAt = nil
		rs.kind = kind
	}
}

func FromResource(r res.Resource) Option {
	return func(rs *resourceStub) {
		rs.id = r.ID()
		rs.kind = res.Type(r.Type())
		rs.createdAt = r.CreatedAt()
		rs.updatedAt = r.UpdatedAt()
		rs.deletedAt = r.DeletedAt()
	}
}

func WithSerialDefaultOpts() Option {
	return func(rs *resourceStub) {
		rs.id = RandomSerialID()
		rs.kind = ResourceTypeStub
		rs.createdAt = time.Now().UTC()
		rs.updatedAt = time.Now().UTC()
	}
}

func (r *resourceStub) ID() string {
	return r.id
}

func (r *resourceStub) LID() string {
	return r.lid
}

func (r *resourceStub) Type() res.Type {
	return r.kind
}

func (r *resourceStub) CreatedAt() time.Time {
	return r.createdAt
}

func (r *resourceStub) UpdatedAt() time.Time {
	return r.updatedAt
}

func (r *resourceStub) DeletedAt() *time.Time {
	return r.deletedAt
}

func RandomSerialID() string {
	//nolint:gosec // not a security issue for tests
	return strconv.Itoa(rand.Intn(Serial4Limit))
}
