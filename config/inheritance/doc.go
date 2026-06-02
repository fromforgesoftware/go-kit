// Package inheritance is a hierarchical config resolver: combine
// multiple partial-config layers (defaults → env → scope → instance)
// into a fully-realised config by taking the highest-priority
// definition per field.
//
// Each field is wrapped in Optional[T] to distinguish "set" from
// "unset". Append-merge for slices and union-merge for maps are
// available via the `cfg:"...,merge=…"` tag; the default is override.
//
// Complements kit/sops + envconfig (boot-time parsing) by handling
// runtime layered lookup with merging.
package inheritance
