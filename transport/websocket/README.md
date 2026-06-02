# WebSocket Client Package

A comprehensive, thread-safe WebSocket client implementation for Go applications, designed for real-time communication with automatic reconnection, message acknowledgment, and event-driven architecture.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [API Reference](#api-reference)
- [Configuration Options](#configuration-options)
- [Event System](#event-system)
- [Message Types](#message-types)
- [Examples](#examples)
- [Best Practices](#best-practices)
- [Testing](#testing)
- [Contributing](#contributing)

## Features

### Core Features

- ✅ **Thread-safe operations** with atomic operations and proper synchronization
- ✅ **Automatic reconnection** with configurable retry attempts and intervals
- ✅ **Message acknowledgment system** for reliable communication
- ✅ **Heartbeat/ping mechanism** to keep connections alive
- ✅ **Event-driven architecture** with customizable callbacks
- ✅ **Token-based authentication** support
- ✅ **Configurable buffer sizes** and timeouts
- ✅ **Connection state management** with proper lifecycle handling

### Advanced Features

- ✅ **Context-based operations** for cancellation and timeouts
- ✅ **Metrics and monitoring** integration
- ✅ **Graceful shutdown** handling
- ✅ **Buffer overflow protection** with event notifications
- ✅ **Race condition free** implementation verified with `go test -race`

## Installation

```bash
go get github.com/domi-unimedia/trading-bot/shared/go-kit/transport/websocket
```

## Quick Start

### 1. Implement Token Provider

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/domi-unimedia/trading-bot/shared/go-kit/transport/websocket"
)

// MyTokenProvider implements the TokenProvider interface
type MyTokenProvider struct {
    endpoint string
    apiKey   string
    secret   string
}

func (tp *MyTokenProvider) GetToken() ([]*websocket.Token, error) {
    // Implement your token retrieval logic here
    // This typically involves making an HTTP request to your API
    token := &websocket.Token{
        Token:        "your-websocket-token",
        PingInterval: 20000, // 20 seconds
        Endpoint:     tp.endpoint,
        Protocol:     "websocket",
        Encrypt:      true,
        PingTimeout:  10000, // 10 seconds
    }

    return []*websocket.Token{token}, nil
}

func (tp *MyTokenProvider) Close() error {
    // Cleanup resources if needed
    return nil
}
```

### 2. Create and Configure WebSocket Client

```go
func main() {
    // Create token provider
    tokenProvider := &MyTokenProvider{
        endpoint: "wss://api.example.com/websocket",
        apiKey:   "your-api-key",
        secret:   "your-api-secret",
    }

    // Create logger (implement monitoring.Logger interface)
    logger := NewLogger() // Your logger implementation

    // Configure WebSocket client options
    options := websocket.NewClientOptionBuilder().
        WithReconnect(true).
        WithReconnectAttempts(5).
        WithReconnectInterval(5 * time.Second).
        WithDialTimeout(10 * time.Second).
        WithReadMessageBuffer(2048).
        WithWriteMessageBuffer(512).
        WithEventCallback(func(event websocket.Event, msg string) {
            log.Printf("WebSocket event: %s - %s", event, msg)
        }).
        Build()

    // Create the WebSocket client
    client := websocket.NewWebSocketClient(tokenProvider, logger, options)

    // Start the connection
    if err := client.Start(); err != nil {
        log.Fatal("Failed to start WebSocket client:", err)
    }
    defer client.Stop()

    // Listen for incoming messages
    go func() {
        for message := range client.Read() {
            log.Printf("Received message: %+v", message)
            // Process your message here
        }
    }()

    // Send a subscription message
    msg := &websocket.Message{
        ID:      "sub-001",
        Type:    websocket.MessageTypeSubscribe,
        Topic:   "/market/ticker:BTC-USDT",
        Subject: "ticker",
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    errChan := client.Write(ctx, msg)
    if err := <-errChan; err != nil {
        log.Printf("Failed to send message: %v", err)
    }

    // Keep the application running
    select {}
}
```

## API Reference

### Client Interface

```go
type Client interface {
    Start() error
    Stop() error
    Write(context.Context, *Message) <-chan error
    Read() <-chan *Message
    Reconnected() <-chan struct{}
}
```

### TokenProvider Interface

```go
type TokenProvider interface {
    GetToken() ([]*Token, error)
    Close() error
}
```

### WebSocketMessageCallback Interface

```go
type WebSocketMessageCallback interface {
    OnMessage(message *Message) error
}
```

## Configuration Options

### ClientOption Structure

```go
type ClientOption struct {
    Reconnect          bool          // Enable auto-reconnect; default: true
    ReconnectAttempts  int           // Maximum reconnect attempts, -1 means forever; default: -1
    ReconnectInterval  time.Duration // Interval between reconnect attempts; default: 5s
    DialTimeout        time.Duration // Timeout for establishing connection; default: 10s
    ReadBufferBytes    int           // I/O buffer sizes in bytes; default: 2048000
    ReadMessageBuffer  int           // Read message buffer size; default: 1024
    WriteMessageBuffer int           // Write message buffer size; default: 256
    WriteTimeout       time.Duration // Write timeout; default: 30s
    EventCallback      Callback      // Event callback function
}
```

### Using ClientOptionBuilder

```go
options := websocket.NewClientOptionBuilder().
    WithReconnect(true).                              // Enable reconnection
    WithReconnectAttempts(10).                        // Max 10 reconnect attempts
    WithReconnectInterval(3 * time.Second).           // 3 seconds between attempts
    WithDialTimeout(15 * time.Second).                // 15 seconds dial timeout
    WithReadBufferBytes(4096000).                     // 4MB read buffer
    WithReadMessageBuffer(2048).                      // 2048 message buffer
    WithWriteMessageBuffer(512).                      // 512 message buffer
    WithWriteTimeout(45 * time.Second).               // 45 seconds write timeout
    WithEventCallback(handleWebSocketEvents).         // Custom event handler
    Build()
```

## Event System

### Available Events

```go
const (
    EventConnected        // Connection established successfully
    EventDisconnected     // Connection closed
    EventTryReconnect     // Attempting to reconnect
    EventMessageReceived  // New message received
    EventErrorReceived    // Error message received
    EventPongReceived     // Pong response received
    EventReadBufferFull   // Read buffer is full (messages may be dropped)
    EventWriteBufferFull  // Write buffer is full (messages may be dropped)
    EventCallbackError    // Error occurred in callback function
    EventReSubscribeOK    // Resubscription successful
    EventReSubscribeError // Resubscription failed
    EventClientFail       // Client failure
    EventClientShutdown   // Client shutdown
)
```

### Event Callback Implementation

```go
func handleWebSocketEvents(event websocket.Event, msg string) {
    switch event {
    case websocket.EventConnected:
        log.Println("✅ WebSocket connected successfully")
        // Re-subscribe to topics if needed

    case websocket.EventDisconnected:
        log.Println("❌ WebSocket disconnected")
        // Handle disconnection logic

    case websocket.EventTryReconnect:
        log.Println("🔄 Attempting to reconnect...")

    case websocket.EventMessageReceived:
        log.Printf("📨 Message received: %s", msg)

    case websocket.EventErrorReceived:
        log.Printf("❌ Error received: %s", msg)

    case websocket.EventPongReceived:
        log.Println("🏓 Pong received - connection alive")

    case websocket.EventReadBufferFull:
        log.Println("⚠️  Read buffer full - some messages may be lost")

    case websocket.EventWriteBufferFull:
        log.Println("⚠️  Write buffer full - some messages may be dropped")

    case websocket.EventClientFail:
        log.Printf("💥 Client failed: %s", msg)

    case websocket.EventClientShutdown:
        log.Println("🛑 Client shutdown")

    default:
        log.Printf("🔍 Unknown event: %s - %s", event, msg)
    }
}
```

## Message Types

### Message Structure

```go
type Message struct {
    ID             string      `json:"id"`             // Unique message identifier
    Type           MessageType `json:"type,omitempty"` // Message type
    SequenceNumber int64       `json:"sn,omitempty"`   // Sequence number
    Topic          string      `json:"topic,omitempty"`// Topic for subscription
    Subject        string      `json:"subject,omitempty"` // Message subject
    PrivateChannel bool        `json:"privateChannel,omitempty"` // Private channel flag
    Response       bool        `json:"response,omitempty"` // Response flag
    Data           interface{} `json:"data,omitempty"`   // Message data
    RawData        string      `json:"-"`               // Raw JSON for debugging
}
```

### Available Message Types

```go
const (
    MessageTypeWelcome   MessageType = "welcome"
    MessageTypePing      MessageType = "ping"
    MessageTypePong      MessageType = "pong"
    MessageTypeAck       MessageType = "ack"
    MessageTypeError     MessageType = "error"
    MessageTypeMessage   MessageType = "message"
    MessageTypeSubscribe MessageType = "subscribe"
)
```

## Examples

### Example 1: Basic Subscription

```go
func basicSubscriptionExample() {
    tokenProvider := &MyTokenProvider{endpoint: "wss://api.example.com"}
    logger := NewLogger()

    options := websocket.NewClientOptionBuilder().
        WithEventCallback(func(event websocket.Event, msg string) {
            log.Printf("Event: %s, Message: %s", event, msg)
        }).
        Build()

    client := websocket.NewWebSocketClient(tokenProvider, logger, options)

    if err := client.Start(); err != nil {
        log.Fatal(err)
    }
    defer client.Stop()

    // Subscribe to market ticker
    subscription := &websocket.Message{
        ID:      "ticker-subscription",
        Type:    websocket.MessageTypeSubscribe,
        Topic:   "/market/ticker:BTC-USDT",
        Subject: "ticker",
    }

    ctx := context.Background()
    errChan := client.Write(ctx, subscription)

    if err := <-errChan; err != nil {
        log.Printf("Subscription failed: %v", err)
        return
    }

    // Process messages
    for message := range client.Read() {
        if message.Subject == "ticker" {
            log.Printf("Ticker update: %+v", message.Data)
        }
    }
}
```

### Example 2: Multiple Subscriptions with Context

```go
func multipleSubscriptionsExample() {
    tokenProvider := &MyTokenProvider{endpoint: "wss://api.example.com"}
    logger := NewLogger()

    client := websocket.NewWebSocketClient(tokenProvider, logger, nil)

    if err := client.Start(); err != nil {
        log.Fatal(err)
    }
    defer client.Stop()

    // Create subscriptions
    subscriptions := []*websocket.Message{
        {
            ID:      "btc-ticker",
            Type:    websocket.MessageTypeSubscribe,
            Topic:   "/market/ticker:BTC-USDT",
            Subject: "ticker",
        },
        {
            ID:      "eth-ticker",
            Type:    websocket.MessageTypeSubscribe,
            Topic:   "/market/ticker:ETH-USDT",
            Subject: "ticker",
        },
    }

    // Send all subscriptions
    for _, sub := range subscriptions {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        errChan := client.Write(ctx, sub)

        if err := <-errChan; err != nil {
            log.Printf("Failed to subscribe to %s: %v", sub.Topic, err)
        }
        cancel()
    }

    // Process messages with filtering
    for message := range client.Read() {
        switch message.Subject {
        case "ticker":
            processTicker(message)
        default:
            log.Printf("Unknown message type: %s", message.Subject)
        }
    }
}

func processTicker(message *websocket.Message) {
    log.Printf("Ticker for %s: %+v", message.Topic, message.Data)
}
```

### Example 3: Reconnection Handling

```go
func reconnectionExample() {
    tokenProvider := &MyTokenProvider{endpoint: "wss://api.example.com"}
    logger := NewLogger()

    var subscriptions []*websocket.Message

    options := websocket.NewClientOptionBuilder().
        WithReconnect(true).
        WithReconnectAttempts(10).
        WithReconnectInterval(2 * time.Second).
        WithEventCallback(func(event websocket.Event, msg string) {
            switch event {
            case websocket.EventConnected:
                log.Println("Connected! Re-subscribing...")
                resubscribe(client, subscriptions)

            case websocket.EventDisconnected:
                log.Println("Disconnected! Will attempt reconnection...")

            case websocket.EventTryReconnect:
                log.Println("Reconnecting...")

            case websocket.EventClientFail:
                log.Printf("Client failed: %s", msg)
            }
        }).
        Build()

    client := websocket.NewWebSocketClient(tokenProvider, logger, options)

    if err := client.Start(); err != nil {
        log.Fatal(err)
    }
    defer client.Stop()

    // Initial subscriptions
    subscriptions = []*websocket.Message{
        {
            ID:      "btc-ticker",
            Type:    websocket.MessageTypeSubscribe,
            Topic:   "/market/ticker:BTC-USDT",
            Subject: "ticker",
        },
    }

    resubscribe(client, subscriptions)

    // Handle reconnections
    go func() {
        for range client.Reconnected() {
            log.Println("Reconnection detected, re-subscribing...")
            resubscribe(client, subscriptions)
        }
    }()

    // Process messages
    for message := range client.Read() {
        log.Printf("Message: %+v", message)
    }
}

func resubscribe(client websocket.Client, subscriptions []*websocket.Message) {
    for _, sub := range subscriptions {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        errChan := client.Write(ctx, sub)

        go func(s *websocket.Message) {
            defer cancel()
            if err := <-errChan; err != nil {
                log.Printf("Failed to resubscribe to %s: %v", s.Topic, err)
            }
        }(sub)
    }
}
```

### Example 4: Custom Message Handler

```go
type MarketDataHandler struct {
    client websocket.Client
    logger monitoring.Logger
}

func (h *MarketDataHandler) OnMessage(message *websocket.Message) error {
    switch message.Subject {
    case "ticker":
        return h.handleTicker(message)
    case "orderbook":
        return h.handleOrderBook(message)
    case "trade":
        return h.handleTrade(message)
    default:
        h.logger.Warn("Unknown message subject", "subject", message.Subject)
        return nil
    }
}

func (h *MarketDataHandler) handleTicker(message *websocket.Message) error {
    h.logger.Info("Processing ticker", "topic", message.Topic, "data", message.Data)
    // Process ticker data
    return nil
}

func (h *MarketDataHandler) handleOrderBook(message *websocket.Message) error {
    h.logger.Info("Processing order book", "topic", message.Topic)
    // Process order book data
    return nil
}

func (h *MarketDataHandler) handleTrade(message *websocket.Message) error {
    h.logger.Info("Processing trade", "topic", message.Topic)
    // Process trade data
    return nil
}

func customHandlerExample() {
    tokenProvider := &MyTokenProvider{endpoint: "wss://api.example.com"}
    logger := NewLogger()

    client := websocket.NewWebSocketClient(tokenProvider, logger, nil)
    handler := &MarketDataHandler{client: client, logger: logger}

    if err := client.Start(); err != nil {
        log.Fatal(err)
    }
    defer client.Stop()

    // Process messages with custom handler
    go func() {
        for message := range client.Read() {
            if err := handler.OnMessage(message); err != nil {
                logger.Error("Message handling failed", "error", err)
            }
        }
    }()

    // Send subscriptions...
}
```

## Best Practices

### 1. Error Handling

```go
// Always handle connection errors
if err := client.Start(); err != nil {
    log.Fatal("Failed to start WebSocket client:", err)
}

// Handle write errors asynchronously
go func() {
    errChan := client.Write(ctx, message)
    if err := <-errChan; err != nil {
        log.Printf("Write failed: %v", err)
        // Implement retry logic or error recovery
    }
}()

// Use context with timeout for operations
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
```

### 2. Resource Management

```go
// Always defer client.Stop()
defer client.Stop()

// Close token provider when done
defer tokenProvider.Close()

// Use buffered channels if processing is slow
messageBuffer := make(chan *websocket.Message, 1000)

go func() {
    for message := range client.Read() {
        select {
        case messageBuffer <- message:
        default:
            log.Println("Message buffer full, dropping message")
        }
    }
}()
```

### 3. Monitoring and Metrics

```go
// Implement proper monitoring
options := websocket.NewClientOptionBuilder().
    WithEventCallback(func(event websocket.Event, msg string) {
        // Log metrics
        metrics.Counter("websocket_events").
            With("event", event.String()).
            Inc()

        switch event {
        case websocket.EventReadBufferFull:
            metrics.Counter("websocket_buffer_overflows").
                With("type", "read").
                Inc()
        case websocket.EventWriteBufferFull:
            metrics.Counter("websocket_buffer_overflows").
                With("type", "write").
                Inc()
        }
    }).
    Build()
```

### 4. Configuration for Production

```go
// Production-ready configuration
options := websocket.NewClientOptionBuilder().
    WithReconnect(true).
    WithReconnectAttempts(-1).                    // Infinite retries
    WithReconnectInterval(5 * time.Second).       // 5 second intervals
    WithDialTimeout(30 * time.Second).            // Longer timeout
    WithReadBufferBytes(4 * 1024 * 1024).        // 4MB buffer
    WithReadMessageBuffer(2048).                  // Large message buffer
    WithWriteMessageBuffer(512).                  // Moderate write buffer
    WithWriteTimeout(30 * time.Second).           // 30 second write timeout
    WithEventCallback(productionEventHandler).    // Comprehensive logging
    Build()
```

## Testing

### Running Tests

```bash
# Run all tests
go test -v ./...

# Run tests with race detection
go test -v -race ./...

# Run specific test
go test -v -run TestWebSocketClient_ping

# Run benchmarks
go test -v -bench=.
```

### Test Coverage

```bash
# Generate coverage report
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Example Test

```go
func TestWebSocketClient_CustomScenario(t *testing.T) {
    // Create mock server
    server := httptest.NewServer(http.HandlerFunc(mockWebSocketServer))
    defer server.Close()

    serverURL := "ws" + server.URL[4:]

    // Create test token provider
    tp := &tokenProviderMock{
        endpoint:     serverURL,
        PingInterval: 200,
    }

    // Configure client options
    options := websocket.NewClientOptionBuilder().
        WithReconnect(false).
        WithEventCallback(func(event websocket.Event, msg string) {
            t.Logf("Event: %s, Message: %s", event, msg)
        }).
        Build()

    // Create client
    client := websocket.NewWebSocketClient(tp, logger, options)

    // Test client operations
    assert.NoError(t, client.Start())
    defer client.Stop()

    // Test message sending
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    message := &websocket.Message{
        ID:   "test-message",
        Type: websocket.MessageTypeSubscribe,
    }

    errChan := client.Write(ctx, message)
    assert.NoError(t, <-errChan)

    // Verify message received
    select {
    case receivedMsg := <-client.Read():
        assert.NotNil(t, receivedMsg)
    case <-time.After(2 * time.Second):
        t.Fatal("No message received within timeout")
    }
}
```

## Performance Considerations

### Buffer Sizing

- **ReadBufferBytes**: Set based on expected message sizes (default: 2MB)
- **ReadMessageBuffer**: Size based on message processing speed (default: 1024)
- **WriteMessageBuffer**: Size based on sending frequency (default: 256)

### Memory Usage

The client uses buffered channels and atomic operations to minimize memory allocations and ensure thread safety.

### Goroutine Management

The client automatically manages goroutines for:

- Connection handling
- Message reading/writing
- Ping/pong keepalive
- Reconnection logic

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure all tests pass with race detection (`go test -race ./...`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Development Guidelines

- All code must be thread-safe
- Write comprehensive tests
- Include documentation for public APIs
- Follow Go best practices and idioms
- Ensure backward compatibility

## License

This package is part of the trading-bot project and follows the same licensing terms.
