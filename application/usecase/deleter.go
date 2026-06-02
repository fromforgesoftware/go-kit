package usecase

import (
	"context"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/search"
)

type Deleter interface {
	Delete(context.Context, repository.DeleteType, ...search.Option) error
}

type deleter struct {
	repo repository.Deleter
}

func NewDeleter(repo repository.Deleter) *deleter {
	return &deleter{
		repo: repo,
	}
}

func (d *deleter) Delete(ctx context.Context, deleteType repository.DeleteType, opts ...search.Option) error {
	return d.repo.Delete(ctx, deleteType, opts...)
}
