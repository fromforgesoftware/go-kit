// Package grpc provides a comprehensive gRPC server and client framework.
//
// Features:
//   - Server and client setup with functional options
//   - Middleware/interceptor support with chaining
//   - Health checks built-in
//   - Service reflection for debugging
//   - TLS support
//   - Connection keepalive
//   - Fx dependency injection integration
//
// Basic server usage:
//
//	server, err := grpc.NewServer(monitor,
//	    grpc.WithAddress(":50051"),
//	    grpc.WithControllers(myController),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	server.Start()
//
// With middleware:
//
//	server, err := grpc.NewServer(monitor,
//	    grpc.WithMiddlewares(
//	        middleware.Recovery(logger),
//	        middleware.Logging(logger),
//	        middleware.RequestID(),
//	    ),
//	)
package grpc
