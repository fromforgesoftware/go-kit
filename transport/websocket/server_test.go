package websocket_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/fromforgesoftware/go-kit/transport/websocket"
)

func wsURL(srv *httptest.Server) string { return "ws" + strings.TrimPrefix(srv.URL, "http") }

// rawClient drives the protocol frame by frame, so the server's behaviour is pinned independently
// of the kit client's own handling.
type rawClient struct {
	t    *testing.T
	conn *gorilla.Conn
}

func dialRaw(t *testing.T, url string) *rawClient {
	t.Helper()
	conn, resp, err := gorilla.DefaultDialer.Dial(url, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return &rawClient{t: t, conn: conn}
}

func (c *rawClient) send(m websocket.Message) {
	c.t.Helper()
	require.NoError(c.t, c.conn.WriteJSON(m))
}

func (c *rawClient) read() websocket.Message {
	c.t.Helper()
	require.NoError(c.t, c.conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	var m websocket.Message
	require.NoError(c.t, c.conn.ReadJSON(&m))
	return m
}

func (c *rawClient) expectWelcome() {
	c.t.Helper()
	m := c.read()
	require.Equal(c.t, websocket.MessageTypeWelcome, m.Type)
	require.NotEmpty(c.t, m.ID)
}

func newHub(t *testing.T, opts ...func(*testing.T) interface{}) *websocket.Hub {
	t.Helper()
	return websocket.NewHub(monitoringtest.NewMonitor(t))
}

func TestHubGreetsAndAnswersPings(t *testing.T) {
	t.Parallel()
	hub := newHub(t)
	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()
	c := dialRaw(t, wsURL(srv))
	c.expectWelcome()

	c.send(websocket.Message{ID: "p1", Type: websocket.MessageTypePing})

	pong := c.read()
	assert.Equal(t, websocket.MessageTypePong, pong.Type)
	assert.Equal(t, "p1", pong.ID, "a pong carries the ping's id, which is how the client matches it")
	assert.Equal(t, 1, hub.Connections())
}

func TestHubSubscribesPublishesAndUnsubscribes(t *testing.T) {
	t.Parallel()
	hub := newHub(t)
	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()
	c := dialRaw(t, wsURL(srv))
	c.expectWelcome()

	c.send(websocket.Message{ID: "s1", Type: websocket.MessageTypeSubscribe, Topic: "market:xbt"})
	ack := c.read()
	assert.Equal(t, websocket.MessageTypeAck, ack.Type)
	assert.Equal(t, "s1", ack.ID)
	assert.Equal(t, 1, hub.Subscribers("market:xbt"))

	assert.Equal(t, 1, hub.Publish("market:xbt", "quote", map[string]any{"bid": 1.0}))
	assert.Equal(t, 1, hub.Publish("market:xbt", "quote", map[string]any{"bid": 2.0}))
	assert.Equal(t, 0, hub.Publish("market:eth", "quote", nil), "nobody listens to eth")

	first := c.read()
	second := c.read()
	assert.Equal(t, websocket.MessageTypeMessage, first.Type)
	assert.Equal(t, websocket.TopicType("market:xbt"), first.Topic)
	assert.Equal(t, "quote", first.Subject)
	assert.Equal(t, int64(1), first.SequenceNumber)
	assert.Equal(t, int64(2), second.SequenceNumber, "sequence numbers are per topic and dense")
	data, _ := second.Data.(map[string]any)
	assert.Equal(t, 2.0, data["bid"])

	c.send(websocket.Message{ID: "u1", Type: websocket.MessageTypeUnsubscribe, Topic: "market:xbt"})
	ack = c.read()
	assert.Equal(t, websocket.MessageTypeAck, ack.Type)
	assert.Equal(t, "u1", ack.ID)
	assert.Equal(t, 0, hub.Subscribers("market:xbt"))
	assert.Equal(t, 0, hub.Publish("market:xbt", "quote", nil))
}

func TestHubHooksAcceptRefuseAndRelease(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var released []string
	hub := websocket.NewHub(monitoringtest.NewMonitor(t),
		websocket.WithSubscribeHook(func(_ context.Context, info websocket.ConnInfo, topic websocket.TopicType) error {
			if topic == "forbidden" {
				return errors.New("no such topic")
			}
			assert.NotEmpty(t, info.ID)
			return nil
		}),
		websocket.WithUnsubscribeHook(func(_ websocket.ConnInfo, topic websocket.TopicType) {
			mu.Lock()
			released = append(released, string(topic))
			mu.Unlock()
		}),
	)
	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()
	c := dialRaw(t, wsURL(srv))
	c.expectWelcome()

	c.send(websocket.Message{ID: "s1", Type: websocket.MessageTypeSubscribe, Topic: "forbidden"})
	refusal := c.read()
	assert.Equal(t, websocket.MessageTypeError, refusal.Type)
	assert.Equal(t, "s1", refusal.ID)
	assert.Equal(t, "no such topic", refusal.Data)
	assert.Equal(t, 0, hub.Subscribers("forbidden"))

	c.send(websocket.Message{ID: "s2", Type: websocket.MessageTypeSubscribe, Topic: "a"})
	c.read()
	c.send(websocket.Message{ID: "s3", Type: websocket.MessageTypeSubscribe, Topic: "b"})
	c.read()
	c.send(websocket.Message{ID: "s4", Type: websocket.MessageTypeSubscribe, Topic: "a"})
	again := c.read()
	assert.Equal(t, websocket.MessageTypeAck, again.Type, "subscribing twice is acknowledged once more, not an error")
	assert.Equal(t, 1, hub.Subscribers("a"))

	require.NoError(t, c.conn.Close())
	waitUntil(t, func() bool { return hub.Connections() == 0 })

	mu.Lock()
	defer mu.Unlock()
	assert.ElementsMatch(t, []string{"a", "b"}, released, "closing releases every subscription once")
	assert.Equal(t, 0, hub.Subscribers("a"))
}

func TestHubRefusesWhatAClientMustNotSend(t *testing.T) {
	t.Parallel()
	hub := newHub(t)
	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()
	c := dialRaw(t, wsURL(srv))
	c.expectWelcome()

	tests := []struct {
		name string
		msg  websocket.Message
		want string
	}{
		{name: "empty topic", msg: websocket.Message{ID: "1", Type: websocket.MessageTypeSubscribe}, want: "a subscription needs a topic"},
		{name: "server-only type", msg: websocket.Message{ID: "2", Type: websocket.MessageTypeWelcome}, want: `a client does not send "welcome"`},
		{name: "unknown type", msg: websocket.Message{ID: "3", Type: "shout"}, want: `unknown message type "shout"`},
	}
	for _, tt := range tests {
		c.send(tt.msg)
		got := c.read()
		assert.Equal(t, websocket.MessageTypeError, got.Type, tt.name)
		assert.Equal(t, tt.msg.ID, got.ID, tt.name)
		assert.Equal(t, tt.want, got.Data, tt.name)
	}
}

func TestHubAuthenticatesTheUpgrade(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(monitoringtest.NewMonitor(t),
		websocket.WithAuthenticator(func(r *http.Request) error {
			if r.URL.Query().Get("token") != "secret" {
				return errors.New("bad token")
			}
			return nil
		}))
	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()

	_, resp, err := gorilla.DefaultDialer.Dial(wsURL(srv), nil)
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	_ = resp.Body.Close()

	c := dialRaw(t, wsURL(srv)+"?token=secret")
	c.expectWelcome()
}

func TestHubDisconnectsASlowConsumerByDefaultAndDropsWhenAsked(t *testing.T) {
	t.Parallel()
	disconnecting := websocket.NewHub(monitoringtest.NewMonitor(t), websocket.WithSendBuffer(1))
	srv := httptest.NewServer(disconnecting.Handler())
	defer srv.Close()
	c := dialRaw(t, wsURL(srv))
	c.expectWelcome()
	c.send(websocket.Message{ID: "s", Type: websocket.MessageTypeSubscribe, Topic: "t"})
	c.read()
	for i := 0; i < 200; i++ {
		disconnecting.Publish("t", "burst", i)
	}
	waitUntil(t, func() bool { return disconnecting.Connections() == 0 })

	dropping := websocket.NewHub(monitoringtest.NewMonitor(t), websocket.WithSendBuffer(1),
		websocket.WithSlowConsumerPolicy(websocket.SlowConsumerDrop))
	srv2 := httptest.NewServer(dropping.Handler())
	defer srv2.Close()
	c2 := dialRaw(t, wsURL(srv2))
	c2.expectWelcome()
	c2.send(websocket.Message{ID: "s", Type: websocket.MessageTypeSubscribe, Topic: "t"})
	c2.read()
	for i := 0; i < 200; i++ {
		dropping.Publish("t", "burst", i)
	}
	assert.Equal(t, 1, dropping.Connections(), "a dropping hub keeps the connection")
	assert.Greater(t, dropping.Dropped(), int64(0))
}

func TestHubCloseEndsEveryConnectionAndRefusesNewOnes(t *testing.T) {
	t.Parallel()
	hub := newHub(t)
	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()
	c := dialRaw(t, wsURL(srv))
	c.expectWelcome()

	hub.Close()

	require.NoError(t, c.conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	var m websocket.Message
	require.Error(t, c.conn.ReadJSON(&m), "the connection is gone")
	_, resp, err := gorilla.DefaultDialer.Dial(wsURL(srv), nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHubRejectsAForeignOriginByDefault(t *testing.T) {
	t.Parallel()
	hub := newHub(t)
	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()

	_, resp, err := gorilla.DefaultDialer.Dial(wsURL(srv), http.Header{"Origin": {"https://evil.example"}})
	require.Error(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	_ = resp.Body.Close()

	loopback := dialRaw(t, wsURL(srv))
	loopback.expectWelcome()
}

// tokenProvider points the kit client at the test server.
type tokenProvider struct{ endpoint string }

func (p tokenProvider) GetToken() ([]*websocket.Token, error) {
	return []*websocket.Token{{Token: "t", Endpoint: p.endpoint, PingInterval: 200, PingTimeout: 2000}}, nil
}

func (tokenProvider) Close() error { return nil }

func TestTheKitClientSpeaksToTheHub(t *testing.T) {
	t.Parallel()
	hub := newHub(t)
	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()

	client := websocket.NewClient(tokenProvider{endpoint: wsURL(srv)}, monitoringtest.NewMonitor(t),
		websocket.WithReconnect(false))
	require.NoError(t, client.Start())
	defer func() { _ = client.Stop() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := <-client.Write(ctx, &websocket.Message{ID: "sub-1", Type: websocket.MessageTypeSubscribe, Topic: "market:xbt"})
	require.NoError(t, err, "the ack the hub sends is what resolves the client's write")
	waitUntil(t, func() bool { return hub.Subscribers("market:xbt") == 1 })

	hub.Publish("market:xbt", "quote", map[string]any{"bid": 65000.5})

	select {
	case m := <-client.Read():
		assert.Equal(t, websocket.MessageTypeMessage, m.Type)
		assert.Equal(t, websocket.TopicType("market:xbt"), m.Topic)
		raw, _ := json.Marshal(m.Data)
		assert.JSONEq(t, `{"bid":65000.5}`, string(raw))
	case <-time.After(5 * time.Second):
		t.Fatal("the kit client never received the published frame")
	}

	err = <-client.Write(ctx, &websocket.Message{ID: "sub-2", Type: websocket.MessageTypeSubscribe, Topic: ""})
	require.Error(t, err, "the hub's error frame resolves the write with an error")

	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, 1, hub.Connections(), "pings keep the connection alive and are answered")
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}
