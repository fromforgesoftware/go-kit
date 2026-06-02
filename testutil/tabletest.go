package testutil

import "testing"

// TableTest runs a slice of cases as parallel subtests. The kit's
// convention is: leaf tests run in parallel under -race, with
// `require` for setup and `assert` for body — TableTest handles the
// t.Run + t.Parallel boilerplate so the call site stays focused on
// the cases themselves.
//
//	type Case struct {
//	    name     string
//	    in, want int
//	}
//	cases := []Case{
//	    {"zero", 0, 0},
//	    {"one",  1, 2},
//	}
//	testutil.TableTest(t, cases, func(c Case) string { return c.name },
//	    func(t *testing.T, c Case) {
//	        assert.Equal(t, c.want, double(c.in))
//	    })
func TableTest[Case any](
	t *testing.T,
	cases []Case,
	name func(Case) string,
	fn func(t *testing.T, c Case),
) {
	t.Helper()
	for _, tc := range cases {
		tc := tc
		t.Run(name(tc), func(t *testing.T) {
			t.Parallel()
			fn(t, tc)
		})
	}
}

// TableTestSerial is TableTest without t.Parallel — for cases that
// share mutable state (file system, env vars, package globals) and
// must run one at a time. Reach for this only when you've established
// the shared state is unavoidable; the kit-wide default is parallel.
func TableTestSerial[Case any](
	t *testing.T,
	cases []Case,
	name func(Case) string,
	fn func(t *testing.T, c Case),
) {
	t.Helper()
	for _, tc := range cases {
		tc := tc
		t.Run(name(tc), func(t *testing.T) {
			fn(t, tc)
		})
	}
}
