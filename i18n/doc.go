// Package i18n provides a framework-agnostic translation engine.
//
// Translations are stored as nested JSON objects keyed by namespace.
// The "::" delimiter separates namespaces so that key names may contain
// dots, underscores, or hyphens (e.g. "shared::enum::role::org.admin").
//
// Lookups follow a fallback chain: exact locale (e.g. "en_US") -> base
// language ("en") -> configured fallback language -> the key itself.
//
// The package does not embed any locale files of its own; consumers
// bring their own translations via NewI18n, LoadFromFS, or LoadFromDir.
// A separate generator (cmd/i18n-gen) produces compile-time-safe key
// constants from a primary locale file.
package i18n
