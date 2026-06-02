package repository

import (
	"context"

	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
)

type Creator[R resource.Resource] interface {
	Create(context.Context, R) (R, error)
}

type CreatorBatch[R resource.Resource] interface {
	CreateBatch(context.Context, []R) ([]R, error)
}

type Getter[R resource.Resource] interface {
	Get(ctx context.Context, opts ...search.Option) (R, error)
}

type Lister[R resource.Resource] interface {
	List(ctx context.Context, opts ...search.Option) (resource.ListResponse[R], error)
}

type Updater[R resource.Resource] interface {
	Update(context.Context, R) (R, error)
}

type Patcher[R resource.Resource] interface {
	Patch(context.Context, ...PatchOption) ([]R, error)
}

type Deleter interface {
	Delete(ctx context.Context, delType DeleteType, opts ...search.Option) error
}
