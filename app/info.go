package app

// Info is the service-identity record provided to the fx graph so any
// component (logger, tracer, OpenAPI spec, custom commands) can pull
// the service name and version without re-deriving them.
type Info struct {
	Name    string
	Version string
}
