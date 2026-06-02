package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"time"
)

// Cached is the stored response for a previously-completed request.
type Cached struct {
	Status      int
	Body        []byte
	RequestHash string
	StoredAt    time.Time
}

// Store persists idempotency records. Implementations must be
// goroutine-safe.
type Store interface {
	// Reserve atomically claims `key` for the supplied request hash.
	//
	// Returns (existing, false, nil) when a record already exists for
	// `key` — the caller serves `existing.Body` directly.
	//
	// Returns (nil, true, nil) when the key is fresh — the caller
	// invokes the handler, then calls Commit to store the response.
	//
	// Errors are transport / store failures; the caller should bubble
	// them as 5xx.
	Reserve(ctx context.Context, key string, requestHash string, ttl time.Duration) (existing *Cached, fresh bool, err error)

	// Commit writes the final response under `key`. Idempotent — a
	// second Commit for the same key with a matching hash is a no-op.
	Commit(ctx context.Context, key string, requestHash string, response Cached) error

	// Release drops a reservation without committing a response. Used
	// when the handler fails in a way that should not be cached (5xx,
	// panic, transport hangup) — subsequent retries should be free to
	// run the handler again. Idempotent.
	Release(ctx context.Context, key string) error
}

// Errors surfaced by the middleware.
var (
	ErrHashMismatch = errors.New("idempotency: request hash differs for cached key")
	ErrWaitTimeout  = errors.New("idempotency: timed out waiting for in-flight key")
)

// Config configures the RESTMiddleware.
type Config struct {
	Store Store
	// DefaultTTL is how long records live. Default 24h.
	DefaultTTL time.Duration
	// HeaderName is the inbound header containing the idempotency key.
	// Default "Idempotency-Key". Requests without the header bypass
	// the middleware entirely (treated as non-idempotent).
	HeaderName string
	// RequestHash derives the request hash. Default: SHA-256 over
	// method + path + body bytes.
	RequestHash func(*http.Request) (string, error)
	// WaitTimeout caps how long a concurrent same-key request blocks
	// on the in-flight reservation. Default 30s.
	WaitTimeout time.Duration
}

// RESTMiddleware returns an HTTP middleware that enforces idempotency
// on requests carrying the configured header. Requests without the
// header pass through unchanged.
func RESTMiddleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.Store == nil {
		panic("idempotency: Config.Store is required")
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = 24 * time.Hour
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "Idempotency-Key"
	}
	if cfg.WaitTimeout <= 0 {
		cfg.WaitTimeout = 30 * time.Second
	}
	if cfg.RequestHash == nil {
		cfg.RequestHash = defaultRequestHash
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(cfg.HeaderName)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			hash, err := cfg.RequestHash(r)
			if err != nil {
				http.Error(w, "idempotency: hashing request: "+err.Error(), http.StatusBadRequest)
				return
			}

			existing, fresh, err := cfg.Store.Reserve(r.Context(), key, hash, cfg.DefaultTTL)
			if err != nil {
				http.Error(w, "idempotency: reserve: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if !fresh && existing != nil {
				if existing.RequestHash != hash {
					http.Error(w, ErrHashMismatch.Error(), http.StatusConflict)
					return
				}
				replay(w, *existing)
				return
			}

			// Fresh reservation — run the handler with a capture writer
			// so we can store the response on success.
			cap := &captureWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(cap, r)

			// Commit even on non-2xx so a deliberate 4xx is also
			// idempotent. For 5xx, drop the reservation so retries
			// re-run the handler instead of getting the cached error.
			if cap.status >= 500 {
				_ = cfg.Store.Release(r.Context(), key)
				return
			}
			_ = cfg.Store.Commit(r.Context(), key, hash, Cached{
				Status:      cap.status,
				Body:        cap.body,
				RequestHash: hash,
				StoredAt:    time.Now(),
			})
		})
	}
}

func replay(w http.ResponseWriter, c Cached) {
	w.Header().Set("Idempotency-Replay", "true")
	w.WriteHeader(c.Status)
	_, _ = w.Write(c.Body)
}

func defaultRequestHash(r *http.Request) (string, error) {
	h := sha256.New()
	_, _ = io.WriteString(h, r.Method)
	_, _ = io.WriteString(h, "\n")
	_, _ = io.WriteString(h, r.URL.Path)
	_, _ = io.WriteString(h, "\n")
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return "", err
		}
		_, _ = h.Write(body)
		// Rebuild the body for the downstream handler.
		r.Body = io.NopCloser(byteReader(body))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// byteReader returns an io.Reader over a byte slice without
// allocating a bytes.Reader header on every call.
type byteReader []byte

func (b byteReader) Read(p []byte) (int, error) {
	if len(b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, b)
	return n, nil
}

// captureWriter records status + body so the middleware can replay
// the response on retried requests.
type captureWriter struct {
	http.ResponseWriter
	status      int
	body        []byte
	wroteHeader bool
}

func (c *captureWriter) WriteHeader(status int) {
	if c.wroteHeader {
		return
	}
	c.status = status
	c.wroteHeader = true
	c.ResponseWriter.WriteHeader(status)
}

func (c *captureWriter) Write(p []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	c.body = append(c.body, p...)
	return c.ResponseWriter.Write(p)
}
