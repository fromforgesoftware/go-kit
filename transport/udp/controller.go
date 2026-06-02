package udp

// Controller defines a UDP controller that registers handlers
type Controller interface {
	Register(Registry)
}
