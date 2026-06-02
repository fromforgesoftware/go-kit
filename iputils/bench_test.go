package iputils_test

import (
	"net/netip"
	"testing"

	"github.com/fromforgesoftware/go-kit/iputils"
)

func BenchmarkSetContains(b *testing.B) {
	prefixes := []netip.Prefix{
		iputils.MustParsePrefix("10.0.0.0/8"),
		iputils.MustParsePrefix("172.16.0.0/12"),
		iputils.MustParsePrefix("192.168.0.0/16"),
		iputils.MustParsePrefix("100.64.0.0/10"),
		iputils.MustParsePrefix("169.254.0.0/16"),
	}
	s := iputils.NewSet(prefixes...)
	addr := iputils.MustParseAddr("10.5.5.5")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Contains(addr)
	}
}

func BenchmarkContainsAny(b *testing.B) {
	prefixes := []netip.Prefix{
		iputils.MustParsePrefix("10.0.0.0/8"),
		iputils.MustParsePrefix("172.16.0.0/12"),
		iputils.MustParsePrefix("192.168.0.0/16"),
	}
	addr := iputils.MustParseAddr("192.168.5.5")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = iputils.ContainsAny(prefixes, addr)
	}
}
