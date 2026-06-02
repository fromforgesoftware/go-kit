package decimal_test

import (
	"testing"

	"github.com/fromforgesoftware/go-kit/decimal"
)

// Benchmarks span fiat (2-dp) and crypto (8-dp sat, 18-dp wei) so the int64
// fast-path boundary is measured, not assumed: ~12-dp values stay allocation-
// free; 18-dp values cross into the big.Int fallback. Run with -benchmem.
var benchSink decimal.Decimal

func benchPair(scale int32) (decimal.Decimal, decimal.Decimal) {
	switch scale {
	case 8:
		return decimal.RequireFromString("1.23456789"), decimal.RequireFromString("9.87654321")
	case 18:
		return decimal.RequireFromString("1.234567890123456789"),
			decimal.RequireFromString("9.876543210987654321")
	default:
		return decimal.RequireFromString("123.45"), decimal.RequireFromString("67.89")
	}
}

func benchAdd(b *testing.B, scale int32) {
	x, y := benchPair(scale)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSink = x.Add(y)
	}
}

func benchMul(b *testing.B, scale int32) {
	x, y := benchPair(scale)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSink = x.Mul(y)
	}
}

func benchString(b *testing.B, scale int32) {
	x, _ := benchPair(scale)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = x.String()
	}
}

func BenchmarkAdd_2dp(b *testing.B)     { benchAdd(b, 2) }
func BenchmarkAdd_8dp(b *testing.B)     { benchAdd(b, 8) }
func BenchmarkAdd_18dp(b *testing.B)    { benchAdd(b, 18) }
func BenchmarkMul_2dp(b *testing.B)     { benchMul(b, 2) }
func BenchmarkMul_8dp(b *testing.B)     { benchMul(b, 8) }
func BenchmarkMul_18dp(b *testing.B)    { benchMul(b, 18) }
func BenchmarkString_2dp(b *testing.B)  { benchString(b, 2) }
func BenchmarkString_18dp(b *testing.B) { benchString(b, 18) }

func BenchmarkAllocate(b *testing.B) {
	m := decimal.NewMoney(decimal.RequireFromString("1000.00"), "USD", 2)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Allocate(1, 1, 1)
	}
}
