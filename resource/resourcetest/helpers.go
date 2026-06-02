package resourcetest

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/fromforgesoftware/go-kit/proto/tb/v1"
	"github.com/fromforgesoftware/go-kit/resource"
)

type AssertConfig struct {
	gotHasID        bool
	checkID         bool
	checkCreatedAt  bool
	checkUpdatedAt  bool
	DBOp            bool
	CheckTimesMilli bool
}

type AssertOption func(*AssertConfig)

func defaultAssertOpts() []AssertOption {
	return []AssertOption{
		assertCheckFlags(true, true, true, true),
	}
}

func AssertTimestampMilli() AssertOption {
	return func(ac *AssertConfig) {
		ac.CheckTimesMilli = true
	}
}

func AssertDBGet() AssertOption {
	return assertDBOpAssertion(true, true, true, true)
}

func AssertDBCreate() AssertOption {
	return assertDBOpAssertion(true, false, false, false)
}

func AssertDBUpdate() AssertOption {
	return assertDBOpAssertion(true, true, true, false)
}

func AssertNotEmptyID() AssertOption {
	return func(c *AssertConfig) {
		c.gotHasID = false
	}
}

func AssertSkipID() AssertOption {
	return func(c *AssertConfig) {
		c.checkID = false
		c.gotHasID = false
	}
}

func assertDBOpAssertion(gotHasID, checkID, checkCreatedAt, checkUpdatedAt bool) AssertOption {
	return func(c *AssertConfig) {
		c.DBOp = true
		assertCheckFlags(gotHasID, checkID, checkCreatedAt, checkUpdatedAt)(c)
	}
}

func assertCheckFlags(gotHasID, checkID, checkCreatedAt, checkUpdatedAt bool) AssertOption {
	return func(c *AssertConfig) {
		c.gotHasID = gotHasID
		c.checkID = checkID
		c.checkCreatedAt = checkCreatedAt
		c.checkUpdatedAt = checkUpdatedAt
	}
}

func AssertEqualIdentifiers(t *testing.T, expected, got []resource.Identifier) {
	t.Helper()

	assert.Len(t, got, len(expected))
	for i, exp := range expected {
		AssertEqualIdentifier(t, exp, got[i])
	}
}

func AssertEqualIdentifier(t *testing.T, expected, got resource.Identifier) {
	t.Helper()

	if expected == nil {
		assert.Nil(t, got)
		return
	}

	assert.Equal(t, expected.ID(), got.ID())
	assert.Equal(t, expected.Type(), got.Type())
}

func AssertEqual(t *testing.T, expected, got resource.Resource, ops ...AssertOption) {
	t.Helper()

	c := new(AssertConfig)
	for _, opt := range append(defaultAssertOpts(), ops...) {
		opt(c)
	}

	if expected == nil {
		assert.Nil(t, got)
		return
	}

	if c.gotHasID {
		assert.NotEmpty(t, got.ID(), "id should not be empty for resource: %+v", got)
	}
	if c.checkID {
		assert.Equal(t, expected.ID(), got.ID())
	}
	expectCreatedAt, gotCreatedAt := expected.CreatedAt(), got.CreatedAt()
	expectUpdatedAt, gotUpdatedAt := expected.UpdatedAt(), got.UpdatedAt()
	if c.DBOp { // Timestamp type in PostgreSQL has microseconds precision
		expectCreatedAt = expectCreatedAt.Truncate(time.Microsecond)
		gotCreatedAt = gotCreatedAt.Truncate(time.Microsecond)
		expectUpdatedAt = expectUpdatedAt.Truncate(time.Microsecond)
		gotUpdatedAt = gotUpdatedAt.Truncate(time.Microsecond)
	}
	if c.CheckTimesMilli {
		expectCreatedAt = expectCreatedAt.Truncate(time.Millisecond)
		gotCreatedAt = gotCreatedAt.Truncate(time.Millisecond)
		expectUpdatedAt = expectUpdatedAt.Truncate(time.Millisecond)
		gotUpdatedAt = gotUpdatedAt.Truncate(time.Millisecond)
	}
	if c.checkCreatedAt {
		assert.True(t, expectCreatedAt.Equal(gotCreatedAt), "created at [%+v] does not match [%+v]", expectCreatedAt, gotCreatedAt)
	}
	if c.checkUpdatedAt {
		assert.True(t, expectUpdatedAt.Equal(gotUpdatedAt), "updated at [%+v] does not match [%+v]", expectUpdatedAt, gotUpdatedAt)
	}

	assert.Equal(t, expected.Type(), got.Type())
	assert.Equal(t, expected.DeletedAt(), got.DeletedAt())
}

