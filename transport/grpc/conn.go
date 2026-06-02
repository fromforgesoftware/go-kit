package grpc

import "google.golang.org/grpc"

// Conn is a transparent type alias for *grpc.ClientConn — methods on
// *grpc.ClientConn are reachable through the alias. The previous form
// (`type Conn *grpc.ClientConn`) was a defined type that blocked method
// dispatch on the alias, which is a well-known Go anti-pattern.
type Conn = *grpc.ClientConn

// ServiceDesc is a transparent type alias for *grpc.ServiceDesc. The
// previous form (`type ServiceDesc *grpc.ServiceDesc`) was a defined
// type that blocked method dispatch and assignment compatibility with
// values produced by protoc-gen-go-grpc.
type ServiceDesc = *grpc.ServiceDesc

// dial creates a client connection to the given target.
func dial(target string, opts ...grpc.DialOption) (Conn, error) {
	c, err := grpc.NewClient(target, opts...)
	if err != nil {
		return c, err
	}
	return c, nil
}
