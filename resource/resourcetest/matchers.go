package resourcetest

import (
	"slices"
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

func MatchByIDFunc(want resource.Resource) func(resource.Resource) bool {
	return func(got resource.Resource) bool {
		return want.ID() == got.ID()
	}
}

type matchConfig struct {
	resID     string
	isCreate  bool
	isUpdate  bool
	isJSONDTO bool
}

type MatchOption func(c *matchConfig)

func MatchCreate() MatchOption {
	return func(c *matchConfig) {
		c.isCreate = true
	}
}

func MatchUpdate() MatchOption {
	return func(c *matchConfig) {
		c.isUpdate = true
	}
}

func MatchJSONDTO() MatchOption {
	return func(c *matchConfig) {
		c.isJSONDTO = true
	}
}

func MatchIdentifiers(want []resource.Identifier) func([]resource.Identifier) bool {
	return func(got []resource.Identifier) bool {
		return slices.EqualFunc(want, got, func(want, got resource.Identifier) bool { return MatchIdentifier(want)(got) })
	}
}

func MatchIdentifier(want resource.Identifier) func(resource.Identifier) bool {
	return func(got resource.Identifier) bool {
		if want == nil {
			return got == nil
		}
		return want.ID() == got.ID() && want.Type() == got.Type()
	}
}

func MatchID(id string) MatchOption {
	return func(c *matchConfig) {
		c.resID = id
	}
}

func Match(want resource.Resource, opts ...MatchOption) func(resource.Resource) bool {
	return func(got resource.Resource) bool {
		matchConfig := &matchConfig{}
		for _, opt := range opts {
			opt(matchConfig)
		}

		if matchConfig.resID != "" {
			return matchConfig.resID == got.ID()
		}
		if matchConfig.isCreate {
			return want.Type() == got.Type()
		}
		if matchConfig.isUpdate {
			return want.ID() == got.ID() && want.Type() == got.Type()
		}
		if matchConfig.isJSONDTO {
			return matchTimestampsWithMillis(want)(got)
		}
		return matchFullResource(want)(got)
	}
}

func matchTimestampsWithMillis(want resource.Resource) func(resource.Resource) bool {
	return func(got resource.Resource) bool {
		return want.ID() == got.ID() &&
			want.Type() == got.Type() &&
			want.CreatedAt().Truncate(time.Millisecond).Equal(got.CreatedAt().Truncate(time.Millisecond)) &&
			want.UpdatedAt().Truncate(time.Millisecond).Equal(got.UpdatedAt().Truncate(time.Millisecond)) &&
			(want.DeletedAt() == nil && got.DeletedAt() == nil ||
				want.DeletedAt().Truncate(time.Millisecond).Equal(got.DeletedAt().Truncate(time.Millisecond)))
	}
}

func matchFullResource(want resource.Resource) func(resource.Resource) bool {
	return func(got resource.Resource) bool {
		return want.ID() == got.ID() &&
			want.Type() == got.Type() &&
			want.CreatedAt().Equal(got.CreatedAt()) &&
			want.UpdatedAt().Equal(got.UpdatedAt()) &&
			(want.DeletedAt() == nil && got.DeletedAt() == nil ||
				want.DeletedAt().Equal(*got.DeletedAt()))
	}
}
