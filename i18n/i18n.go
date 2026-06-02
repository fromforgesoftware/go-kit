package i18n

import (
	"sort"
	"strings"
)

// NamespaceDelimiter separates namespaces in a translation key.
// Dots, underscores, and hyphens are preserved as part of key names.
const NamespaceDelimiter = "::"

// Translation is a nested map of translation values. Leaf values are
// strings; intermediate nodes are nested Translation maps.
type Translation map[string]any

// I18n is a stateless translation engine. Safe for concurrent use; the
// internal state is built once in NewI18n and never mutated after that.
//
// Internally each locale is held in two shapes:
//   - the original nested tree, used by GetKeys / GetKeyValues which
//     need subtree access;
//   - a flattened map[fullKey]value built at construction time, used by
//     the hot Translate / HasTranslation paths so each lookup is a
//     single map probe with no string splitting or tree-walking.
//
// This costs roughly one extra string pointer per (locale, leaf-key)
// pair — kilobytes for a typical app — and turns Translate into an
// allocation-free O(1) call once the value is found.
type I18n struct {
	defaultLanguage  string
	fallbackLanguage string
	translations     map[string]Translation
	flat             map[string]map[string]string
}

// NewI18n creates a new I18n instance from an in-memory map of
// translations keyed by locale code. The nested trees are walked once
// to build per-locale flat lookup tables; after this call the result
// is safe for concurrent reads.
func NewI18n(defaultLanguage, fallbackLanguage string, translations map[string]Translation) *I18n {
	flat := make(map[string]map[string]string, len(translations))
	for locale, tree := range translations {
		m := make(map[string]string, 64)
		flatten(tree, "", m)
		flat[locale] = m
	}
	return &I18n{
		defaultLanguage:  defaultLanguage,
		fallbackLanguage: fallbackLanguage,
		translations:     translations,
		flat:             flat,
	}
}

// Translate resolves key for the requested locale, falling back to the
// base language (e.g. "en" for "en_US") and then the configured
// fallback language. If no translation is found the key is returned
// unchanged. When params is supplied, {{name}} placeholders are
// interpolated; passing no params (or a nil map) skips interpolation
// entirely.
func (i *I18n) Translate(locale, key string, params ...map[string]string) string {
	value, ok := i.lookup(locale, key)
	if !ok && locale != i.fallbackLanguage {
		value, ok = i.lookup(i.fallbackLanguage, key)
	}
	if !ok {
		value = key
	}
	if len(params) > 0 && params[0] != nil {
		return interpolate(value, params[0])
	}
	return value
}

// GetDefaultLanguage returns the default language code.
func (i *I18n) GetDefaultLanguage() string { return i.defaultLanguage }

// GetFallbackLanguage returns the fallback language code.
func (i *I18n) GetFallbackLanguage() string { return i.fallbackLanguage }

// GetAvailableLanguages returns the sorted list of loaded locale codes.
func (i *I18n) GetAvailableLanguages() []string {
	out := make([]string, 0, len(i.translations))
	for lang := range i.translations {
		out = append(out, lang)
	}
	sort.Strings(out)
	return out
}

// HasTranslation reports whether a translation exists for key in the
// requested locale (including base-language fallback).
func (i *I18n) HasTranslation(locale, key string) bool {
	_, ok := i.lookup(locale, key)
	return ok
}

