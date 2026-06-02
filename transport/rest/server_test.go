package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/fromforgesoftware/go-kit/transport/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRequest struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type testResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// TestServerHealthEndpoints tests default health check endpoints
func TestServerHealthEndpoints(t *testing.T) {
	server := rest.NewServer(
		rest.WithAddress(":0"),
	)

	tests := []struct {
		path string
	}{
		{"/healthz"},
		{"/healthz/"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			server.Handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// TestServerEndpointRegistration tests custom endpoint registration
func TestServerEndpointRegistration(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(testResponse{Message: "created", Code: 201})
	})

	endpoint := rest.NewEndpoint(http.MethodPost, "/test", handler)
	server := rest.NewServer(
		rest.WithEndpoints(endpoint),
	)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()

	server.Handler.ServeHTTP(w, req)

	assert.True(t, called, "Handler should be called")
	assert.Equal(t, http.StatusCreated, w.Code)
}

// TestServerMiddlewareChaining tests middleware execution order. The
// server now applies WithMiddlewares with the FIRST element as the
// outermost runtime layer — matching Router.Use semantics so the
// behaviour is consistent whether middleware is declared at server
// level or inside a Controller's Routes(). Tagging both pre- and
// post-handler phases pins down the order both ways.
func TestServerMiddlewareChaining(t *testing.T) {
	var trace []string
	var mu sync.Mutex
	tag := func(name, phase string) {
		mu.Lock()
		defer mu.Unlock()
		trace = append(trace, name+":"+phase)
	}

	mwFactory := func(name string) rest.Middleware {
		return rest.MiddlewareFunc(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tag(name, "pre")
				next.ServeHTTP(w, r)
				tag(name, "post")
			})
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tag("handler", "run")
		w.WriteHeader(http.StatusOK)
	})

	server := rest.NewServer(
		rest.WithEndpoints(rest.NewEndpoint(http.MethodGet, "/x", handler)),
		rest.WithMiddlewares(mwFactory("first"), mwFactory("second")),
	)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mu.Lock()
	defer mu.Unlock()
	// "first" is at the head of the slice → outermost runtime layer.
	// "second" sits between first and the handler.
	assert.Equal(t, []string{
		"first:pre",
		"second:pre",
		"handler:run",
		"second:post",
		"first:post",
	}, trace, "first-appended middleware ends up outermost")
}

// TestHandlerRequestDecoding tests handler request decoding
func TestHandlerRequestDecoding(t *testing.T) {
	endpoint := func(ctx context.Context, req testRequest) (testResponse, error) {
		return testResponse{
			Message: fmt.Sprintf("Hello %s", req.Name),
			Code:    req.Value,
		}, nil
	}

	decoder := func(ctx context.Context, r *http.Request) (any, error) {
		var req testRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, err
		}
		return req, nil
	}

	encoder := rest.NewHTTPEncoder(func(ctx context.Context, w http.ResponseWriter, resp testResponse) error {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		return json.NewEncoder(w).Encode(resp)
	})

	handler := rest.NewHandler(endpoint, decoder, encoder)

	reqBody := `{"name":"World","value":42}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp testResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", resp.Message)
	assert.Equal(t, 42, resp.Code)
}

// TestHandlerErrorHandling tests error responses
func TestHandlerErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       func(context.Context, testRequest) (testResponse, error)
		expectedStatus int
	}{
		{
			name: "BadRequest",
			endpoint: func(ctx context.Context, req testRequest) (testResponse, error) {
				return testResponse{}, fmt.Errorf("bad request: invalid input")
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "InternalError",
			endpoint: func(ctx context.Context, req testRequest) (testResponse, error) {
				return testResponse{}, fmt.Errorf("something went wrong")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := func(ctx context.Context, r *http.Request) (any, error) {
				return testRequest{}, nil
			}

			encoder := rest.NewHTTPEncoder(func(ctx context.Context, w http.ResponseWriter, resp testResponse) error {
				w.WriteHeader(http.StatusOK)
				return json.NewEncoder(w).Encode(resp)
			})

			handler := rest.NewHandler(tt.endpoint, decoder, encoder)

			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("{}"))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestHandlerDecodingError tests decoder error handling
func TestHandlerDecodingError(t *testing.T) {
	endpoint := func(ctx context.Context, req testRequest) (testResponse, error) {
		return testResponse{Message: "ok"}, nil
	}

	decoder := func(ctx context.Context, r *http.Request) (any, error) {
		return nil, fmt.Errorf("invalid JSON")
	}

	encoder := rest.NewHTTPEncoder(func(ctx context.Context, w http.ResponseWriter, resp testResponse) error {
		return json.NewEncoder(w).Encode(resp)
	})

	handler := rest.NewHandler(endpoint, decoder, encoder)

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("invalid"))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestJSONEncoder tests JSON response encoding
func TestJSONEncoder(t *testing.T) {
	encoder := rest.RestJSONEncoder(
		func(in testResponse) testResponse { return in },
		http.StatusOK,
	)

	resp := testResponse{Message: "created", Code: 201}
	w := httptest.NewRecorder()

	err := encoder(context.Background(), w, resp)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var decoded testResponse
	err = json.NewDecoder(w.Body).Decode(&decoded)
	require.NoError(t, err)
	assert.Equal(t, resp, decoded)
}
