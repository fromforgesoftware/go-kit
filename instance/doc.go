// Package instance provides utilities for creating instances of generic types.
//
// This package helps with dynamic type instantiation, particularly useful
// when working with generics where you need to create new instances at runtime.
//
// # Creating Instances
//
// Use New to create an instance of any type:
//
//	// For pointer types (like proto messages)
//	user := instance.New[*pb.User]()  // Returns allocated *pb.User
//
//	// For value types
//	count := instance.New[int]()  // Returns 0
//	name := instance.New[string]()  // Returns ""
//
// # Use Cases
//
// Proto message instantiation:
//
//	func unmarshalProto[P proto.Message](data []byte) (P, error) {
//	    p := instance.New[P]()  // P is typically *pb.SomeMessage
//	    err := proto.Unmarshal(data, p)
//	    return p, err
//	}
//
// Generic container initialization:
//
//	func createContainer[T any]() *Container[T] {
//	    return &Container[T]{
//	        value: instance.New[T](),
//	    }
//	}
//
// gRPC response handling:
//
//	reply := instance.New[ResponseType]()
//	err := client.Call(ctx, req, reply)
package instance
