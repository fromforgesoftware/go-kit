// Package iputils is small, allocation-aware helpers on top of
// net/netip for the patterns every service eventually re-implements:
// CIDR allowlists / denylists, classification (loopback, private,
// link-local), efficient many-prefix containment via a compiled Set.
//
// Built on netip.Addr / netip.Prefix throughout — never the legacy
// net.IP. Set is the only stateful type; everything else is
// stateless functions.
package iputils
