package usecase

import (
	"context"
	"reflect"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/resource"
)

type Creator[R resource.Resource] interface {
	Create(ctx context.Context, r R) (R, error)
}

type creator[R resource.Resource] struct {
	repo           repository.Creator[R]
	validationFunc func(context.Context, R) error
}

func NewCreator[R resource.Resource](repo repository.Creator[R], validationFunc func(context.Context, R) error) Creator[R] {
	return &creator[R]{repo: repo, validationFunc: validationFunc}
}

func (c *creator[R]) Create(ctx context.Context, r R) (R, error) {
	var zero R
	if err := c.validationFunc(ctx, r); err != nil {
		return zero, err
	}

	result, err := c.repo.Create(ctx, r)
	if err != nil {
		return zero, err
	}

	return result, nil
}

type CreatorBatch[R resource.Resource] interface {
	CreateBatch(ctx context.Context, r []R) ([]R, error)
}

type creatorBatch[R resource.Resource] struct {
	repo           repository.CreatorBatch[R]
	validationFunc func(context.Context, []R) error
}

func NewCreatorBatch[R resource.Resource](repo repository.CreatorBatch[R], validationFunc func(context.Context, []R) error) CreatorBatch[R] {
	return &creatorBatch[R]{repo: repo, validationFunc: validationFunc}
}

func (c *creatorBatch[R]) CreateBatch(ctx context.Context, r []R) ([]R, error) {
	var zero []R
	if len(r) == 0 {
		return nil, apierrors.New(apierrors.CodeMissingField)
	}

	for _, d := range r {
		if reflect.ValueOf(d).IsNil() {
			return zero, apierrors.New(apierrors.CodeInvalidArgument, apierrors.WithMessage("request cannot be zero value"))
		}
	}

	if err := c.validationFunc(ctx, r); err != nil {
		return zero, err
	}

	return c.repo.CreateBatch(ctx, r)
}
