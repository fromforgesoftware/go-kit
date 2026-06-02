package repository

import (
	"maps"
	"slices"

	"github.com/fromforgesoftware/go-kit/search"
)

type (
	PatchQuery interface {
		SearchOpts() []search.Option
		PatchFields() map[string]any
		FilterPatchFields(allow ...string) map[string]any
	}

	patchQuery struct {
		patchFields map[string]any
		searchOpts  []search.Option
	}

	PatchOption func(pq *patchQuery)
)

func NewPatchQuery(opts ...PatchOption) *patchQuery {
	pq := &patchQuery{
		patchFields: make(map[string]any),
	}
	for _, opt := range opts {
		opt(pq)
	}
	return pq
}

func (pq *patchQuery) SearchOpts() []search.Option {
	return pq.searchOpts
}

func (pq *patchQuery) PatchFields() map[string]any {
	return pq.patchFields
}

func (pq *patchQuery) PatchFieldsAsOptions() []PatchOption {
	opts := []PatchOption{}
	for fName, fVal := range pq.patchFields {
		opts = append(opts, PatchField(fName, fVal))
	}

	return opts
}

func (pq *patchQuery) PatchFieldExists(fName string) bool {
	_, exists := pq.patchFields[fName]
	return exists
}

func (pq *patchQuery) FilterPatchFields(allow ...string) map[string]any {
	if len(allow) == 0 {
		return pq.patchFields
	}
	filtered := maps.Clone(pq.patchFields)
	for k := range pq.patchFields {
		if !slices.Contains(allow, k) {
			delete(filtered, k)
		}
	}
	return filtered
}

func WithPatchQuery(query PatchQuery) PatchOption {
	return func(pq *patchQuery) {
		pq.searchOpts = query.SearchOpts()
		pq.patchFields = query.PatchFields()
	}
}

func WithPatchQueryOpts(opts ...PatchOption) PatchOption {
	return func(pq *patchQuery) {
		WithPatchQuery(NewPatchQuery(opts...))(pq)
	}
}

func WithPatchFields(patchFields map[string]any) PatchOption {
	return func(pq *patchQuery) {
		pq.patchFields = patchFields
	}
}

func PatchSearchOpts(opts ...search.Option) PatchOption {
	return func(pq *patchQuery) {
		pq.searchOpts = opts
	}
}

func PatchField(name string, value any) PatchOption {
	return func(pq *patchQuery) {
		pq.patchFields[name] = value
	}
}
