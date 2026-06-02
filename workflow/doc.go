// Package workflow is a state-machine engine with persistent
// checkpoints: a running instance can crash, restart, and resume from
// its last recorded state without losing position in the flow.
//
// Distinct from kit/fsm, which is purely in-memory: workflow adds a
// Store, idempotent event delivery, compensation hooks for rollback,
// and a Resume entrypoint for picking up in-flight instances after a
// process restart.
//
// Use cases:
//   - Order processing / shipping pipelines.
//   - KYC / payment-authorisation multi-step flows.
//   - CI run state, deployment pipelines.
//   - Game encounter state with mid-encounter persistence.
//
// State and Event must be string-aliased types so that transitions can
// be safely serialised. The package ships an in-memory Store; Postgres
// and other backends plug in behind the Store interface.
package workflow
