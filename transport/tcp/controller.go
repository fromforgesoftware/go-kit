package tcp

import (
	"context"

	"go.uber.org/fx"
)

// Registry defines the interface for registering TCP handlers, keyed by
// a uint16 opcode. Matches udp.Registry so a service's controller code
// is identical across the two transports.
type Registry interface {
	Register(opcode uint16, handler Handler)
	RegisterFunc(opcode uint16, handler func(context.Context, Session, []byte) error)
}

// Controller defines a component that registers routes to a TCP Registry (e.g. Mux)
type Controller interface {
	Register(Registry)
}

// ControllerFunc is an adapter to allow the use of ordinary functions as Controllers
type ControllerFunc func(Registry)

func (f ControllerFunc) Register(r Registry) {
	f(r)
}

// NewFxController registers a controller in the Fx dependency graph.
// The controller will be automatically picked up by FxModule.
//
// Example:
//
//	tcp.NewFxController(func() tcp.Controller {
//	    return myController
//	})
func NewFxController(ctrl any) fx.Option {
	return fx.Provide(
		fx.Annotate(
			ctrl,
			fx.ResultTags(`group:"tcpControllers"`),
			fx.As(new(Controller)),
		),
	)
}