func AssertEqualListResponse[T resource.Resource](t *testing.T, expected, got resource.ListResponse[T], opts ...AssertOption) {
	t.Helper()

	if expected == nil {
		assert.Nil(t, got)
		return
	}

	assert.Len(t, got.Results(), len(expected.Results()))
	assert.Equal(t, expected.TotalCount(), got.TotalCount())

	for i := range expected.Results() {
		AssertEqual(t, expected.Results()[i], got.Results()[i], opts...)
	}
}

func AssertEqualProto(t *testing.T, expected, got *pb.Resource, ops ...AssertOption) {
	t.Helper()

	c := new(AssertConfig)
	for _, opt := range append(defaultAssertOpts(), ops...) {
		opt(c)
	}

	if expected == nil {
		assert.Nil(t, got)
		return
	}

	if c.gotHasID {
		assert.NotEmpty(t, got.Id, "id should not be empty for resource: %+v", got)
	}
	if c.checkID {
		assert.Equal(t, expected.Id, got.Id)
	}
	if c.DBOp || c.CheckTimesMilli { // Timestamp type in PostgreSQL has microseconds precision
		assert.Equal(t, expected.CreatedAt.AsTime().Truncate(time.Millisecond), got.CreatedAt.AsTime().Truncate(time.Millisecond))
		assert.Equal(t, expected.UpdatedAt.AsTime().Truncate(time.Millisecond), got.UpdatedAt.AsTime().Truncate(time.Millisecond))
		assert.Equal(t, expected.DeletedAt.AsTime().Truncate(time.Millisecond), got.DeletedAt.AsTime().Truncate(time.Millisecond))
	}
	if c.checkCreatedAt {
		assert.Equal(t, expected.CreatedAt.AsTime().Truncate(time.Millisecond), got.CreatedAt.AsTime().Truncate(time.Millisecond))
	}
	if c.checkUpdatedAt {
		assert.Equal(t, expected.UpdatedAt.AsTime().Truncate(time.Millisecond), got.UpdatedAt.AsTime().Truncate(time.Millisecond))
	}
	assert.Equal(t, expected.Type, got.Type)
}

func AssertEqualIdentifiersProto(t *testing.T, expected, got []*pb.ResourceIdentifier) {
	t.Helper()

	assert.Len(t, got, len(expected))
	for i := range expected {
		AssertEqualIdentifierProto(t, expected[i], got[i])
	}
}

func AssertEqualIdentifierProto(t *testing.T, expected, got *pb.ResourceIdentifier) {
	t.Helper()

	if expected == nil {
		assert.Nil(t, got)
		return
	}

	assert.Equal(t, expected.Id, got.Id)
	assert.Equal(t, expected.Type, got.Type)
}

func AssertIsResourceOrResourceArray(t *testing.T, got any) {
	t.Helper()

	_, isRes := got.(resource.Resource)
	if !isRes {
		rt := reflect.TypeOf(got)
		//nolint:exhaustive // we only care about slice and array
		switch rt.Kind() {
		case reflect.Slice, reflect.Array:
			s := reflect.ValueOf(got)
			for i := 0; i < s.Len(); i++ {
				_, isRes := s.Index(i).Interface().(resource.Resource)
				assert.True(t, isRes)
			}
		default:
			assert.Fail(t, "got is not a resource or resource array")
		}
	}
}

func AssertNullTime(t *testing.T, want, got *time.Time, resOpts ...AssertOption) {
	t.Helper()

	cfg := &AssertConfig{}
	for _, opt := range append(defaultAssertOpts(), resOpts...) {
		opt(cfg)
	}

	if want == nil {
		assert.Nil(t, got)
		return
	}

	if cfg.DBOp {
		*got = got.Truncate(time.Microsecond)
		*want = want.Truncate(time.Microsecond)
	}

	assert.True(t, want.Equal(*got))
}

func AssertEqualIdentifiersOrAssertFunc[T any](
	t *testing.T,
	expected, got []resource.Identifier,
	fn func(t *testing.T, expected, got T, opts ...AssertOption),
	ops ...AssertOption,
) {
	t.Helper()

	assert.Len(t, got, len(expected))
	for i, exp := range expected {
		AssertEqualIdentifierOrAssertFunc(t, exp, got[i], fn, ops...)
	}
}

func AssertEqualIdentifierOrAssertFunc[T any](
	t *testing.T,
	expected,
	got resource.Identifier,
	fn func(t *testing.T, expected, got T, opts ...AssertOption),
	ops ...AssertOption,
) {
	t.Helper()

	if expected == nil {
		assert.Nil(t, got)
		return
	}

	val, ok := expected.(T)
	if !ok {
		AssertEqualIdentifier(t, expected, got)
		return
	}

	gotVal, ok := got.(T)
	require.True(t, ok)

	fn(t, val, gotVal, ops...)
}
