// Package auth provides authentication primitives and middleware components.
// It supports token abstraction, extraction from HTTP/gRPC requests, validation
// via Firebase, and context injection.
//
// Key components:
// - Token: Represents an authentication token with claims.
// - Authenticator: Validates tokens (currently supports Firebase ID tokens).
// - TokenExtractor: Extracts tokens from requests (HTTP headers or gRPC metadata).
// - ContextInjector: Injects validated tokens into the context for usage by services.
package auth