// GetKeys returns every leaf key under parentKey, with the full
// namespace prefix preserved. Results are merged across exact locale,
// base language, and the fallback language so the caller sees the
// complete set of available keys regardless of which file defines them.
// Pass an empty parentKey to walk the full tree.
func (i *I18n) GetKeys(locale, parentKey string) []string {
	seen := make(map[string]struct{})
	collect := func(lang string) {
		obj := i.resolveObject(parentKey, lang)
		if obj == nil {
			return
		}
		var keys []string
		collectLeafKeys(obj, parentKey, &keys)
		for _, k := range keys {
			seen[k] = struct{}{}
		}
	}

	collect(locale)
	if base := baseLanguage(locale); base != locale {
		collect(base)
	}
	if locale != i.fallbackLanguage && baseLanguage(locale) != i.fallbackLanguage {
		collect(i.fallbackLanguage)
	}

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// GetKeyValues returns direct string children of parentKey as a
// {localKey: translation} map, suitable for populating dropdowns and
// enum-like UIs. Nested objects are ignored. Translation values are
// resolved using the same fallback chain as Translate, with optional
// placeholder interpolation.
func (i *I18n) GetKeyValues(locale, parentKey string, params ...map[string]string) map[string]string {
	out := make(map[string]string)
	taken := make(map[string]struct{})

	merge := func(lang string) {
		obj := i.resolveObject(parentKey, lang)
		if obj == nil {
			return
		}
		for key, value := range obj {
			if _, ok := taken[key]; ok {
				continue
			}
			if _, isString := value.(string); !isString {
				continue
			}
			full := key
			if parentKey != "" {
				full = parentKey + NamespaceDelimiter + key
			}
			out[key] = i.Translate(locale, full, params...)
			taken[key] = struct{}{}
		}
	}

	merge(locale)
	if base := baseLanguage(locale); base != locale {
		merge(base)
	}
	if locale != i.fallbackLanguage && baseLanguage(locale) != i.fallbackLanguage {
		merge(i.fallbackLanguage)
	}
	return out
}

// KeyValue is an ordered (id, value) pair returned by GetValues.
type KeyValue struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// GetValues is the slice-of-pairs variant of GetKeyValues, sorted by
// id. Convenient for API responses where map ordering is unwanted.
func (i *I18n) GetValues(locale, parentKey string, params ...map[string]string) []KeyValue {
	kv := i.GetKeyValues(locale, parentKey, params...)
	out := make([]KeyValue, 0, len(kv))
	for id, value := range kv {
		out = append(out, KeyValue{ID: id, Value: value})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

// lookup is the hot path: a flat-map probe in the exact locale, then
// the base language. Returns (value, true) only for non-empty hits —
// empty-string translations are treated as missing to match historical
// behaviour and let the fallback chain run.
func (i *I18n) lookup(locale, key string) (string, bool) {
	if m := i.flat[locale]; m != nil {
		if v := m[key]; v != "" {
			return v, true
		}
	}
	if base := baseLanguage(locale); base != locale {
		if m := i.flat[base]; m != nil {
			if v := m[key]; v != "" {
				return v, true
			}
		}
	}
	return "", false
}

// resolveObject returns the subtree at parentKey, or nil when the path
// resolves to a string or doesn't exist. An empty parentKey returns
// the whole root for the locale.
func (i *I18n) resolveObject(parentKey, language string) map[string]any {
	translations, ok := i.translations[language]
	if !ok {
		return nil
	}
	if parentKey == "" {
		return translations
	}
	current := map[string]any(translations)
	for _, segment := range strings.Split(parentKey, NamespaceDelimiter) {
		if current == nil {
			return nil
		}
		val, ok := current[segment]
		if !ok {
			return nil
		}
		next, isObj := val.(map[string]any)
		if !isObj {
			return nil
		}
		current = next
	}
	return current
}

// baseLanguage returns "en" for "en_US", or the input unchanged when
// there is no underscore.
func baseLanguage(locale string) string {
	if idx := strings.IndexByte(locale, '_'); idx > 0 {
		return locale[:idx]
	}
	return locale
}

// flatten walks the nested tree and writes every leaf into out keyed by
// its full "::"-joined path.
func flatten(obj map[string]any, prefix string, out map[string]string) {
	for key, value := range obj {
		full := key
		if prefix != "" {
			full = prefix + NamespaceDelimiter + key
		}
		switch v := value.(type) {
		case string:
			out[full] = v
		case map[string]any:
			flatten(v, full, out)
		}
	}
}

// collectLeafKeys appends the full path of every string leaf under obj.
func collectLeafKeys(obj map[string]any, prefix string, out *[]string) {
	for key, value := range obj {
		full := key
		if prefix != "" {
			full = prefix + NamespaceDelimiter + key
		}
		switch v := value.(type) {
		case string:
			*out = append(*out, full)
		case map[string]any:
			collectLeafKeys(v, full, out)
		}
	}
}

// interpolate substitutes {{name}} placeholders using params. Missing
// keys are left as-is so the caller can spot them during development.
//
// Fast paths:
//   - strings with no "{{" return immediately, no allocation;
//   - the scanner is hand-written so we avoid a regexp allocation on
//     every call (the previous compiled-regex version still allocated
//     a temporary []byte during ReplaceAllStringFunc).
func interpolate(text string, params map[string]string) string {
	start := strings.Index(text, "{{")
	if start < 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	for start >= 0 {
		b.WriteString(text[:start])
		rest := text[start+2:]
		end := strings.Index(rest, "}}")
		if end < 0 {
			// Unterminated — emit the rest as literal and stop.
			b.WriteString(text[start:])
			return b.String()
		}
		key := rest[:end]
		if isWord(key) {
			if val, ok := params[key]; ok {
				b.WriteString(val)
			} else {
				// Unknown placeholder — preserve as-is for visibility.
				b.WriteString("{{")
				b.WriteString(key)
				b.WriteString("}}")
			}
		} else {
			// Doesn't match \w+ — leave the literal in place.
			b.WriteString("{{")
			b.WriteString(key)
			b.WriteString("}}")
		}
		text = rest[end+2:]
		start = strings.Index(text, "{{")
	}
	b.WriteString(text)
	return b.String()
}

// isWord reports whether s matches /^\w+$/ (the placeholder grammar
// used by the original workair regex).
func isWord(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_':
		default:
			return false
		}
	}
	return true
}
