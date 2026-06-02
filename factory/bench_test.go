package factory_test

import (
	"testing"

	"github.com/fromforgesoftware/go-kit/factory"
)

func BenchmarkRegistryGet(b *testing.B) {
	r := factory.New[widget]()
	for _, k := range []string{"a", "b", "c", "d", "e"} {
		_ = r.Register(k, widget{Name: k})
	}
	b.Run("unfrozen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = r.Get("c")
		}
	})
	r.Freeze()
	b.Run("frozen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = r.Get("c")
		}
	})
}

func BenchmarkBuilderBuild(b *testing.B) {
	bs := factory.NewBuilders[int, widget]()
	_ = bs.Register("k", func(n int) (widget, error) { return widget{Name: "k"}, nil })
	bs.Freeze()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = bs.Build("k", i)
	}
}
