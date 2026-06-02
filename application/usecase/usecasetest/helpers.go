package usecasetest

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/application/usecase"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/resource/resourcetest"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/stretchr/testify/assert"
)

type UsecaseListerTestDef[R resource.Resource] struct {
	Ctx       context.Context
	SearchOpt []search.Option
	Want      resource.ListResponse[R]
	WantErr   error
}

func AssertListResults[R, I resource.Resource](
	t *testing.T,
	uc usecase.Lister[R],
	testDef UsecaseListerTestDef[R],
	assertResultFn func(t *testing.T, expect, actual R, opts ...resourcetest.AssertOption),
	assertIncludeFn func(t *testing.T, expect, actual I, opts ...resourcetest.AssertOption),
) {
	t.Helper()

	got, err := uc.List(testDef.Ctx, testDef.SearchOpt...)
	assert.ErrorIs(t, err, testDef.WantErr)
	if err != nil {
		return
	}

	assert.Equal(t, len(testDef.Want.Results()), len(got.Results()))
	for i, w := range testDef.Want.Results() {
		assertResultFn(t, w, got.Results()[i])
	}
}
