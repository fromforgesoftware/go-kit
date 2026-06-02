package usecase

import (
	"context"
	"reflect"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
)

type Getter[R resource.Resource] interface {
	Get(ctx context.Context, opts ...search.Option) (R, error)
}

type getter[R resource.Resource] struct {
	repo    repository.Getter[R]
	resType resource.Type
}

func (c *getter[R]) Get(ctx context.Context, opts ...search.Option) (R, error) {
	res, err := c.repo.Get(ctx, opts...)
	if err != nil {
		return res, err
	}
	var zero R
	if reflect.DeepEqual(res, zero) {
		return zero, errors.NotFound(c.resType.String(), search.FieldNameSearch+"."+search.FieldNameOptions)
	}

	return res, nil
}

func NewGetter[R resource.Resource](repo repository.Getter[R], resType resource.Type) *getter[R] {
	return &getter[R]{
		repo:    repo,
		resType: resType,
	}
}
