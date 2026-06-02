package usecase

import (
	"context"
	"fmt"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/persistence"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
)

type (
	Patcher[R any] interface {
		Patch(ctx context.Context, opts ...repository.PatchOption) (R, error)
	}
)

type singlePatcher[R resource.Resource] struct {
	repo             repository.Patcher[R]
	resType          resource.Type
	validationFunc   func(context.Context, repository.PatchQuery) error
	defaultPatchOpts []repository.PatchOption
	tx               persistence.Transactioner
	monitor          monitoring.Monitor
}

func NewSinglePatcher[R resource.Resource](
	repo repository.Patcher[R],
	resType resource.Type,
	validationFunc func(context.Context, repository.PatchQuery) error,
	tx persistence.Transactioner,
	monitor monitoring.Monitor,
	defaultPatchOpts ...repository.PatchOption,
) *singlePatcher[R] {
	return &singlePatcher[R]{
		repo:             repo,
		resType:          resType,
		validationFunc:   validationFunc,
		defaultPatchOpts: defaultPatchOpts,
		monitor:          monitor,
		tx:               tx,
	}
}

func (p *singlePatcher[R]) Patch(ctx context.Context, opts ...repository.PatchOption) (R, error) {
	var zero R
	var err error

	opts = append(opts, p.defaultPatchOpts...)

	patchQuery := repository.NewPatchQuery(opts...)

	err = p.validationFunc(ctx, patchQuery)
	if err != nil {
		return zero, err
	}
	var res []R
	err = p.tx.Exec(ctx, func(txCtx context.Context) error {
		res, err = p.repo.Patch(txCtx, repository.WithPatchQuery(patchQuery))
		if err != nil {
			return err
		}

		if len(res) == 0 {
			s := search.New(patchQuery.SearchOpts()...)
			id := query.GetFilterValOrDefault(string("id"), s.Query().Filters(), "")
			return errors.NotFound(p.resType.String(), resource.NewIdentifier(id, p.resType))
		}

		if len(res) > 1 {
			p.monitor.Logger().WithKeysAndValues("patches", len(res)).ErrorContext(ctx, "unexpected number of patches")
			return errors.Conflict(fmt.Sprintf("unexpected number of patches: %d", len(res)))
		}
		return nil
	})
	if err != nil {
		return zero, err
	}

	return res[0], nil
}
