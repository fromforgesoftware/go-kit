// Package oidc is a minimal OIDC *client*: discovery, an authorization-code +
// PKCE redirect, code→token exchange, and ID-token verification. It is the
// shared client used when a forge service signs users in via an external IdP
// (or another forge service acting as the IdP).
package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
)

// Provider is the static config for one IdP.
type Provider struct {
	Issuer       string // base issuer URL; discovery is issuer + /.well-known/openid-configuration
	ClientID     string
	ClientSecret string // empty for a public client (PKCE only)
	Scopes       []string
}

// Tokens is the token endpoint response we care about.
type Tokens struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

// Claims is the identity extracted from a verified ID token.
type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// HTTPDoer is the request surface (so tests can stub it).
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type discovery struct {
	Issuer       string `json:"issuer"`
	AuthorizeURL string `json:"authorization_endpoint"`
	TokenURL     string `json:"token_endpoint"`
	JWKSURI      string `json:"jwks_uri"`
	fetchedAt    time.Time
}

// Client is an OIDC relying-party for one Provider, caching discovery + JWKS.
type Client struct {
	p    Provider
	http HTTPDoer

	mu   sync.Mutex
	disc *discovery
}

func NewClient(p Provider, h HTTPDoer) *Client {
	if h == nil {
		h = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{p: p, http: h}
}

// --- PKCE ---

func base64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// NewVerifier returns a high-entropy PKCE code_verifier.
func NewVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64url(b), nil
}

// Challenge is the S256 code_challenge for a verifier.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64url(sum[:])
}

// RandomState returns an opaque CSRF state value.
func RandomState() (string, error) { return NewVerifier() }

// --- flow ---

// AuthCodeURL builds the authorization-code + PKCE redirect to the IdP.
func (c *Client) AuthCodeURL(ctx context.Context, redirectURI, state, codeChallenge string) (string, error) {
	d, err := c.discovery(ctx)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(d.AuthorizeURL)
	if err != nil {
		return "", err
	}
	scopes := c.p.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "email", "profile"}
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.p.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", strings.Join(scopes, " "))
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Exchange swaps an authorization code for tokens (PKCE; public or confidential).
func (c *Client) Exchange(ctx context.Context, redirectURI, code, codeVerifier string) (Tokens, error) {
	d, err := c.discovery(ctx)
	if err != nil {
		return Tokens{}, err
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {c.p.ClientID},
		"code_verifier": {codeVerifier},
	}
	if c.p.ClientSecret != "" {
		form.Set("client_secret", c.p.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return Tokens{}, fmt.Errorf("token endpoint unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Tokens{}, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}
	var t Tokens
	if err := json.Unmarshal(body, &t); err != nil {
		return Tokens{}, fmt.Errorf("token response not JSON: %w", err)
	}
	return t, nil
}

// VerifyIDToken validates the ID token's signature against the IdP's JWKS and
// its issuer/audience, returning the identity claims.
func (c *Client) VerifyIDToken(ctx context.Context, rawIDToken string) (Claims, error) {
	d, err := c.discovery(ctx)
	if err != nil {
		return Claims{}, err
	}
	keys, err := c.fetchJWKS(ctx, d.JWKSURI)
	if err != nil {
		return Claims{}, err
	}
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(rawIDToken, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		for _, k := range keys.Keys {
			if k.KeyID == kid {
				return k.Key, nil
			}
		}
		return nil, fmt.Errorf("no JWKS key matches kid %q", kid)
	}, jwt.WithValidMethods([]string{"RS256", "ES256"})); err != nil {
		return Claims{}, fmt.Errorf("invalid ID token: %w", err)
	}
	if iss, _ := claims["iss"].(string); iss != d.Issuer {
		return Claims{}, fmt.Errorf("issuer mismatch: %q != %q", iss, d.Issuer)
	}
	if !audienceContains(claims["aud"], c.p.ClientID) {
		return Claims{}, fmt.Errorf("audience mismatch")
	}
	out := Claims{}
	if v, ok := claims["sub"].(string); ok {
		out.Subject = v
	}
	if v, ok := claims["email"].(string); ok {
		out.Email = v
	}
	if v, ok := claims["email_verified"].(bool); ok {
		out.EmailVerified = v
	}
	if v, ok := claims["name"].(string); ok {
		out.Name = v
	}
	return out, nil
}

func (c *Client) discovery(ctx context.Context) (*discovery, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.disc != nil && time.Since(c.disc.fetchedAt) < time.Hour {
		return c.disc, nil
	}
	var d discovery
	u := strings.TrimRight(c.p.Issuer, "/") + "/.well-known/openid-configuration"
	if err := c.fetchJSON(ctx, u, &d); err != nil {
		return nil, err
	}
	if d.Issuer == "" || d.AuthorizeURL == "" || d.TokenURL == "" || d.JWKSURI == "" {
		return nil, fmt.Errorf("incomplete OIDC discovery document at %s", u)
	}
	d.fetchedAt = time.Now()
	c.disc = &d
	return &d, nil
}

func (c *Client) fetchJWKS(ctx context.Context, jwksURI string) (*jose.JSONWebKeySet, error) {
	var set jose.JSONWebKeySet
	if err := c.fetchJSON(ctx, jwksURI, &set); err != nil {
		return nil, err
	}
	return &set, nil
}

func (c *Client) fetchJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", u, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("GET %s returned %d", u, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func audienceContains(aud any, want string) bool {
	switch v := aud.(type) {
	case string:
		return v == want
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok && s == want {
				return true
			}
		}
	case []string:
		for _, s := range v {
			if s == want {
				return true
			}
		}
	}
	return false
}
