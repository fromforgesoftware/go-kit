// Package predicates provides a pluggable condition registry: typed
// predicates that evaluate a Source (caller-supplied context) and compose
// with And/Or/Not. Predicates can be constructed from data (Spec → tree)
// for config-driven feature flagging, A/B segmentation, policy rules,
// ability gating, loot drop conditions, alert rules.
package predicates
