package i18n_test

import (
	"fmt"
	"testing"

	"github.com/fromforgesoftware/go-kit/i18n"
)

// benchFixture builds a representative translation set:
//   - depth-3/4 nesting (matches real-world key shapes)
//   - 1000 leaf keys per locale
//   - 5 locales including one regional variant
//   - mix of plain strings and {{name}}-style interpolated strings
//
// Numbers chosen to match a mid-sized web app's en.json (~1k strings).
func benchFixture() *i18n.I18n {
	build := func() i18n.Translation {
		t := i18n.Translation{}
		// common::keyN  (250 keys, mostly plain)
		common := map[string]any{}
		for n := 0; n < 250; n++ {
			common[fmt.Sprintf("key%d", n)] = fmt.Sprintf("Value %d", n)
		}
		t["common"] = common

		// auth::greeting::keyN  (250 keys with {{name}} interpolation)
		greeting := map[string]any{}
		for n := 0; n < 250; n++ {
			greeting[fmt.Sprintf("msg%d", n)] = fmt.Sprintf("Hello, {{name}}! You have %d messages.", n)
		}
		t["auth"] = map[string]any{"greeting": greeting}

		// shared::enum::role::xN.adminM  (500 enum-style keys with dots)
		roles := map[string]any{}
		for n := 0; n < 500; n++ {
			roles[fmt.Sprintf("role.%d", n)] = fmt.Sprintf("Role %d", n)
		}
		t["shared"] = map[string]any{"enum": map[string]any{"role": roles}}

		return t
	}

	return i18n.NewI18n("en", "en", map[string]i18n.Translation{
		"en":    build(),
		"es":    build(),
		"fr":    build(),
		"de":    build(),
		"es_MX": {"common": map[string]any{"key0": "Override"}},
	})
}

// Baseline: simple top-level key.
func BenchmarkTranslate_ShallowHit(b *testing.B) {
	i := benchFixture()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = i.Translate("en", "common::key123")
	}
}

// Deep key (typical for enum/permission tables).
func BenchmarkTranslate_DeepHit(b *testing.B) {
	i := benchFixture()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = i.Translate("en", "shared::enum::role::role.250")
	}
}

// Worst case: regional locale that misses, base hits.
func BenchmarkTranslate_RegionalFallback(b *testing.B) {
	i := benchFixture()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = i.Translate("es_MX", "common::key50")
	}
}

// Locale unknown -> must fall back to the configured fallback language.
func BenchmarkTranslate_FallbackToDefault(b *testing.B) {
	i := benchFixture()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = i.Translate("ja", "common::key75")
	}
}

// Total miss — returns the key.
func BenchmarkTranslate_Miss(b *testing.B) {
	i := benchFixture()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = i.Translate("en", "no::such::key")
	}
}

// Interpolation path — exercises the regex + replace.
func BenchmarkTranslate_WithInterpolation(b *testing.B) {
	i := benchFixture()
	params := map[string]string{"name": "Ada"}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = i.Translate("en", "auth::greeting::msg42", params)
	}
}

// Interpolation requested but the string has no placeholders — exercises
// the fast-path skip after optimization.
func BenchmarkTranslate_ParamsButNoPlaceholder(b *testing.B) {
	i := benchFixture()
	params := map[string]string{"name": "Ada"}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = i.Translate("en", "common::key10", params)
	}
}

// GetKeyValues — typical "populate a dropdown" call.
func BenchmarkGetKeyValues(b *testing.B) {
	i := benchFixture()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = i.GetKeyValues("en", "shared::enum::role")
	}
}

// GetKeys — typical "list available keys" call.
func BenchmarkGetKeys(b *testing.B) {
	i := benchFixture()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = i.GetKeys("en", "common")
	}
}

// Realistic page render: translate 50 strings in a tight loop, mix of
// hits, deep keys, and one interpolation.
func BenchmarkPageRender_50Strings(b *testing.B) {
	i := benchFixture()
	params := map[string]string{"name": "Ada"}
	keys := make([]string, 50)
	for n := 0; n < 25; n++ {
		keys[n] = fmt.Sprintf("common::key%d", n)
	}
	for n := 25; n < 49; n++ {
		keys[n] = fmt.Sprintf("shared::enum::role::role.%d", n)
	}
	keys[49] = "auth::greeting::msg10"
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for _, k := range keys {
			_ = i.Translate("en", k, params)
		}
	}
}
