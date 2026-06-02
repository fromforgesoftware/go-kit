package testutil_test

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/go-kit/testutil"
)

type doubleCase struct {
	name string
	in   int
	want int
}

func TestTableTest_RunsAllCases(t *testing.T) {
	cases := []doubleCase{
		{"zero", 0, 0},
		{"one", 1, 2},
		{"five", 5, 10},
	}
	var ran atomic.Int32
	// Wrap in an inner t.Run so the test harness blocks here until
	// every parallel subtest has finished — only then can we observe
	// the count.
	t.Run("group", func(t *testing.T) {
		testutil.TableTest(t, cases,
			func(c doubleCase) string { return c.name },
			func(t *testing.T, c doubleCase) {
				ran.Add(1)
				assert.Equal(t, c.want, c.in*2)
			})
	})
	assert.Equal(t, int32(len(cases)), ran.Load())
}

func TestTableTestSerial_RunsAllCases(t *testing.T) {
	cases := []doubleCase{
		{"a", 1, 2},
		{"b", 2, 4},
	}
	var seen []string
	testutil.TableTestSerial(t, cases,
		func(c doubleCase) string { return c.name },
		func(t *testing.T, c doubleCase) {
			seen = append(seen, c.name)
		})
	assert.Equal(t, []string{"a", "b"}, seen)
}
