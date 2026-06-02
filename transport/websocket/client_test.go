package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func mockWebSocketServer(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	err = conn.WriteJSON(&Message{
		ID:   IntToString(time.Now().UnixNano()),
		Type: MessageTypeWelcome,
	})
	if err != nil {
		panic(err)
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		m := &Message{}

		err = json.Unmarshal(message, m)
		if err != nil {
			panic(err)
		}

		switch m.Type {
		case MessageTypePing:
			{
				err = conn.WriteJSON(&Message{
					ID:   m.ID,
					Type: MessageTypePong,
				})
				if err != nil {
					panic(err)
				}
			}
		case MessageTypeSubscribe:
			{
				err = conn.WriteJSON(&Message{
					ID:   m.ID,
					Type: MessageTypeAck,
				})
				if err != nil {
					panic(err)
				}
				time.Sleep(time.Millisecond * 10)

				for i := 0; i < 10; i++ {
					err = conn.WriteJSON(&Message{
						ID:   IntToString(int64(i)),
						Type: MessageTypeMessage,
					})
					if err != nil {
						panic(err)
					}
					time.Sleep(time.Millisecond * 10)
				}
			}

		}
	}
}

func mockWebSocketServer2(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	err = conn.WriteJSON(&Message{
		ID:   IntToString(time.Now().UnixNano()),
		Type: MessageTypeWelcome,
	})
	if err != nil {
		panic(err)
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		m := &Message{}

		err = json.Unmarshal(message, m)
		if err != nil {
			panic(err)
		}

		switch m.Type {
		case MessageTypePing:
			{
				err = conn.WriteJSON(&Message{
					ID:   m.ID,
					Type: MessageTypePong,
				})
				if err != nil {
					panic(err)
				}
			}
		case MessageTypeSubscribe:
			{
				log.Println("[server]receive subscribe message")
				return
			}

		}
	}
}

type tokenProviderMock struct {
	endpoint     string
	PingInterval int64
}

func (m *tokenProviderMock) GetToken() ([]*Token, error) {
	if m.PingInterval == 0 {
		m.PingInterval = 10
	}
	return []*Token{{
		Token:        "token-test",
		PingInterval: m.PingInterval,
		Endpoint:     m.endpoint,
		Protocol:     "",
		Encrypt:      false,
		PingTimeout:  200,
	}}, nil
}

func (m *tokenProviderMock) Close() error {
	return nil
}

func TestWebSocketClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockWebSocketServer))
	defer server.Close()

	serverURL := "ws" + server.URL[4:]

	tp := &tokenProviderMock{endpoint: serverURL}

	eventCnt := 0
	opts := []clientOption{WithEventCallback(func(event Event, msg string) {
		if event == EventConnected || event == EventDisconnected {
			eventCnt++
		}
	})}

	m := monitoringtest.NewMonitor(t)

	client := NewClient(tp, m, opts...)
	err := client.Start()
	assert.Nil(t, err)
	<-time.After(time.Millisecond * 20)
	assert.Equal(t, int64(3), atomic.LoadInt64(&client.metric.goroutines))
	err = client.Start()
	assert.NotNil(t, err)
	assert.Equal(t, int64(3), atomic.LoadInt64(&client.metric.goroutines))

	assert.Equal(t, 1, eventCnt)

	client.Stop()
	client.Stop()
	assert.Equal(t, int64(0), atomic.LoadInt64(&client.metric.goroutines))
	assert.Equal(t, 2, eventCnt)
	assert.Equal(t, 0, len(client.ackEvent))
}

func TestWebSocketClient2(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockWebSocketServer))
	defer server.Close()

	serverURL := "ws" + server.URL[4:]

	tp := &tokenProviderMock{endpoint: serverURL}

	eventCnt := 0
	opts := []clientOption{WithEventCallback(func(event Event, msg string) {
		if event == EventConnected || event == EventDisconnected {
			eventCnt++
		}
	})}

	m := monitoringtest.NewMonitor(t)

	client := NewClient(tp, m, opts...)

	for i := 0; i < 10; i++ {
		go func() {
			client.Start()
		}()
	}
	<-time.After(time.Millisecond * 20)
	assert.Equal(t, int64(3), atomic.LoadInt64(&client.metric.goroutines))
	client.Stop()
	assert.Equal(t, int64(0), atomic.LoadInt64(&client.metric.goroutines))
	assert.Equal(t, 2, eventCnt)
	assert.Equal(t, 0, len(client.ackEvent))
}

func TestWebSocketClientPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockWebSocketServer))
	defer server.Close()

	serverURL := "ws" + server.URL[4:]

	tp := &tokenProviderMock{endpoint: serverURL}

	m := monitoringtest.NewMonitor(t)

	client := NewClient(tp, m)
	err := client.Start()
	assert.Nil(t, err)

	<-time.After(time.Millisecond * 200)

	assert.Equal(t, int64(0), atomic.LoadInt64(&client.metric.pingErr))
	assert.True(t, atomic.LoadInt64(&client.metric.pingSuccess) > 0)

	assert.Nil(t, client.Stop())
	assert.Equal(t, 0, len(client.ackEvent))
	assert.Equal(t, int64(0), atomic.LoadInt64(&client.metric.goroutines))

}

func TestWebSocketClientWriteRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockWebSocketServer))
	defer server.Close()

	serverURL := "ws" + server.URL[4:]

	tp := &tokenProviderMock{endpoint: serverURL}

	m := monitoringtest.NewMonitor(t)

	client := NewClient(tp, m)
	err := client.Start()
	assert.Nil(t, err)

	ctx, fc := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer fc()

	ch := client.Write(ctx, &Message{ID: "id123", Type: MessageTypeSubscribe})
	assert.Nil(t, <-ch)

	var cnt int64
	go func() {
		for m := range client.Read() {
			fmt.Println(m)
			atomic.AddInt64(&cnt, 1)
		}
	}()

	<-time.After(time.Millisecond * 200)
	assert.Nil(t, client.Stop())
	assert.Equal(t, int64(10), atomic.LoadInt64(&cnt))
	assert.Equal(t, 0, len(client.ackEvent))
	assert.Equal(t, int64(0), atomic.LoadInt64(&client.metric.goroutines))

}

func TestWebSocketClientReconnect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockWebSocketServer2))
	defer server.Close()

	serverURL := "ws" + server.URL[4:]

	tp := &tokenProviderMock{endpoint: serverURL, PingInterval: 100}

	opts := []clientOption{
		WithEventCallback(func(event Event, msg string) {
			fmt.Println(event)
		}),
		WithReconnectInterval(time.Millisecond * 5),
	}

	m := monitoringtest.NewMonitor(t)

	client := NewClient(tp, m, opts...)
	assert.Nil(t, client.Start())

	<-time.After(time.Millisecond * 200)

	ctx, fc := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer fc()
	ch := client.Write(ctx, &Message{ID: "id123", Type: MessageTypeSubscribe})
	<-ch

	<-time.After(time.Millisecond * 200)

	assert.True(t, atomic.LoadInt64(&client.metric.pingSuccess) > 0)
	assert.Equal(t, int64(3), atomic.LoadInt64(&client.metric.goroutines))

	client.Stop()
	assert.Equal(t, 0, len(client.ackEvent))
	assert.Equal(t, int64(0), atomic.LoadInt64(&client.metric.goroutines))

}

func TestWebSocketClientReconnect2(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockWebSocketServer2))
	defer server.Close()

	serverURL := "ws" + server.URL[4:]

	tp := &tokenProviderMock{endpoint: serverURL, PingInterval: 200}

	var ev atomic.Value
	opts := []clientOption{
		WithReconnectInterval(time.Millisecond * 5),
		WithEventCallback(func(event Event, msg string) {
			fmt.Println(event)
			ev.Store(event)
		}),
		WithReconnect(false),
	}

	m := monitoringtest.NewMonitor(t)

	client := NewClient(tp, m, opts...)
	assert.Nil(t, client.Start())

	<-time.After(time.Millisecond * 200)

	ctx, fc := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer fc()
	ch := client.Write(ctx, &Message{ID: "id123", Type: MessageTypeSubscribe})
	<-ch

	<-time.After(time.Millisecond * 200)
	assert.Equal(t, int64(0), atomic.LoadInt64(&client.metric.goroutines))
	assert.Equal(t, EventClientFail, ev.Load())
}

func TestWebSocketClientReconnect3(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockWebSocketServer2))
	defer server.Close()

	serverURL := "ws" + server.URL[4:]

	tp := &tokenProviderMock{endpoint: serverURL}
	tp.PingInterval = 50

	opts := []clientOption{
		WithReconnectInterval(time.Millisecond * 5),
		WithEventCallback(func(event Event, msg string) {
			fmt.Println(event, msg)
		}),
	}

	m := monitoringtest.NewMonitor(t)

	client := NewClient(tp, m, opts...)
	assert.Nil(t, client.Start())

	<-time.After(time.Millisecond * 200)

	ctx, fc := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer fc()
	ch := client.Write(ctx, &Message{ID: "id123", Type: MessageTypeSubscribe})
	<-ch

	<-time.After(time.Millisecond * 200)

	assert.True(t, atomic.LoadInt64(&client.metric.pingSuccess) > 0)
	assert.Equal(t, int64(3), atomic.LoadInt64(&client.metric.goroutines))

	client.Stop()
	assert.Equal(t, 0, len(client.ackEvent))
	assert.Equal(t, int64(0), atomic.LoadInt64(&client.metric.goroutines))
}
