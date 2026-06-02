package usecase_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/repository/repositorytest"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/fromforgesoftware/go-kit/search/searchtest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeleterDelete(t *testing.T) {
	t.Parallel()

	inOpts := search.WithQueryOpts(
		query.FilterBy(filter.OpEq, "id", uuid.NewString()),
	)

	tests := []struct {
		name       string
		deleteType repository.DeleteType
		sOpts      search.Option
		wantErr    error
		mocks      func(*repositorytest.Deleter)
	}{
		{
			name:       "if repository returns an error, return error",
			deleteType: repository.DeleteTypeSoft,
			sOpts:      inOpts,
			wantErr:    assert.AnError,
			mocks: func(repo *repositorytest.Deleter) {
				repo.EXPECT().Delete(context.TODO(), repository.DeleteTypeSoft, mock.MatchedBy(searchtest.OptMatcherFunc(inOpts))).Return(assert.AnError)
			},
		},
		{
			name:       "repository deletes successfully",
			deleteType: repository.DeleteTypeSoft,
			sOpts:      inOpts,
			mocks: func(repo *repositorytest.Deleter) {
				repo.EXPECT().Delete(context.TODO(), repository.DeleteTypeSoft, mock.MatchedBy(searchtest.OptMatcherFunc(inOpts))).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repositorytest.NewDeleter(t)
			if tt.mocks != nil {
				tt.mocks(repo)
			}

			uc := usecase.NewDeleter(repo)
			err := uc.Delete(context.TODO(), tt.deleteType, tt.sOpts)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
