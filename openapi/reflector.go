package openapi

import (
	"encoding/json"
	"fmt"

	"github.com/fromforgesoftware/go-kit/openapi/internal/swaggestimpl"
)

// Reflector is the engine that builds an OpenAPI 3.1 spec by reflecting
// Go types provided through OperationContext. Forge owns this interface;
// the default implementation lives behind internal/swaggestimpl.
type Reflector interface {
	// NewOperationContext returns a fresh context for the operation
	// at (method, pattern). Callers fill it via OperationContext
	// methods then pass it back to AddOperation.
	NewOperationContext(method, pattern string) (OperationContext, error)

	// AddOperation registers a fully-described operation on the spec.
	AddOperation(oc OperationContext) error

	// ApplySpecConfig applies spec-level configuration (info, servers,
	// tags, security schemes, default security, external docs).
	ApplySpecConfig(cfg SpecConfig) error

	// SpecJSON returns the assembled spec as JSON bytes.
	SpecJSON() ([]byte, error)
}

// NewReflector returns the default OpenAPI 3.1 reflector — the
// swaggest-backed implementation.
func NewReflector() Reflector {
	return &reflector{r: swaggestimpl.NewReflector()}
}

type reflector struct {
	r swaggestimpl.Reflector
}

func (r *reflector) NewOperationContext(method, pattern string) (OperationContext, error) {
	oc, err := r.r.NewOperationContext(method, pattern)
	if err != nil {
		return nil, err
	}
	return &operationContext{oc: oc}, nil
}

func (r *reflector) AddOperation(oc OperationContext) error {
	wrap, ok := oc.(*operationContext)
	if !ok {
		return fmt.Errorf("openapi: foreign OperationContext implementation %T", oc)
	}
	return r.r.AddOperation(wrap.oc)
}

func (r *reflector) ApplySpecConfig(cfg SpecConfig) error {
	spec := swaggestimpl.Spec(r.r)
	if cfg.Title != "" {
		spec.SetTitle(cfg.Title)
	}
	if cfg.Version != "" {
		spec.SetVersion(cfg.Version)
	}
	if cfg.Description != "" {
		spec.SetDescription(cfg.Description)
	}
	if len(cfg.Servers) > 0 {
		urls := make([]string, len(cfg.Servers))
		descs := make([]string, len(cfg.Servers))
		for i, s := range cfg.Servers {
			urls[i] = s.URL
			descs[i] = s.Description
		}
		swaggestimpl.ApplyServers(spec, urls, descs)
	}
	if len(cfg.Tags) > 0 {
		names := make([]string, len(cfg.Tags))
		descs := make([]string, len(cfg.Tags))
		for i, t := range cfg.Tags {
			names[i] = t.Name
			descs[i] = t.Description
		}
		swaggestimpl.ApplyTags(spec, names, descs)
	}
	if cfg.ExternalDocs != nil {
		swaggestimpl.ApplyExternalDocs(spec, cfg.ExternalDocs.URL, cfg.ExternalDocs.Description)
	}
	for name, scheme := range cfg.SecuritySchemes {
		if err := applySecurityScheme(spec, name, scheme); err != nil {
			return fmt.Errorf("openapi: apply security scheme %q: %w", name, err)
		}
	}
	if len(cfg.DefaultSecurity) > 0 {
		swaggestimpl.ApplyDefaultSecurity(spec, cfg.DefaultSecurity)
	}
	return nil
}

func (r *reflector) SpecJSON() ([]byte, error) {
	return json.MarshalIndent(r.r.SpecSchema(), "", "  ")
}

func applySecurityScheme(spec *swaggestimpl.Spec31, name string, scheme SecurityScheme) error {
	switch s := scheme.(type) {
	case HTTPBearer:
		swaggestimpl.ApplyBearerToken(spec, name, s.BearerFormat)
	case HTTPBasic:
		swaggestimpl.ApplyBasicAuth(spec, name)
	case APIKey:
		swaggestimpl.ApplyAPIKey(spec, name, s.Name, string(s.In))
	case OAuth2:
		switch {
		case s.ClientCredentials != nil:
			swaggestimpl.ApplyOAuth2ClientCredentials(spec, name, s.ClientCredentials.TokenURL, s.ClientCredentials.Scopes)
		case s.AuthorizationCode != nil:
			swaggestimpl.ApplyOAuth2AuthorizationCode(spec, name, s.AuthorizationCode.AuthorizationURL, s.AuthorizationCode.TokenURL, s.AuthorizationCode.Scopes)
		case s.Implicit != nil:
			swaggestimpl.ApplyOAuth2Implicit(spec, name, s.Implicit.AuthorizationURL, s.Implicit.Scopes)
		case s.Password != nil:
			swaggestimpl.ApplyOAuth2Password(spec, name, s.Password.TokenURL, s.Password.Scopes)
		default:
			return fmt.Errorf("oauth2 scheme has no flow configured")
		}
	case OIDC:
		swaggestimpl.ApplyOIDC(spec, name, s.DiscoveryURL)
	case MutualTLS:
		swaggestimpl.ApplyMutualTLS(spec, name)
	default:
		return fmt.Errorf("unknown SecurityScheme type %T", scheme)
	}
	return nil
}
