package decimal

import (
	"math/big"

	"github.com/alpacahq/alpacadecimal"
)

// Decimal is an exact fixed-point number.
type Decimal struct {
	d alpacadecimal.Decimal
}

// Zero is the additive identity.
var Zero = Decimal{}

func wrap(d alpacadecimal.Decimal) Decimal { return Decimal{d: d} }

// New builds coefficient × 10^exp (e.g. New(1230, -2) == 12.30).
func New(coefficient int64, exp int32) Decimal { return wrap(alpacadecimal.New(coefficient, exp)) }

// NewFromInt builds a whole number.
func NewFromInt(i int64) Decimal { return wrap(alpacadecimal.NewFromInt(i)) }

// NewFromBigInt builds value × 10^exp from an arbitrary-precision coefficient.
func NewFromBigInt(value *big.Int, exp int32) Decimal {
	return wrap(alpacadecimal.NewFromBigInt(value, exp))
}

// FromString parses a decimal string (e.g. "12.34", "-0.000000000000000001").
func FromString(s string) (Decimal, error) {
	d, err := alpacadecimal.NewFromString(s)
	if err != nil {
		return Decimal{}, err
	}
	return wrap(d), nil
}

// RequireFromString parses s and panics on error — for constants/tests only.
func RequireFromString(s string) Decimal { return wrap(alpacadecimal.RequireFromString(s)) }

func (d Decimal) Add(o Decimal) Decimal { return wrap(d.d.Add(o.d)) }
func (d Decimal) Sub(o Decimal) Decimal { return wrap(d.d.Sub(o.d)) }
func (d Decimal) Mul(o Decimal) Decimal { return wrap(d.d.Mul(o.d)) }
func (d Decimal) Div(o Decimal) Decimal { return wrap(d.d.Div(o.d)) }

// DivRound divides and rounds (half-away-from-zero) to precision places.
func (d Decimal) DivRound(o Decimal, precision int32) Decimal {
	return wrap(d.d.DivRound(o.d, precision))
}
func (d Decimal) Mod(o Decimal) Decimal { return wrap(d.d.Mod(o.d)) }
func (d Decimal) Neg() Decimal          { return wrap(d.d.Neg()) }
func (d Decimal) Abs() Decimal          { return wrap(d.d.Abs()) }

// Round rounds half away from zero to places decimal places.
func (d Decimal) Round(places int32) Decimal { return wrap(d.d.Round(places)) }

// RoundBank rounds half to even (banker's rounding) — the money default.
func (d Decimal) RoundBank(places int32) Decimal { return wrap(d.d.RoundBank(places)) }

// RoundDown truncates toward zero to places decimal places.
func (d Decimal) RoundDown(places int32) Decimal { return wrap(d.d.RoundDown(places)) }

// Shift moves the decimal point right by shift places (left if negative).
func (d Decimal) Shift(shift int32) Decimal { return wrap(d.d.Shift(shift)) }

func (d Decimal) Cmp(o Decimal) int                 { return d.d.Cmp(o.d) }
func (d Decimal) Equal(o Decimal) bool              { return d.d.Equal(o.d) }
func (d Decimal) GreaterThan(o Decimal) bool        { return d.d.GreaterThan(o.d) }
func (d Decimal) GreaterThanOrEqual(o Decimal) bool { return d.d.GreaterThanOrEqual(o.d) }
func (d Decimal) LessThan(o Decimal) bool           { return d.d.LessThan(o.d) }
func (d Decimal) LessThanOrEqual(o Decimal) bool    { return d.d.LessThanOrEqual(o.d) }
func (d Decimal) IsZero() bool                      { return d.d.IsZero() }
func (d Decimal) IsNegative() bool                  { return d.d.IsNegative() }
func (d Decimal) IsPositive() bool                  { return d.d.IsPositive() }
func (d Decimal) Sign() int                         { return d.d.Sign() }

func (d Decimal) String() string                  { return d.d.String() }
func (d Decimal) StringFixed(places int32) string { return d.d.StringFixed(places) }
func (d Decimal) BigInt() *big.Int                { return d.d.BigInt() }
func (d Decimal) Exponent() int32                 { return d.d.Exponent() }
func (d Decimal) IntPart() int64                  { return d.d.IntPart() }

// InexactFloat64 returns the nearest float64 — lossy; for display/metrics only,
// never for money arithmetic.
func (d Decimal) InexactFloat64() float64 { return d.d.InexactFloat64() }
