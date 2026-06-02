package usecase_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/application/repository/repositorytest"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/resource/resourcetest"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/searchtest"
	"github.com/stretchr/testify/assert"
)

func TestListerList(t *testing.T) {
	ctx := context.Background()
	inOpts := searchtest.AnyOpts()
	defaultOpts := searchtest.AnyOpts()

	emptyRes := resource.NewEmptyListResponse[*resourcetest.ResourceStub]()
	nonEmptyRes := resource.NewListResponse(
		[]*resourcetest.ResourceStub{resourcetest.NewStub(), resourcetest.NewStub()},
		10,
	)

	tests := []struct {
		name      string
		inRepoRes resource.ListResponse[*resourcetest.ResourceStub]
		inRepoErr error
		wantColl  resource.ListResponse[*resourcetest.ResourceStub]
		wantErr   error
	}{
		{
			name:      "repo returning error",
			inRepoRes: nil,
			inRepoErr: assert.AnError,
			wantColl:  emptyRes,
			wantErr:   assert.AnError,
		},
		{
			name:      "repo returning nil res",
			inRepoRes: nil,
			inRepoErr: nil,
			wantColl:  emptyRes,
			wantErr:   nil,
		},
		{
			name:      "repo returning empty list",
			inRepoRes: emptyRes,
			inRepoErr: nil,
			wantColl:  emptyRes,
			wantErr:   nil,
		},
		{
			name:      "repo returning a non-empty list",
			inRepoRes: nonEmptyRes,
			inRepoErr: nil,
			wantColl:  nonEmptyRes,
			wantErr:   nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			listerRepo := repositorytest.NewListerStub(
				test.inRepoRes, test.inRepoErr, repositorytest.WithStubInterceptor(
					func(gotCtx context.Context, gotOpts ...search.Option) {
						assert.Equal(t, gotCtx, ctx)
						searchtest.OptsEqual(t, gotOpts, append(defaultOpts, inOpts...))
					},
				),
			)
			listerUsecase := usecase.NewLister(listerRepo, defaultOpts...)

			gotRes, gotErr := listerUsecase.List(ctx, inOpts...)
			assert.NotNil(t, gotRes)
			assert.Equal(t, test.wantColl.TotalCount(), gotRes.TotalCount())
			assert.Len(t, gotRes.Results(), len(test.wantColl.Results()))
			for _, elem := range test.wantColl.Results() {
				found := false
				for _, gotResElem := range gotRes.Results() {
					if gotResElem == elem {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("elem %+v not found", elem)
				}
			}
			assert.ErrorIs(t, gotErr, test.wantErr)
		})
	}
}
