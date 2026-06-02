package repositorytest

import (
	"reflect"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/searchtest"
)

type matchPatchOptCfg struct {
	keyValMatcher map[string]func(any) bool
	searchMatcher func([]search.Option) bool
}

type MatchPatchOptOption func(*matchPatchOptCfg)

func MatchPatchOptValueMatcher[T any](fName string, valMatcher func(T) bool) MatchPatchOptOption {
	return func(cfg *matchPatchOptCfg) {
		cfg.keyValMatcher[fName] = func(val any) bool {
			var v T
			if val != nil {
				v = val.(T)
			}
			return valMatcher(v)
		}
	}
}

func MatchPatchOpts(want []repository.PatchOption, opts ...MatchPatchOptOption) func([]repository.PatchOption) bool {
	return func(got []repository.PatchOption) bool {
		if len(want) != len(got) {
			return false
		}

		for i, wantOpt := range want {
			if !MatchPatchOpt(wantOpt, opts...)(got[i]) {
				return false
			}
		}

		return true
	}
}

func MatchPatchSearchWithMatcher(f func([]search.Option) bool) MatchPatchOptOption {
	return func(cfg *matchPatchOptCfg) {
		cfg.searchMatcher = f
	}
}

func MatchPatchOpt(want repository.PatchOption, opts ...MatchPatchOptOption) func(repository.PatchOption) bool {
	return func(got repository.PatchOption) bool {
		cfg := matchPatchOptCfg{
			keyValMatcher: make(map[string]func(any) bool),
			searchMatcher: searchtest.OptsMatcherFunc(repository.NewPatchQuery(want).SearchOpts()...),
		}
		for _, opt := range opts {
			opt(&cfg)
		}

		wantQuery := repository.NewPatchQuery(want)
		gotQuery := repository.NewPatchQuery(got)

		if (len(wantQuery.SearchOpts()) != len(gotQuery.SearchOpts())) ||
			(len(wantQuery.PatchFields()) != len(gotQuery.PatchFields())) {
			return false
		}
		if len(wantQuery.SearchOpts()) > 0 {
			if !cfg.searchMatcher(gotQuery.SearchOpts()) {
				return false
			}
		}

		for key, wantVal := range wantQuery.PatchFields() {
			gotVal, ok := gotQuery.PatchFields()[key]
			if !ok {
				return false
			}
			if !matchPatchFieldVal(&cfg, key, wantVal, gotVal) {
				return false
			}
		}

		return true
	}
}

func matchPatchFieldVal(cfg *matchPatchOptCfg, key string, wantVal, gotVal any) bool {
	if cfg.keyValMatcher[key] == nil {
		return reflect.DeepEqual(gotVal, wantVal)
	}
	return cfg.keyValMatcher[key](gotVal)
}
