// Package repositorytest Repository test utils
package repositorytest

import (
	"context"

	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
)

type StubOption func(c *stubConfig)

func WithStubInterceptor(f func(ctx context.Context, opts ...search.Option)) StubOption {
	return func(c *stubConfig) {
		c.paramsInterceptor = f
	}
}

type stubConfig struct {
	paramsInterceptor func(ctx context.Context, opts ...search.Option)
}

type baseStub struct {
	err    error
	config *stubConfig
}

type GetterStub[R resource.Resource] struct {
	res R
	baseStub
}

func (gs *GetterStub[R]) Get(ctx context.Context, opts ...search.Option) (R, error) {
	if gs.config.paramsInterceptor != nil {
		gs.config.paramsInterceptor(ctx, opts...)
	}
	return gs.res, gs.err
}

func NewGetterStub[R resource.Resource](res R, err error, opts ...StubOption) *GetterStub[R] {
	c := new(stubConfig)
	for _, opt := range opts {
		opt(c)
	}
	return &GetterStub[R]{
		baseStub: baseStub{
			config: c, err: err,
		},
		res: res,
	}
}

type ListerStub[R resource.Resource] struct {
	res resource.ListResponse[R]
	baseStub
}

func (ls *ListerStub[R]) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[R], error) {
	if ls.config.paramsInterceptor != nil {
		ls.config.paramsInterceptor(ctx, opts...)
	}
	return ls.res, ls.err
}

func NewListerStub[R resource.Resource](res resource.ListResponse[R], err error, opts ...StubOption) *ListerStub[R] {
	c := new(stubConfig)
	for _, opt := range opts {
		opt(c)
	}
	return &ListerStub[R]{
		baseStub: baseStub{
			config: c, err: err,
		},
		res: res,
	}
}
