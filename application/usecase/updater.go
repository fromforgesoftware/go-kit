package usecase

import (
	"context"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/resource"
)

type Updater[R resource.Resource] interface {
	Update(ctx context.Context, req R) (R, error)
}

type updater[R resource.Resource] struct {
	repo           repository.Updater[R]
	validationFunc func(context.Context, R) error
}

func NewUpdater[R resource.Resource](
	repo repository.Updater[R],
	validationFunc func(context.Context, R) error,
) *updater[R] {
	return &updater[R]{
		repo:           repo,
		validationFunc: validationFunc,
	}
}

func (u *updater[R]) Update(ctx context.Context, req R) (R, error) {
	var zero R

	err := u.validationFunc(ctx, req)
	if err != nil {
		return zero, err
	}

	return u.repo.Update(ctx, req)
}
