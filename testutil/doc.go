// Package testutil collects the small, layer-agnostic helpers every
// forge test eventually wants — table-driven runners, HTTP response
// assertions, and a handful of golden-path predicates.
//
// Per-layer harnesses (HTTP handler suites, gnomock-backed
// integration DBs, mock factories, fixture builders) live next to
// their domain in the kit, not here:
//
//   - Transport (HTTP): go/kit/transport/rest/restest
//   - Database (integration): go/kit/persistence/sqldb/sqldbtest
//   - JSON:API encoding: go/kit/jsonapi/jsonapitest
//   - Generated fixtures: produced by `forge generate fixtures` (§23)
//   - Mocks: produced by `forge generate mocks` / mockery v3
//
// See docs/cookbook/testing.md for the canonical patterns per layer.
package testutil
