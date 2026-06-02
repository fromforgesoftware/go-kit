// Package websocket provides a comprehensive WebSocket client framework.
//
// Features:
//   - WebSocket client with functional options
//   - Automatic reconnection with configurable retry
//   - Message acknowledgment (ACK/NACK)
//   - Heartbeat/ping mechanism
//   - Event-driven architecture with callbacks
//   - Connection lifecycle management
//   - Token-based authentication
//   - Concurrent read/write channels
//   - Graceful shutdown
//
// Basic client usage:
//
//	tokenProvider := &MyTokenProvider{}
//
//	client := websocket.NewClient(tokenProvider, logger,
//	    websocket.WithReconnect(true),
//	    websocket.WithDialTimeout(10*time.Second),
//	    websocket.WithEventCallback(func(event websocket.Event, msg string) {
//	        log.Printf("Event: %s, Message: %s", event, msg)
//	    }),
//	)
//
//	err := client.Start()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Stop()
//
// Sending and receiving messages:
//
//	// Send
//	msg := &websocket.Message{
//	    ID:   "123",
//	    Type: websocket.MessageTypeSubscribe,
//	    Data: subscriptionData,
//	}
//	errChan := client.Write(ctx, msg)
//	if err := <-errChan; err != nil {
//	    log.Printf("Write error: %v", err)
//	}
//
//	// Receive
//	for msg := range client.Read() {
//	    log.Printf("Received: %+v", msg)
//	}
package websocket
