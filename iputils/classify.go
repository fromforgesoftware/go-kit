package iputils

import "net/netip"

// IsLoopback reports whether addr is in 127.0.0.0/8 (v4) or ::1 (v6).
func IsLoopback(addr netip.Addr) bool { return addr.IsValid() && addr.IsLoopback() }

// IsLinkLocal reports whether addr is in 169.254.0.0/16 (v4) or
// fe80::/10 (v6).
func IsLinkLocal(addr netip.Addr) bool { return addr.IsValid() && addr.IsLinkLocalUnicast() }

// IsMulticast reports whether addr is multicast.
func IsMulticast(addr netip.Addr) bool { return addr.IsValid() && addr.IsMulticast() }

// IsUnspecified reports whether addr is 0.0.0.0 / ::.
func IsUnspecified(addr netip.Addr) bool { return addr.IsValid() && addr.IsUnspecified() }

// Private RFC1918 + RFC4193 prefixes.
var privatePrefixes = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("fc00::/7"), // RFC4193 unique-local
}

// IsPrivate reports whether addr is in any RFC1918 / RFC4193 range.
// netip itself ships IsPrivate() since 1.20; this wrapper exists for
// API symmetry and stable scope across Go versions.
func IsPrivate(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	for _, p := range privatePrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// IsPublic reports whether addr is routable on the public internet.
// (Not loopback, not link-local, not multicast, not unspecified, not
// private.)
func IsPublic(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	switch {
	case addr.IsLoopback(),
		addr.IsLinkLocalUnicast(),
		addr.IsMulticast(),
		addr.IsUnspecified(),
		IsPrivate(addr):
		return false
	}
	return true
}
