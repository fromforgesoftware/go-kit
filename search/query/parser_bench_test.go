package query_test

import (
	"net/url"
	"testing"

	"github.com/fromforgesoftware/go-kit/search/query"
)

// BenchmarkParseURLQueryOpts measures the hot-path parser used by
// every JSON:API list endpoint to turn `?filter[...]=...&sort=...`
// query strings into typed Options. Regressions show up in any
// service whose request volume is read-heavy.
func BenchmarkParseURLQueryOpts(b *testing.B) {
	// Representative query — multiple filters, sort, pagination,
	// includes. Mirrors a real JSON:API list call.
	raw := "filter[name][like]=rex&filter[status][eq]=available&filter[tag][in]=dog,cat" +
		"&sort=-created_at,name" +
		"&page[number]=2&page[size]=50" +
		"&include=owner,tags"
	u, err := url.Parse("/v1/pets?" + raw)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := query.ParseURLQueryOpts(u)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseOperator covers the smaller operator-string parser
// hit once per filter on every list call.
func BenchmarkParseOperator(b *testing.B) {
	ops := []string{"eq", "ne", "gt", "gte", "lt", "lte", "in", "nin", "like", "ilike"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = query.ParseOperator(ops[i%len(ops)])
	}
}
