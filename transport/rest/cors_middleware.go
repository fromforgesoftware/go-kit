package rest

import (
	"net/http"
	"strings"
)

// CORSMiddleware provides Cross-Origin Resource Sharing support.
//
// Per the Fetch spec the response must include `Vary: Origin` on every
// cross-origin response so caches keyed on the request URL alone do not
// serve the wrong Access-Control-Allow-Origin to a different client. The
// header is emitted on every response from this middleware regardless of
// whether the request itself was cross-origin — a cached non-cross-origin
// response that later gets reused by a cross-origin client would otherwise
// inherit the wrong CORS headers.
type CORSMiddleware struct {
	allowedOrigins  []string
	allowedMethods  []string
	allowedHeaders  []string
	allowAllOrigins bool
}

func newCORSMiddleware(origins []string) *CORSMiddleware {
	m := &CORSMiddleware{
		allowedOrigins: origins,
		allowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		allowedHeaders: []string{"Content-Type", "Authorization", "X-Requested-With"},
	}
	for _, o := range origins {
		if o == "*" {
			m.allowAllOrigins = true
			break
		}
	}
	return m
}

// NewCORSMiddleware creates a new CORS middleware that accepts any origin.
func NewCORSMiddleware() Middleware {
	return newCORSMiddleware([]string{"*"})
}

// NewCORSMiddlewareWithOrigins creates a CORS middleware with specific
// allowed origins. Cross-origin requests from origins not in the list
// receive a response with NO Access-Control-Allow-Origin header — the
// browser then blocks the request. The previous implementation echoed
// `m.allowedOrigins[0]` for disallowed origins, which silently granted
// CORS access to the wrong domain.
func NewCORSMiddlewareWithOrigins(origins []string) Middleware {
	return newCORSMiddleware(origins)
}

// Intercept implements the Middleware interface
func (m *CORSMiddleware) Intercept(next http.Handler) http.Handler {
	methods := strings.Join(m.allowedMethods, ", ")
	headers := strings.Join(m.allowedHeaders, ", ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Vary: Origin so shared caches don't reuse a response cached for
		// one origin against a request from another.
		w.Header().Add("Vary", "Origin")

		origin := r.Header.Get("Origin")
		if origin != "" && m.isAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", methods)
			w.Header().Set("Access-Control-Allow-Headers", headers)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight OPTIONS request — always 204 so the browser
		// proceeds to the real call (or rejects it based on the headers
		// we did or did not set above).
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *CORSMiddleware) isAllowed(origin string) bool {
	if m.allowAllOrigins {
		return true
	}
	for _, allowed := range m.allowedOrigins {
		if allowed == origin {
			return true
		}
	}
	return false
}
