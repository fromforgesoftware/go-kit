package usecase

import (
	"context"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
)

type Lister[R resource.Resource] interface {
	List(ctx context.Context, opts ...search.Option) (resource.ListResponse[R], error)
}

type lister[R resource.Resource] struct {
	repo        repository.Lister[R]
	defaultOpts []search.Option
}

func NewLister[R resource.Resource](repo repository.Lister[R], defaultOpts ...search.Option) *lister[R] {
	return &lister[R]{
		repo:        repo,
		defaultOpts: defaultOpts,
	}
}

func (c *lister[R]) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[R], error) {
	opts = append(c.defaultOpts, opts...)
	res, err := c.repo.List(ctx, opts...)
	if err != nil || res == nil {
		return resource.NewListResponse([]R{}, 0), err
	}

	return res, nil
}
