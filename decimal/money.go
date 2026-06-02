package decimal

import (
	"errors"
	"math/big"
)

var (
	ErrAssetMismatch = errors.New("decimal: money asset mismatch")
	ErrBadRatios     = errors.New("decimal: ratios must be non-empty and sum positive")
	ErrBadSplit      = errors.New("decimal: split count must be positive")
)

// Money is an Amount in an asset, rounded to the asset's smallest unit (scale:
// 2 for USD, 8 for BTC/sat, 18 for ETH/wei).
type Money struct {
	amount Decimal
	asset  string
	scale  int32
}

// NewMoney builds a Money, truncating amount to the asset's scale.
func NewMoney(amount Decimal, asset string, scale int32) Money {
	return Money{amount: amount.RoundDown(scale), asset: asset, scale: scale}
}

func (m Money) Amount() Decimal { return m.amount }
func (m Money) Asset() string   { return m.asset }
func (m Money) Scale() int32    { return m.scale }
func (m Money) IsZero() bool    { return m.amount.IsZero() }
func (m Money) String() string  { return m.amount.StringFixed(m.scale) + " " + m.asset }

func (m Money) Add(o Money) (Money, error) {
	if m.asset != o.asset {
		return Money{}, ErrAssetMismatch
	}
	return Money{m.amount.Add(o.amount), m.asset, m.scale}, nil
}

func (m Money) Sub(o Money) (Money, error) {
	if m.asset != o.asset {
		return Money{}, ErrAssetMismatch
	}
	return Money{m.amount.Sub(o.amount), m.asset, m.scale}, nil
}

// minor returns the amount in smallest units as a big.Int.
func (m Money) minor() *big.Int { return m.amount.Shift(m.scale).BigInt() }

func (m Money) fromMinor(units *big.Int) Money {
	return Money{NewFromBigInt(units, -m.scale), m.asset, m.scale}
}

// Allocate splits the amount across the given integer ratios, conserving the
// sum exactly — no minor unit created or lost. Any indivisible remainder is
// handed out one unit at a time to the first parts (round-robin).
func (m Money) Allocate(ratios ...int) ([]Money, error) {
	if len(ratios) == 0 {
		return nil, ErrBadRatios
	}
	total := int64(0)
	for _, r := range ratios {
		if r < 0 {
			return nil, ErrBadRatios
		}
		total += int64(r)
	}
	if total == 0 {
		return nil, ErrBadRatios
	}

	units := m.minor()
	sum := big.NewInt(total)
	out := make([]Money, len(ratios))
	allocated := new(big.Int)
	for i, r := range ratios {
		share := new(big.Int).Mul(units, big.NewInt(int64(r)))
		share.Quo(share, sum) // truncate toward zero
		out[i] = m.fromMinor(share)
		allocated.Add(allocated, share)
	}

	remainder := new(big.Int).Sub(units, allocated)
	step := big.NewInt(1)
	if remainder.Sign() < 0 { // negative total: hand out -1 units
		step.Neg(step)
		remainder.Neg(remainder)
	}
	for i := 0; remainder.Sign() > 0; i = (i + 1) % len(out) {
		out[i] = out[i].addMinor(step)
		remainder.Sub(remainder, big.NewInt(1))
	}
	return out, nil
}

func (m Money) addMinor(units *big.Int) Money {
	return m.fromMinor(new(big.Int).Add(m.minor(), units))
}

// Split divides the amount into n equal sum-conserving parts.
func (m Money) Split(n int) ([]Money, error) {
	if n <= 0 {
		return nil, ErrBadSplit
	}
	ratios := make([]int, n)
	for i := range ratios {
		ratios[i] = 1
	}
	return m.Allocate(ratios...)
}
