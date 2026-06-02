package iputils_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/iputils"
)

func TestParseAddr(t *testing.T) {
	_, err := iputils.ParseAddr("not-an-ip")
	assert.Error(t, err)
	a, err := iputils.ParseAddr("1.2.3.4")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3.4", a.String())
}

func TestParsePrefix(t *testing.T) {
	_, err := iputils.ParsePrefix("1.2.3.4/x")
	assert.Error(t, err)
	p, err := iputils.ParsePrefix("10.0.0.0/8")
	require.NoError(t, err)
	assert.Equal(t, 8, p.Bits())
}

func TestContainsCrossFamilyReturnsFalse(t *testing.T) {
	v4 := iputils.MustParsePrefix("10.0.0.0/8")
	v6 := iputils.MustParseAddr("::1")
	assert.False(t, iputils.Contains(v4, v6))
}

func TestContainsBasic(t *testing.T) {
	p := iputils.MustParsePrefix("10.0.0.0/8")
	assert.True(t, iputils.Contains(p, iputils.MustParseAddr("10.5.5.5")))
	assert.False(t, iputils.Contains(p, iputils.MustParseAddr("11.0.0.1")))
}

func TestContainsAny(t *testing.T) {
	prefixes := []netip.Prefix{
		iputils.MustParsePrefix("10.0.0.0/8"),
		iputils.MustParsePrefix("192.168.0.0/16"),
	}
	assert.True(t, iputils.ContainsAny(prefixes, iputils.MustParseAddr("192.168.5.5")))
	assert.False(t, iputils.ContainsAny(prefixes, iputils.MustParseAddr("8.8.8.8")))
}

func TestSetMixedFamily(t *testing.T) {
	s := iputils.NewSet(
		iputils.MustParsePrefix("10.0.0.0/8"),
		iputils.MustParsePrefix("192.168.0.0/16"),
		iputils.MustParsePrefix("fc00::/7"),
	)
	assert.True(t, s.Contains(iputils.MustParseAddr("10.5.5.5")))
	assert.True(t, s.Contains(iputils.MustParseAddr("192.168.1.1")))
	assert.True(t, s.Contains(iputils.MustParseAddr("fd00::1")))
	assert.False(t, s.Contains(iputils.MustParseAddr("8.8.8.8")))
	assert.False(t, s.Contains(iputils.MustParseAddr("2606:4700::1")))
}

func TestSetLongestMatchOrder(t *testing.T) {
	s := iputils.NewSet(
		iputils.MustParsePrefix("10.0.0.0/8"),
		iputils.MustParsePrefix("10.5.0.0/16"),
		iputils.MustParsePrefix("10.5.5.0/24"),
	)
	str := s.String()
	// Longest-match first: /24 before /16 before /8.
	assert.Less(t, indexOf(str, "10.5.5.0/24"), indexOf(str, "10.5.0.0/16"))
	assert.Less(t, indexOf(str, "10.5.0.0/16"), indexOf(str, "10.0.0.0/8"))
}

func TestSetIgnoresInvalid(t *testing.T) {
	s := iputils.NewSet(netip.Prefix{})
	assert.Equal(t, 0, s.Len())
}

func TestClassifyLoopback(t *testing.T) {
	assert.True(t, iputils.IsLoopback(iputils.MustParseAddr("127.0.0.1")))
	assert.True(t, iputils.IsLoopback(iputils.MustParseAddr("::1")))
	assert.False(t, iputils.IsLoopback(iputils.MustParseAddr("8.8.8.8")))
}

func TestClassifyPrivate(t *testing.T) {
	for _, ip := range []string{"10.5.5.5", "172.16.1.1", "192.168.1.1", "fd00::1"} {
		assert.Truef(t, iputils.IsPrivate(iputils.MustParseAddr(ip)), "%s should be private", ip)
	}
	for _, ip := range []string{"8.8.8.8", "2606:4700::1"} {
		assert.Falsef(t, iputils.IsPrivate(iputils.MustParseAddr(ip)), "%s should be public", ip)
	}
}

func TestClassifyPublic(t *testing.T) {
	assert.True(t, iputils.IsPublic(iputils.MustParseAddr("8.8.8.8")))
	assert.False(t, iputils.IsPublic(iputils.MustParseAddr("127.0.0.1")))
	assert.False(t, iputils.IsPublic(iputils.MustParseAddr("10.0.0.1")))
	assert.False(t, iputils.IsPublic(iputils.MustParseAddr("0.0.0.0")))
}

func TestInvalidAddrEverywhereFalse(t *testing.T) {
	var z netip.Addr
	assert.False(t, iputils.IsLoopback(z))
	assert.False(t, iputils.IsPrivate(z))
	assert.False(t, iputils.IsPublic(z))
	assert.False(t, iputils.IsMulticast(z))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
