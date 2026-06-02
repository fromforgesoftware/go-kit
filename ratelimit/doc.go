// Package ratelimit is a token-bucket rate limiter for forge services.
//
// A Limiter decides whether a keyed caller may proceed under a Policy
// (limit per window + burst). Keys are caller-defined: per org, per API key,
// per client IP, or — in Conduit — per upstream provider so a fleet of
// tenants can't collectively trip an exchange's API limits.
//
// The token-bucket math lives in the Store, so the in-memory store guards it
// with a mutex and a future Redis store can do it atomically in Lua for
// multi-replica correctness. HTTP middleware and a gRPC interceptor wire a
// Limiter onto a transport with the standard RateLimit-* / Retry-After headers
// and a 429 on denial.
package ratelimit
