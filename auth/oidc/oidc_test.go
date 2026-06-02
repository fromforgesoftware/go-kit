package oidc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/auth/oidc"
)

// idpStub serves the OIDC discovery + token endpoints.
func idpStub(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var base string
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 base,
			"authorization_endpoint": base + "/authorize",
			"token_endpoint":         base + "/token",
			"jwks_uri":               base + "/jwks",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") != "authorization_code" || r.PostForm.Get("code_verifier") == "" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "at", "id_token": "it", "token_type": "Bearer"})
	})
	ts := httptest.NewServer(mux)
	base = ts.URL
	t.Cleanup(ts.Close)
	return ts
}

func TestPKCE(t *testing.T) {
	v, err := oidc.NewVerifier()
	require.NoError(t, err)
	require.NotEmpty(t, v)
	require.NotEqual(t, v, oidc.Challenge(v))
}

func TestAuthCodeURL(t *testing.T) {
	ts := idpStub(t)
	c := oidc.NewClient(oidc.Provider{Issuer: ts.URL, ClientID: "cid", Scopes: []string{"openid", "email"}}, nil)

	raw, err := c.AuthCodeURL(context.Background(), "https://app/cb", "state-1", "chal-1")
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, ts.URL+"/authorize", u.Scheme+"://"+u.Host+u.Path)
	q := u.Query()
	require.Equal(t, "code", q.Get("response_type"))
	require.Equal(t, "cid", q.Get("client_id"))
	require.Equal(t, "https://app/cb", q.Get("redirect_uri"))
	require.Equal(t, "openid email", q.Get("scope"))
	require.Equal(t, "state-1", q.Get("state"))
	require.Equal(t, "chal-1", q.Get("code_challenge"))
	require.Equal(t, "S256", q.Get("code_challenge_method"))
}

func TestExchange(t *testing.T) {
	ts := idpStub(t)
	c := oidc.NewClient(oidc.Provider{Issuer: ts.URL, ClientID: "cid"}, nil)

	toks, err := c.Exchange(context.Background(), "https://app/cb", "code-1", "verifier-1")
	require.NoError(t, err)
	require.Equal(t, "it", toks.IDToken)
	require.Equal(t, "at", toks.AccessToken)
}
