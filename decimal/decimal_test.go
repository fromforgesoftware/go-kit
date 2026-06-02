package decimal_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/decimal"
)

func TestArithmetic_Exact(t *testing.T) {
	// The float64 trap: 0.1 + 0.2 must be exactly 0.3.
	got := decimal.RequireFromString("0.1").Add(decimal.RequireFromString("0.2"))
	assert.Equal(t, "0.3", got.String())

	// No drift across a chain.
	d := decimal.RequireFromString("100.00")
	for i := 0; i < 3; i++ {
		d = d.Add(decimal.RequireFromString("0.1")).Sub(decimal.RequireFromString("0.1"))
	}
	assert.True(t, d.Equal(decimal.RequireFromString("100")), "got %s", d)

	assert.Equal(t, "6.25", decimal.RequireFromString("2.5").Mul(decimal.RequireFromString("2.5")).String())
}

func TestRounding_Modes(t *testing.T) {
	half := decimal.RequireFromString("2.5")
	assert.Equal(t, "3", half.Round(0).String(), "half away from zero")
	assert.Equal(t, "2", half.RoundBank(0).String(), "half to even")
	assert.Equal(t, "2", half.RoundDown(0).String(), "truncate")

	assert.Equal(t, "4", decimal.RequireFromString("3.5").RoundBank(0).String(), "half to even rounds up to even")
}

func TestComparisons(t *testing.T) {
	a := decimal.RequireFromString("1.5")
	b := decimal.RequireFromString("2.5")
	assert.True(t, a.LessThan(b))
	assert.True(t, b.GreaterThan(a))
	assert.Equal(t, -1, a.Cmp(b))
	assert.True(t, decimal.Zero.IsZero())
	assert.True(t, a.Neg().IsNegative())
}

func TestJSON_AlwaysString(t *testing.T) {
	type wallet struct {
		Balance decimal.Decimal `json:"balance"`
	}
	b, err := json.Marshal(wallet{Balance: decimal.RequireFromString("0.000000000000000001")})
	require.NoError(t, err)
	// Quoted string — never a float that would lose the 18th decimal.
	assert.JSONEq(t, `{"balance":"0.000000000000000001"}`, string(b))

	var w wallet
	require.NoError(t, json.Unmarshal([]byte(`{"balance":"123.45"}`), &w))
	assert.True(t, w.Balance.Equal(decimal.RequireFromString("123.45")))

	// Bare numbers and null still decode.
	require.NoError(t, json.Unmarshal([]byte(`{"balance":99.5}`), &w))
	assert.True(t, w.Balance.Equal(decimal.RequireFromString("99.5")))
	require.NoError(t, json.Unmarshal([]byte(`{"balance":null}`), &w))
	assert.True(t, w.Balance.IsZero())
}

func TestSQL_RoundTrip(t *testing.T) {
	orig := decimal.RequireFromString("12.34")
	v, err := orig.Value()
	require.NoError(t, err)
	var scanned decimal.Decimal
	require.NoError(t, scanned.Scan(v))
	assert.True(t, scanned.Equal(orig), "got %v", scanned)
}

func TestFromString_Error(t *testing.T) {
	_, err := decimal.FromString("not-a-number")
	assert.Error(t, err)
}
