// Package factory is a generic self-registering registry: register
// values (or constructors that produce values) under string keys; look
// them up by key. Solves the "every package re-implements a
// registry" duplication across kit/predicates, kit/sampling,
// kit/milestones, kit/mcp, etc.
//
// Two flavours via the same struct shape:
//   - Registry[T]  — instance registry: register a T, retrieve by key.
//   - Builders[P,T] — constructor registry: register a func(P)(T,error),
//     invoke by key with parameters.
//
// Thread-safe writes; lock-free reads after Freeze() (the common
// init-once / lookup-many pattern).
package factory
