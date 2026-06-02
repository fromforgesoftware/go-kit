package decimal_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/decimal"
)

func TestAllocate_Examples(t *testing.T) {
	m := decimal.NewMoney(decimal.RequireFromString("1.00"), "USD", 2)
	parts, err := m.Allocate(1, 1, 1)
	require.NoError(t, err)
	require.Len(t, parts, 3)
	// Indivisible cent goes to the first part; the total is conserved exactly.
	assert.Equal(t, "0.34", parts[0].Amount().String())
	assert.Equal(t, "0.33", parts[1].Amount().String())
	assert.Equal(t, "0.33", parts[2].Amount().String())
	assertSumEquals(t, m, parts)
}

func TestAllocate_WeightedRatios(t *testing.T) {
	m := decimal.NewMoney(decimal.RequireFromString("100.00"), "USD", 2)
	parts, err := m.Allocate(70, 30)
	require.NoError(t, err)
	assert.Equal(t, "70", parts[0].Amount().String())
	assert.Equal(t, "30", parts[1].Amount().String())
	assertSumEquals(t, m, parts)
}

func TestSplit_Conserves(t *testing.T) {
	m := decimal.NewMoney(decimal.RequireFromString("10.00"), "USD", 2)
	parts, err := m.Split(3)
	require.NoError(t, err)
	require.Len(t, parts, 3)
	assertSumEquals(t, m, parts) // 3.34 + 3.33 + 3.33 = 10.00
}

// TestAllocate_SumConserved is the load-bearing invariant: across random
// amounts, scales (incl. 18-dp crypto), and ratios, the parts always sum back
// to the original exactly — no minor unit created or lost.
func TestAllocate_SumConserved(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	scales := []int32{0, 2, 8, 18}
	for i := 0; i < 500; i++ {
		scale := scales[rng.Intn(len(scales))]
		coeff := rng.Int63n(1_000_000_000) - 500_000_000
		m := decimal.NewMoney(decimal.New(coeff, -scale), "X", scale)

		n := rng.Intn(5) + 1
		ratios := make([]int, n)
		nonZero := false
		for j := range ratios {
			ratios[j] = rng.Intn(5)
			nonZero = nonZero || ratios[j] > 0
		}
		if !nonZero {
			ratios[0] = 1
		}

		parts, err := m.Allocate(ratios...)
		require.NoError(t, err)
		assertSumEquals(t, m, parts)
	}
}

func TestAllocate_Rejects(t *testing.T) {
	m := decimal.NewMoney(decimal.NewFromInt(10), "USD", 2)
	_, err := m.Allocate()
	assert.ErrorIs(t, err, decimal.ErrBadRatios)
	_, err = m.Allocate(0, 0)
	assert.ErrorIs(t, err, decimal.ErrBadRatios)
	_, err = m.Split(0)
	assert.ErrorIs(t, err, decimal.ErrBadSplit)
}

func TestMoney_AssetMismatch(t *testing.T) {
	usd := decimal.NewMoney(decimal.NewFromInt(1), "USD", 2)
	eur := decimal.NewMoney(decimal.NewFromInt(1), "EUR", 2)
	_, err := usd.Add(eur)
	assert.ErrorIs(t, err, decimal.ErrAssetMismatch)
	_, err = usd.Sub(eur)
	assert.ErrorIs(t, err, decimal.ErrAssetMismatch)
}

func assertSumEquals(t *testing.T, m decimal.Money, parts []decimal.Money) {
	t.Helper()
	sum := decimal.Zero
	for _, p := range parts {
		sum = sum.Add(p.Amount())
	}
	assert.Truef(t, sum.Equal(m.Amount()),
		"sum %s != original %s (parts=%v)", sum, m.Amount(), parts)
}
