package ratelimit

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fromforgesoftware/go-kit/auth"
)

// KeyFunc derives the rate-limit bucket key from a request.
type KeyFunc func(*http.Request) string

// PolicyFunc resolves the policy for a request (e.g. by tier/route).
type PolicyFunc func(*http.Request) Policy

// StaticPolicy always yields p.
func StaticPolicy(p Policy) PolicyFunc { return func(*http.Request) Policy { return p } }

// ByOrg keys on the active organization claim, falling back to client IP.
func ByOrg() KeyFunc {
	return func(r *http.Request) string {
		if org, ok := auth.OrgIDFromCtx(r.Context()); ok {
			return "org:" + org
		}
		return "ip:" + clientIP(r)
	}
}

// ByIP keys on the client IP.
func ByIP() KeyFunc { return func(r *http.Request) string { return "ip:" + clientIP(r) } }

// HTTPMiddleware enforces a Limiter on an HTTP handler chain.
type HTTPMiddleware struct {
	limiter Limiter
	key     KeyFunc
	policy  PolicyFunc
}

// NewHTTPMiddleware builds the middleware. It implements the kit rest
// Middleware interface (Intercept).
func NewHTTPMiddleware(l Limiter, key KeyFunc, policy PolicyFunc) *HTTPMiddleware {
	return &HTTPMiddleware{limiter: l, key: key, policy: policy}
}

func (m *HTTPMiddleware) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res, err := m.limiter.Allow(r.Context(), m.key(r), m.policy(r))
		if err != nil {
			http.Error(w, "rate limiter unavailable", http.StatusInternalServerError)
			return
		}
		writeRateLimitHeaders(w, res)
		if !res.Allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(res.RetryAfter)))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeRateLimitHeaders(w http.ResponseWriter, res Result) {
	h := w.Header()
	h.Set("RateLimit-Limit", strconv.Itoa(res.Limit))
	h.Set("RateLimit-Remaining", strconv.Itoa(res.Remaining))
	h.Set("RateLimit-Reset", strconv.Itoa(ceilSeconds(res.ResetAfter)))
}

func retryAfterSeconds(d time.Duration) int {
	if s := ceilSeconds(d); s > 0 {
		return s
	}
	return 1
}

func ceilSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int((d + time.Second - 1) / time.Second)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
