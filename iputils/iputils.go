package iputils

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"
)

// ParseAddr is netip.ParseAddr with a clearer error wrap.
func ParseAddr(s string) (netip.Addr, error) {
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("iputils: parse addr %q: %w", s, err)
	}
	return a, nil
}

// ParsePrefix is netip.ParsePrefix with a clearer error wrap.
func ParsePrefix(s string) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("iputils: parse prefix %q: %w", s, err)
	}
	return p, nil
}

// MustParseAddr panics on error. For test fixtures and init-time
// constants only.
func MustParseAddr(s string) netip.Addr {
	a, err := ParseAddr(s)
	if err != nil {
		panic(err)
	}
	return a
}

// MustParsePrefix panics on error. For test fixtures and init-time
// constants only.
func MustParsePrefix(s string) netip.Prefix {
	p, err := ParsePrefix(s)
	if err != nil {
		panic(err)
	}
	return p
}

// Contains reports whether prefix contains addr. Cross-family inputs
// (an IPv4 addr against an IPv6 prefix or vice versa) return false.
func Contains(prefix netip.Prefix, addr netip.Addr) bool {
	if !prefix.IsValid() || !addr.IsValid() {
		return false
	}
	if prefix.Addr().Is4() != addr.Is4() {
		return false
	}
	return prefix.Contains(addr)
}

// ContainsAny reports whether any prefix in prefixes contains addr.
// For repeated lookups over a large list prefer Set.
func ContainsAny(prefixes []netip.Prefix, addr netip.Addr) bool {
	for _, p := range prefixes {
		if Contains(p, addr) {
			return true
		}
	}
	return false
}

// Set is a compiled multi-prefix container. Lookups are O(P) over the
// prefix count but with no allocations on the hot path; suitable for
// allowlist / denylist sizes up to a few thousand.
type Set struct {
	mu sync.RWMutex
	v4 []netip.Prefix // sorted by bit length descending (longest-match first)
	v6 []netip.Prefix
}

// NewSet builds a Set from the given prefixes. Order is not preserved.
func NewSet(prefixes ...netip.Prefix) *Set {
	s := &Set{}
	for _, p := range prefixes {
		s.Add(p)
	}
	return s
}

// Add registers an additional prefix. Safe to call concurrently with
// Contains.
func (s *Set) Add(prefix netip.Prefix) {
	if !prefix.IsValid() {
		return
	}
	p := prefix.Masked()
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.Addr().Is4() {
		s.v4 = insertSortedDescByBits(s.v4, p)
	} else {
		s.v6 = insertSortedDescByBits(s.v6, p)
	}
}

// Contains reports whether any prefix in the set contains addr.
func (s *Set) Contains(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pool := s.v6
	if addr.Is4() {
		pool = s.v4
	}
	for _, p := range pool {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// String returns a comma-joined prefix list, longest-match first.
func (s *Set) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	parts := make([]string, 0, len(s.v4)+len(s.v6))
	for _, p := range s.v4 {
		parts = append(parts, p.String())
	}
	for _, p := range s.v6 {
		parts = append(parts, p.String())
	}
	return strings.Join(parts, ", ")
}

// Len returns the total prefix count.
func (s *Set) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.v4) + len(s.v6)
}

func insertSortedDescByBits(dst []netip.Prefix, p netip.Prefix) []netip.Prefix {
	idx := sort.Search(len(dst), func(i int) bool {
		return dst[i].Bits() < p.Bits()
	})
	dst = append(dst, netip.Prefix{})
	copy(dst[idx+1:], dst[idx:])
	dst[idx] = p
	return dst
}
