package openapi

// SecurityScheme is the forge representation of an OpenAPI 3.1 security
// scheme. Each implementation describes one of the schemes the spec
// supports (http+bearer, apiKey, oauth2, openIdConnect, mutualTLS).
//
// The sealed marker method keeps consumers from defining their own —
// the bridge in internal/swaggestimpl needs to know every concrete type
// to translate it into the underlying spec.
type SecurityScheme interface {
	isSecurityScheme()
}

// HTTPBearer represents an HTTP bearer-token scheme (most common
// for JWT-based auth). BearerFormat is informational, e.g. "JWT".
type HTTPBearer struct {
	BearerFormat string
}

func (HTTPBearer) isSecurityScheme() {}

// BearerJWT is shorthand for HTTPBearer{BearerFormat: "JWT"}.
func BearerJWT() SecurityScheme {
	return HTTPBearer{BearerFormat: "JWT"}
}

// BearerToken returns an HTTP bearer scheme with the given format.
func BearerToken(format string) SecurityScheme {
	return HTTPBearer{BearerFormat: format}
}

// HTTPBasic represents HTTP basic authentication.
type HTTPBasic struct{}

func (HTTPBasic) isSecurityScheme() {}

// BasicAuth returns a Basic HTTP scheme.
func BasicAuth() SecurityScheme { return HTTPBasic{} }

// APIKeyIn identifies where an API key is presented.
type APIKeyIn string

const (
	APIKeyInHeader APIKeyIn = "header"
	APIKeyInQuery  APIKeyIn = "query"
	APIKeyInCookie APIKeyIn = "cookie"
)

// APIKey is the apiKey scheme.
type APIKey struct {
	Name string
	In   APIKeyIn
}

func (APIKey) isSecurityScheme() {}

// APIKeyHeader is shorthand for an apiKey scheme presented in the
// named HTTP header.
func APIKeyHeader(headerName string) SecurityScheme {
	return APIKey{Name: headerName, In: APIKeyInHeader}
}

// APIKeyQuery is shorthand for an apiKey scheme presented in the
// named query parameter.
func APIKeyQuery(paramName string) SecurityScheme {
	return APIKey{Name: paramName, In: APIKeyInQuery}
}

// APIKeyCookie is shorthand for an apiKey scheme presented in the
// named cookie.
func APIKeyCookie(cookieName string) SecurityScheme {
	return APIKey{Name: cookieName, In: APIKeyInCookie}
}

// OAuth2 represents the oauth2 scheme.
//
// Exactly one of Flow* fields should be populated per scheme. Callers
// should usually use the OAuth2* builder helpers rather than
// constructing this struct directly.
type OAuth2 struct {
	ClientCredentials *OAuth2ClientCredentialsFlow
	AuthorizationCode *OAuth2AuthorizationCodeFlow
	Implicit          *OAuth2ImplicitFlow
	Password          *OAuth2PasswordFlow
}

func (OAuth2) isSecurityScheme() {}

// OAuth2ClientCredentialsFlow describes the client-credentials flow.
type OAuth2ClientCredentialsFlow struct {
	TokenURL string
	Scopes   map[string]string
}

// OAuth2AuthorizationCodeFlow describes the authorization-code flow.
type OAuth2AuthorizationCodeFlow struct {
	AuthorizationURL string
	TokenURL         string
	Scopes           map[string]string
}

// OAuth2ImplicitFlow describes the (deprecated) implicit flow.
type OAuth2ImplicitFlow struct {
	AuthorizationURL string
	Scopes           map[string]string
}

// OAuth2PasswordFlow describes the (deprecated) password flow.
type OAuth2PasswordFlow struct {
	TokenURL string
	Scopes   map[string]string
}

// OAuth2ClientCredentials is shorthand for the client-credentials flow.
func OAuth2ClientCredentials(tokenURL string, scopes map[string]string) SecurityScheme {
	return OAuth2{ClientCredentials: &OAuth2ClientCredentialsFlow{TokenURL: tokenURL, Scopes: scopes}}
}

// OAuth2AuthorizationCode is shorthand for the authorization-code flow.
func OAuth2AuthorizationCode(authURL, tokenURL string, scopes map[string]string) SecurityScheme {
	return OAuth2{AuthorizationCode: &OAuth2AuthorizationCodeFlow{AuthorizationURL: authURL, TokenURL: tokenURL, Scopes: scopes}}
}

// OAuth2Implicit is shorthand for the implicit flow (deprecated by
// the OAuth 2.0 BCP but still occasionally encountered).
func OAuth2Implicit(authURL string, scopes map[string]string) SecurityScheme {
	return OAuth2{Implicit: &OAuth2ImplicitFlow{AuthorizationURL: authURL, Scopes: scopes}}
}

// OAuth2Password is shorthand for the resource-owner-password flow
// (deprecated by the OAuth 2.0 BCP).
func OAuth2Password(tokenURL string, scopes map[string]string) SecurityScheme {
	return OAuth2{Password: &OAuth2PasswordFlow{TokenURL: tokenURL, Scopes: scopes}}
}

// OIDC represents the openIdConnect scheme. The discovery URL points
// at the issuer's well-known configuration endpoint.
type OIDC struct {
	DiscoveryURL string
}

func (OIDC) isSecurityScheme() {}

// OIDCScheme returns an openIdConnect scheme. (Named OIDCScheme rather
// than OIDC to avoid colliding with the OIDC struct type.)
func OIDCScheme(discoveryURL string) SecurityScheme {
	return OIDC{DiscoveryURL: discoveryURL}
}

// MutualTLS represents the mutualTLS scheme.
type MutualTLS struct{}

func (MutualTLS) isSecurityScheme() {}

// MutualTLSScheme returns a mutualTLS scheme.
func MutualTLSScheme() SecurityScheme { return MutualTLS{} }
