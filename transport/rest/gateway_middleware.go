package rest

import (
	"net/http"
	"os"
	"strings"

	"github.com/fromforgesoftware/go-kit/auth/jwt"
)

const bearerPrefix = "Bearer "

// GatewayMiddleware optionally gates a service's /api/* admin surface so only a
// caller holding the shared FORGE_GATEWAY_SECRET (the Foundry gateway) can reach
// it. When the secret is unset it is a no-op. Non-/api paths (OAuth/OIDC/hosted
// pages, health) are never gated — those stay public.
type GatewayMiddleware struct {
	validator jwt.Validator
}

func NewGatewayMiddleware() (*GatewayMiddleware, error) {
	secret := os.Getenv("FORGE_GATEWAY_SECRET")
	if secret == "" {
		return &GatewayMiddleware{}, nil
	}
	v, err := jwt.NewHMACIssuer(secret)
	if err != nil {
		return nil, err
	}
	return &GatewayMiddleware{validator: v}, nil
}

func (m *GatewayMiddleware) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.validator == nil || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, bearerPrefix) {
			http.Error(w, `{"error":"gateway token required"}`, http.StatusUnauthorized)
			return
		}
		if _, err := m.validator.Validate(r.Context(), strings.TrimPrefix(auth, bearerPrefix)); err != nil {
			http.Error(w, `{"error":"invalid gateway token"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
