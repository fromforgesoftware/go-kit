package openapi

// SpecConfig captures spec-level configuration assembled by SpecOpts.
// Consumed by rest.WithOpenAPI to wire the Collector and the spec/UI
// routes on the server.
type SpecConfig struct {
	Title        string
	Version      string
	Description  string
	SpecPath     string
	UIPath       string
	UIRenderer   UIRenderer
	Servers      []SpecServer
	Tags         []SpecTagInfo
	ExternalDocs *ExternalDocsInfo

	SecuritySchemes map[string]SecurityScheme
	DefaultSecurity []string
}

// SpecServer is one entry in the spec's servers array.
type SpecServer struct {
	URL         string
	Description string
}

// SpecTagInfo is one entry in the spec's tags array.
type SpecTagInfo struct {
	Name        string
	Description string
}

// ExternalDocsInfo is the external documentation block attached to
// the spec root.
type ExternalDocsInfo struct {
	URL         string
	Description string
}

// SpecOpt mutates a SpecConfig as it's assembled. rest.WithOpenAPI
// composes a slice of these into the final config.
type SpecOpt func(*SpecConfig)

// SpecTitle sets the spec's info.title.
func SpecTitle(s string) SpecOpt {
	return func(c *SpecConfig) { c.Title = s }
}

// SpecVersion sets the spec's info.version.
func SpecVersion(s string) SpecOpt {
	return func(c *SpecConfig) { c.Version = s }
}

// SpecDescription sets the spec's info.description.
func SpecDescription(s string) SpecOpt {
	return func(c *SpecConfig) { c.Description = s }
}

// SpecPath overrides the default URL the OpenAPI JSON is served at
// (default "/openapi.json").
func SpecPath(p string) SpecOpt {
	return func(c *SpecConfig) { c.SpecPath = p }
}

// UIPath overrides the default URL the UI is served at (default "/docs").
func UIPath(p string) SpecOpt {
	return func(c *SpecConfig) { c.UIPath = p }
}

// UI selects which UI renderer to use (default SwaggerUI).
func UI(r UIRenderer) SpecOpt {
	return func(c *SpecConfig) { c.UIRenderer = r }
}

// Server appends a base URL to the spec's servers array. Call multiple
// times to declare multiple environments.
func Server(url, description string) SpecOpt {
	return func(c *SpecConfig) {
		c.Servers = append(c.Servers, SpecServer{URL: url, Description: description})
	}
}

// SpecTag declares a top-level tag with a description, used by UIs to
// group and describe sets of operations.
func SpecTag(name, description string) SpecOpt {
	return func(c *SpecConfig) {
		c.Tags = append(c.Tags, SpecTagInfo{Name: name, Description: description})
	}
}

// SpecExternalDocs attaches a single external-docs block to the spec root.
func SpecExternalDocs(url, description string) SpecOpt {
	return func(c *SpecConfig) {
		c.ExternalDocs = &ExternalDocsInfo{URL: url, Description: description}
	}
}

// SpecSecurityScheme registers a named security scheme under
// components.securitySchemes. Reference the same name from operation
// Security(...) opts or from DefaultSecurity.
func SpecSecurityScheme(name string, scheme SecurityScheme) SpecOpt {
	return func(c *SpecConfig) {
		if c.SecuritySchemes == nil {
			c.SecuritySchemes = map[string]SecurityScheme{}
		}
		c.SecuritySchemes[name] = scheme
	}
}

// DefaultSecurity sets the root-level security requirement applied to
// every operation that doesn't override it via Security(...) or
// NoSecurity().
func DefaultSecurity(schemeNames ...string) SpecOpt {
	return func(c *SpecConfig) {
		c.DefaultSecurity = append([]string(nil), schemeNames...)
	}
}

// ApplyDefaults fills empty fields with sensible defaults. Called by
// the rest package before constructing the Collector.
func (c *SpecConfig) ApplyDefaults() {
	if c.Title == "" {
		c.Title = "API"
	}
	if c.Version == "" {
		c.Version = "0.0.0"
	}
	if c.SpecPath == "" {
		c.SpecPath = "/openapi.json"
	}
	if c.UIPath == "" {
		c.UIPath = "/docs"
	}
	if c.UIRenderer == "" {
		c.UIRenderer = SwaggerUI
	}
}
