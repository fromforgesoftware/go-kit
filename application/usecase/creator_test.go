package usecase_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/application/repository/repositorytest"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/resource/resourcetest"
	"github.com/stretchr/testify/assert"
)

func TestCreate(t *testing.T) {
	repo := repositorytest.NewCreator[resource.Resource](t)

	t.Run("if validation func fails, return error", func(t *testing.T) {
		uc := usecase.NewCreator(repo, func(context.Context, resource.Resource) error { return assert.AnError })
		got, err := uc.Create(context.TODO(), resourcetest.NewStub())
		assert.Nil(t, got)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("if repository returns an error, return error", func(t *testing.T) {
		uc := usecase.NewCreator(repo, func(context.Context, resource.Resource) error { return nil })
		in := resourcetest.NewStub()
		repo.EXPECT().Create(context.TODO(), in).Return(resourcetest.NewStub(), assert.AnError)
		got, err := uc.Create(context.TODO(), in)
		assert.Nil(t, got)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("if repository returns a resource, return the resource", func(t *testing.T) {
		uc := usecase.NewCreator(repo, func(context.Context, resource.Resource) error { return nil })
		in := resourcetest.NewStub()
		want := resourcetest.NewStub()
		repo.EXPECT().Create(context.TODO(), in).Return(want, nil)
		got, err := uc.Create(context.TODO(), in)
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})
}

func TestCreateBatch(t *testing.T) {
	t.Parallel()

	resourceStub := []resource.Resource{resourcetest.NewStub()}
	expected := []resource.Resource{resourcetest.NewStub()}

	tests := []struct {
		name                string
		in                  []resource.Resource
		expectedValidatorFn error
		expected            []resource.Resource
		expectedErr         error
		mocks               func(*repositorytest.CreatorBatch[resource.Resource])
	}{
		{
			name:        "if data is empty, return error",
			in:          []resource.Resource{},
			expectedErr: apierrors.New(apierrors.CodeMissingField),
			mocks:       func(*repositorytest.CreatorBatch[resource.Resource]) {},
		},
		{
			name:        "if resource in data is null, return error",
			in:          []resource.Resource{(*resource.RestDTO)(nil)},
			expectedErr: apierrors.New(apierrors.CodeInvalidArgument, apierrors.WithMessage("request cannot be zero value")),
			mocks:       func(*repositorytest.CreatorBatch[resource.Resource]) {},
		},
		{
			name:                "if validation func fails, return error",
			in:                  resourceStub,
			expectedValidatorFn: assert.AnError,
			expectedErr:         assert.AnError,
			mocks:               func(*repositorytest.CreatorBatch[resource.Resource]) {},
		},
		{
			name:        "if repository returns an error, return error",
			in:          resourceStub,
			expectedErr: assert.AnError,
			mocks: func(repo *repositorytest.CreatorBatch[resource.Resource]) {
				repo.EXPECT().CreateBatch(context.TODO(), resourceStub).Return(nil, assert.AnError)
			},
		},
		{
			name:     "if repository returns a resource, return the resource",
			in:       resourceStub,
			expected: expected,
			mocks: func(repo *repositorytest.CreatorBatch[resource.Resource]) {
				repo.EXPECT().CreateBatch(context.TODO(), resourceStub).Return(expected, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repositorytest.NewCreatorBatch[resource.Resource](t)
			tt.mocks(repo)

			uc := usecase.NewCreatorBatch(repo, func(context.Context, []resource.Resource) error { return tt.expectedValidatorFn })
			got, err := uc.CreateBatch(context.TODO(), tt.in)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr, "Error should be: %v, got: %v", tt.expectedErr, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
