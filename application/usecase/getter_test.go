package usecase_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/application/repository/repositorytest"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/resource/resourcetest"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/searchtest"
	"github.com/stretchr/testify/assert"
)

func TestNewGetter(t *testing.T) {
	ctx := context.Background()
	inOpts := searchtest.AnyOpts()
	inType := resource.Type(resourcetest.NewStub().Type())

	var res *resourcetest.ResourceStub
	var err error

	t.Run("returning error", func(t *testing.T) {
		res = nil
		err = assert.AnError

		getterRepo := repositorytest.NewGetterStub(
			res, err, repositorytest.WithStubInterceptor(
				func(gotCtx context.Context, gotOpts ...search.Option) {
					assert.Equal(t, gotCtx, ctx)
					searchtest.OptsEqual(t, gotOpts, inOpts)
				},
			),
		)
		getterUcase := usecase.NewGetter[*resourcetest.ResourceStub](getterRepo, inType)

		gotResource, gotErr := getterUcase.Get(ctx, inOpts...)
		assert.Nil(t, gotResource)
		assert.ErrorIs(t, gotErr, err)
	})
	t.Run("repo returns nil, return not found", func(t *testing.T) {
		res = nil
		err = nil

		getterRepo := repositorytest.NewGetterStub(
			res, err, repositorytest.WithStubInterceptor(
				func(gotCtx context.Context, gotOpts ...search.Option) {
					assert.Equal(t, gotCtx, ctx)
					searchtest.OptsEqual(t, gotOpts, inOpts)
				},
			),
		)
		getterUcase := usecase.NewGetter[*resourcetest.ResourceStub](getterRepo, inType)

		gotResource, gotErr := getterUcase.Get(ctx, inOpts...)
		assert.Nil(t, gotResource)
		assert.ErrorIs(t, gotErr, errors.NotFound(
			inType.String(),
			search.FieldNameSearch+"."+search.FieldNameOptions,
		))
	})
	t.Run("repo returns resource and no error", func(t *testing.T) {
		res = resourcetest.NewStub()
		err = nil

		getterRepo := repositorytest.NewGetterStub(
			res, err, repositorytest.WithStubInterceptor(
				func(gotCtx context.Context, gotOpts ...search.Option) {
					assert.Equal(t, gotCtx, ctx)
					searchtest.OptsEqual(t, gotOpts, inOpts)
				},
			),
		)
		getterUcase := usecase.NewGetter[*resourcetest.ResourceStub](getterRepo, inType)

		gotResource, gotErr := getterUcase.Get(ctx, inOpts...)
		assert.Equal(t, gotResource, res)
		assert.NoError(t, gotErr)
	})
}
