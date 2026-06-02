package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/fromforgesoftware/go-kit/transport/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestRequest struct {
	Name string `json:"name"`
}

type TestResponse struct {
	Message string `json:"message"`
}

func jsonEncoder(ctx context.Context, r *http.Request, req TestRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(data))
	r.Header.Set("Content-Type", "application/json")
	return nil
}

func jsonDecoder(ctx context.Context, r *http.Response) (TestResponse, error) {
	var resp TestResponse
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func TestClientCall(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test" && r.Method == http.MethodPost {
			var req TestRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(TestResponse{Message: "Hello " + req.Name})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	endpoint, err := rest.NewPOST("/test", jsonEncoder, jsonDecoder)
	require.NoError(t, err)

	client, err := rest.NewClient("test-client", u, []rest.ClientEndpoint{endpoint})
	require.NoError(t, err)

	// Test Call
	ctx := context.Background()
	req := TestRequest{Name: "World"}
	resp, err := rest.POST[TestResponse](ctx, client, "/test", req)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", resp.Message)
}

func TestClientInterceptors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-value", r.Header.Get("X-Test-Header"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"ok"}`))
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	endpoint, _ := rest.NewGET("/interceptor",
		func(_ context.Context, _ *http.Request, _ struct{}) error { return nil },
		func(_ context.Context, _ *http.Response) (struct{}, error) { return struct{}{}, nil },
	)

	client, err := rest.NewClient("test-client", u, []rest.ClientEndpoint{endpoint},
		rest.WithClientReqInterceptors(func(ctx context.Context, r *http.Request) context.Context {
			r.Header.Set("X-Test-Header", "test-value")
			return ctx
		}),
	)
	require.NoError(t, err)

	_, err = rest.GET[struct{}](context.Background(), client, "/interceptor", struct{}{})
	require.NoError(t, err)
}
