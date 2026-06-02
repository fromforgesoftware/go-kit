// Package tcp provides a comprehensive TCP server and client framework.
//
// Features:
//   - TCP server and client with functional options
//   - Generic packet handling with custom splitters
//   - Session management
//   - Middleware support with chaining
//   - Controller registration pattern
//   - Graceful shutdown
//   - Connection lifecycle hooks (onConnect, onDisconnect)
//   - Fx dependency injection integration
//
// Basic server usage:
//
//	handler := tcp.HandlerFunc(func(ctx context.Context, sess tcp.Session, data []byte) error {
//	    return sess.Write([]byte("pong"))
//	})
//
//	server, err := tcp.NewServer(monitor,
//	    tcp.WithAddress(":9000"),
//	    tcp.WithHandler(handler),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	server.Start()
//
// With controllers and middleware:
//
//	server, err := tcp.NewServer(monitor,
//	    tcp.WithAddress(":9000"),
//	    tcp.WithControllers(myController),
//	    tcp.WithMiddlewares(loggingMiddleware),
//	)
//
// Client usage:
//
//	client, err := tcp.NewClient("localhost:9000")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	err = client.Write([]byte("ping"))
//	data, err := client.Read()
package tcp
