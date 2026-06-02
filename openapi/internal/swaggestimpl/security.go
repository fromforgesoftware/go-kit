package swaggestimpl

import (
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"
)

// ApplyBearerJWT writes an HTTP-bearer scheme with BearerFormat=JWT
// to components.securitySchemes[name].
func ApplyBearerJWT(spec *Spec31, name string) {
	ApplyBearerToken(spec, name, "JWT")
}

// ApplyBearerToken writes an HTTP-bearer scheme with the given
// bearer format. Format is informational (e.g. "JWT", "opaque").
func ApplyBearerToken(spec *Spec31, name, bearerFormat string) {
	spec.SetHTTPBearerTokenSecurity(name, bearerFormat, "")
}

// ApplyBasicAuth writes an HTTP-basic scheme.
func ApplyBasicAuth(spec *Spec31, name string) {
	spec.SetHTTPBasicSecurity(name, "")
}

// ApplyAPIKey writes an apiKey scheme.
func ApplyAPIKey(spec *Spec31, name, fieldName, in string) {
	spec.SetAPIKeySecurity(name, fieldName, openapi.In(in), "")
}

// ApplyOAuth2ClientCredentials writes an OAuth2 scheme with the
// client-credentials flow.
func ApplyOAuth2ClientCredentials(spec *Spec31, name, tokenURL string, scopes map[string]string) {
	flow := openapi31.OauthFlowsDefsClientCredentials{TokenURL: tokenURL}
	if scopes != nil {
		flow.Scopes = scopes
	}
	flows := openapi31.OauthFlows{}
	flows.WithClientCredentials(flow)
	applyOAuth2(spec, name, flows)
}

// ApplyOAuth2AuthorizationCode writes an OAuth2 scheme with the
// authorization-code flow.
func ApplyOAuth2AuthorizationCode(spec *Spec31, name, authURL, tokenURL string, scopes map[string]string) {
	flow := openapi31.OauthFlowsDefsAuthorizationCode{AuthorizationURL: authURL, TokenURL: tokenURL}
	if scopes != nil {
		flow.Scopes = scopes
	}
	flows := openapi31.OauthFlows{}
	flows.WithAuthorizationCode(flow)
	applyOAuth2(spec, name, flows)
}

// ApplyOAuth2Implicit writes an OAuth2 scheme with the implicit flow.
func ApplyOAuth2Implicit(spec *Spec31, name, authURL string, scopes map[string]string) {
	flow := openapi31.OauthFlowsDefsImplicit{AuthorizationURL: authURL}
	if scopes != nil {
		flow.Scopes = scopes
	}
	flows := openapi31.OauthFlows{}
	flows.WithImplicit(flow)
	applyOAuth2(spec, name, flows)
}

// ApplyOAuth2Password writes an OAuth2 scheme with the password flow.
func ApplyOAuth2Password(spec *Spec31, name, tokenURL string, scopes map[string]string) {
	flow := openapi31.OauthFlowsDefsPassword{TokenURL: tokenURL}
	if scopes != nil {
		flow.Scopes = scopes
	}
	flows := openapi31.OauthFlows{}
	flows.WithPassword(flow)
	applyOAuth2(spec, name, flows)
}

func applyOAuth2(spec *Spec31, name string, flows openapi31.OauthFlows) {
	oauth := openapi31.SecuritySchemeOauth2{}
	oauth.WithFlows(flows)
	scheme := openapi31.SecurityScheme{}
	scheme.WithOauth2(oauth)
	spec.ComponentsEns().WithSecuritySchemesItem(name, openapi31.SecuritySchemeOrReference{SecurityScheme: &scheme})
}

// ApplyOIDC writes an openIdConnect scheme.
func ApplyOIDC(spec *Spec31, name, discoveryURL string) {
	oidc := openapi31.SecuritySchemeOidc{}
	oidc.WithOpenIDConnectURL(discoveryURL)
	scheme := openapi31.SecurityScheme{}
	scheme.WithOidc(oidc)
	spec.ComponentsEns().WithSecuritySchemesItem(name, openapi31.SecuritySchemeOrReference{SecurityScheme: &scheme})
}

// ApplyMutualTLS writes a mutualTLS scheme.
func ApplyMutualTLS(spec *Spec31, name string) {
	scheme := openapi31.SecurityScheme{}
	scheme.WithMutualTLS(openapi31.MutualTLS{})
	spec.ComponentsEns().WithSecuritySchemesItem(name, openapi31.SecuritySchemeOrReference{SecurityScheme: &scheme})
}

// ApplyServers writes the servers array.
func ApplyServers(spec *Spec31, urls []string, descs []string) {
	servers := make([]openapi31.Server, 0, len(urls))
	for i, u := range urls {
		s := openapi31.Server{URL: u}
		if i < len(descs) && descs[i] != "" {
			d := descs[i]
			s.Description = &d
		}
		servers = append(servers, s)
	}
	spec.WithServers(servers...)
}

// ApplyTags writes the tags array.
func ApplyTags(spec *Spec31, names, descriptions []string) {
	tags := make([]openapi31.Tag, 0, len(names))
	for i, n := range names {
		t := openapi31.Tag{Name: n}
		if i < len(descriptions) && descriptions[i] != "" {
			d := descriptions[i]
			t.Description = &d
		}
		tags = append(tags, t)
	}
	spec.WithTags(tags...)
}

// ApplyExternalDocs sets the spec-level externalDocs block.
func ApplyExternalDocs(spec *Spec31, url, description string) {
	ed := spec.ExternalDocsEns()
	ed.URL = url
	if description != "" {
		d := description
		ed.Description = &d
	}
}

// ApplyDefaultSecurity sets the root-level security requirement.
func ApplyDefaultSecurity(spec *Spec31, schemeNames []string) {
	if len(schemeNames) == 0 {
		return
	}
	req := map[string][]string{}
	for _, name := range schemeNames {
		req[name] = []string{}
	}
	spec.WithSecurity(req)
}
