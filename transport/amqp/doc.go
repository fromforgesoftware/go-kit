// Package amqp provides a comprehensive RabbitMQ/AMQP consumer and producer framework.
//
// Features:
//   - Consumer and Producer with functional options
//   - Automatic reconnection via atomic.Pointer connection swap
//   - Exchange and queue management
//   - Routing key patterns
//   - Message encoding/decoding
//   - Broker-side prefetch (QoS) matched to consumer concurrency
//   - In-flight handler drain on Unsubscribe
//   - Error handling with callbacks
//   - OpenTelemetry trace propagation through message headers
//   - Fx dependency injection integration
//
// Basic consumer usage:
//
//	handler := amqp.HandlerFunc(func(ctx context.Context, event MyEvent) error {
//	    log.Printf("Received: %+v", event)
//	    return nil
//	})
//
//	consumer, err := amqp.NewConsumer(
//	    conn, logger, exchange, routingKey, queue,
//	    decodeFunc, handler,
//	    amqp.WithTimeout(10*time.Second),
//	    amqp.WithAutoAck(false),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	consumer.Subscribe(ctx, func(ctx context.Context, err error) {
//	    log.Printf("Error: %v", err)
//	})
//
// Producer usage:
//
//	producer, err := amqp.NewProducer(conn, logger, exchange)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	err = producer.Publish(ctx, routingKey, myEvent)
package amqp
