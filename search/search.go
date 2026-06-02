// Package search composes the Query DSL (filter / sort / fieldsets /
// includes / pagination) into the search.Option callbacks every kit
// repository / list endpoint consumes.
package search

import (
	"github.com/fromforgesoftware/go-kit/search/query"
)

const (
	FieldNameSearch  string = "search"
	FieldNameOptions string = "options"
)

type Search interface {
	Query() query.Query
	Equal(another Search) bool
}

type search struct {
	query query.Query
}

func (s *search) Query() query.Query {
	return s.query
}

func (s *search) Equal(another Search) bool {
	if (s == nil && another != nil) ||
		s != nil && another == nil {
		return false
	}
	if s == nil && another == nil {
		return true
	}

	return s.query.Equal(another.Query())
}

type Option func(s *search)

func (o Option) Equal(another Option) bool {
	return New(o).Equal(New(another))
}

func WithQuery(q query.Query) Option {
	return func(s *search) {
		s.query.Merge(q)
	}
}

func WithQueryOpts(opts ...query.Option) Option {
	return func(s *search) {
		WithQuery(query.New(opts...))(s)
	}
}

func New(opts ...Option) Search {
	s := &search{
		query: query.New(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}
