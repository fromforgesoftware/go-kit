// Package decimal is forge's fixed-point decimal type for money and crypto.
//
// It wraps alpacahq/alpacadecimal (a shopspring/decimal drop-in that keeps the
// common case in a single int64 and falls back to big.Int only when a value
// exceeds that envelope) behind a stable forge API, so the backend can change
// without touching callers. Arithmetic is exact — never float64.
//
// Money adds asset + smallest-unit scale and a sum-conserving Allocate/Split,
// so distributing a total across parts never creates or loses a minor unit.
//
// Crypto note: the fast int64 path covers ~12 dp and |value| ≲ 9.22M; 8-dp
// (sat) and 18-dp (wei) values beyond that stay correct via the big.Int
// fallback. For hot-path crypto balances, prefer integer minor units at the
// asset's scale (Money carries that scale) and widen to Decimal only for
// cross-asset or display math.
package decimal
