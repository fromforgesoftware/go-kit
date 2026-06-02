package rest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
)

const (
	defaultClientReqTimeout = 10 * time.Second
	defaultUserAgent        = "Go-rest-client/1.0"

	// Tuned transport defaults for high-throughput service-to-service
	// calls. The stdlib defaults (MaxIdleConnsPerHost=2) throttle modern
	// API workloads to ~2 keep-alive conns/host, serializing requests
	// behind connection acquisition under load.
	defaultMaxIdleConns        = 100
	defaultMaxIdleConnsPerHost = 100
	defaultIdleConnTimeout     = 90 * time.Second
	defaultTLSHandshakeTimeout = 10 * time.Second
)

// NewDefaultHTTPClient returns an *http.Client with a tuned transport
// suitable for service-to-service calls. Use it as the http.Client passed
// to WithHTTPClient when the caller does not have a project-specific
// transport. The returned client uses Timeout=0 — pair with
// WithClientReqTimeout or per-request ctx deadlines.
func NewDefaultHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        defaultMaxIdleConns,
			MaxIdleConnsPerHost: defaultMaxIdleConnsPerHost,
			IdleConnTimeout:     defaultIdleConnTimeout,
			TLSHandshakeTimeout: defaultTLSHandshakeTimeout,
			ForceAttemptHTTP2:   true,
		},
	}
}

// AnyEndpoint is a function that takes a request and returns a response
type AnyEndpoint func(ctx context.Context, request interface{}) (interface{}, error)

type (
	ClientRequestInterceptor  func(context.Context, *http.Request) context.Context
	ClientResponseInterceptor func(context.Context, *http.Response) context.Context
	ClientErrorInterceptor    func(context.Context, error)

	Client interface {
		Call(ctx context.Context, method, path string, req any) (any, error)
	}

	ClientEndpoint interface {
		HttpClient() *http.Client
		Target() *url.URL
		Path() string
		Method() string
		Encode(context.Context, *http.Request, any) error
		Decode(context.Context, *http.Response) (response any, err error)
		ReqInterceptors() []ClientRequestInterceptor
		ResInterceptors() []ClientResponseInterceptor
		ErrInterceptors() []ClientErrorInterceptor
		Endpoint() AnyEndpoint
	}
)

func ClientRequestWithHeaders(kvs ...string) ClientRequestInterceptor {
	return func(ctx context.Context, r *http.Request) context.Context {
		for i := 0; i < len(kvs); i += 2 {
			hval := r.Header.Get(kvs[i])
			if kvs[i] == "Content-Type" && strings.Contains(hval, "multipart/form-data") {
				continue
			}
			r.Header.Set(kvs[i], kvs[i+1])
		}
		return ctx
	}
}

func ClientRequestWithQueryParams(qParams url.Values) ClientRequestInterceptor {
	return func(ctx context.Context, r *http.Request) context.Context {
		for q, val := range qParams {
			qVals := r.URL.Query()
			qVals.Set(q, strings.Join(val, ","))
			r.URL.RawQuery = qVals.Encode()
		}
		return ctx
	}
}

func roundtripReqForMandatoryParams() ClientRequestInterceptor {
	return func(ctx context.Context, r *http.Request) context.Context {
		if r.ContentLength == 0 && r.Body != nil {
			bodyCopy := io.NopCloser(r.Body)
			defer func() { _ = bodyCopy.Close() }()
			bs, readErr := io.ReadAll(bodyCopy)
			if readErr != nil {
				log.Printf("rest.client: body read failed during ContentLength rebuild (%s %s): %v",
					r.Method, r.URL.String(), readErr)
			}
			headers := r.Header.Clone()
			reqWithContentLength, err := http.NewRequestWithContext(ctx, r.Method, r.URL.String(), bytes.NewBuffer(bs))
			if err == nil {
				*r = *reqWithContentLength
				r.Header = headers
				if r.ContentLength > 0 {
					r.Header.Set("Content-Length", fmt.Sprintf("%d", r.ContentLength))
				}
			} else {
				// On rebuild failure we leave the original request untouched
				// — downstream may still succeed with a chunked-encoded
				// body — but log so the silent failure is debuggable.
				log.Printf("rest.client: NewRequestWithContext failed during ContentLength rebuild (%s %s): %v",
					r.Method, r.URL.String(), err)
			}
		}
		if len(r.UserAgent()) < 1 {
			ClientRequestWithHeaders("User-Agent", defaultUserAgent)(ctx, r)
		}
		return ctx
	}
}

func ClientRequestWithBearer(token string) ClientRequestInterceptor {
	return ClientRequestWithHeaders("Authorization", fmt.Sprintf("Bearer %s", token))
}

func ClientRequestWithContentType(mime string) ClientRequestInterceptor {
	return ClientRequestWithHeaders("Content-Type", mime)
}

type ClientEndpointOpt func(c *clientEndpoint)

func WithClientEndpointHttpClient(client *http.Client) ClientEndpointOpt {
	return func(c *clientEndpoint) {
		c.client = client
	}
}

func WithClientEndpointTarget(target *url.URL) ClientEndpointOpt {
	return func(c *clientEndpoint) {
		c.target = target
	}
}

func WithClientEndpointReqInterceptors(reqInterceptors ...ClientRequestInterceptor) ClientEndpointOpt {
	return func(c *clientEndpoint) {
		c.reqInterceptors = append(c.reqInterceptors, reqInterceptors...)
	}
}

func WithClientEndpointResInterceptors(resInterceptors ...ClientResponseInterceptor) ClientEndpointOpt {
	return func(c *clientEndpoint) {
		c.resInterceptors = append(c.resInterceptors, resInterceptors...)
	}
}

func WithClientEndpointErrInterceptors(errInterceptors ...ClientErrorInterceptor) ClientEndpointOpt {
	return func(c *clientEndpoint) {
		c.errInterceptors = append(c.errInterceptors, errInterceptors...)
	}
}

type clientEndpoint struct {
	client          *http.Client
	target          *url.URL
	path            string
	method          string
	enc             func(context.Context, *http.Request, any) error
	dec             func(context.Context, *http.Response) (response any, err error)
	reqInterceptors []ClientRequestInterceptor
	resInterceptors []ClientResponseInterceptor
	errInterceptors []ClientErrorInterceptor
}

func updateEndpoint(ce ClientEndpoint, opts ...ClientEndpointOpt) ClientEndpoint {
	update := &clientEndpoint{
		client:          ce.HttpClient(),
		target:          ce.Target(),
		path:            ce.Path(),
		method:          ce.Method(),
		enc:             ce.Encode,
		dec:             ce.Decode,
		reqInterceptors: ce.ReqInterceptors(),
		resInterceptors: ce.ResInterceptors(),
		errInterceptors: ce.ErrInterceptors(),
	}
	for _, opt := range opts {
		opt(update)
	}
	return update
}

func (e *clientEndpoint) HttpClient() *http.Client { return e.client }
func (e *clientEndpoint) Target() *url.URL         { return e.target }
func (e *clientEndpoint) Path() string {
	if e.path == "/" {
		return ""
	}
	return e.path
}
func (e *clientEndpoint) Method() string { return e.method }

func (e *clientEndpoint) Encode(ctx context.Context, r *http.Request, req any) error {
	return e.enc(ctx, r, req)
}

func (e *clientEndpoint) Decode(ctx context.Context, res *http.Response) (response any, err error) {
	return e.dec(ctx, res)
}

func (e *clientEndpoint) ReqInterceptors() []ClientRequestInterceptor  { return e.reqInterceptors }
func (e *clientEndpoint) ResInterceptors() []ClientResponseInterceptor { return e.resInterceptors }
func (e *clientEndpoint) ErrInterceptors() []ClientErrorInterceptor    { return e.errInterceptors }

func (e *clientEndpoint) Endpoint() AnyEndpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		if e.errInterceptors != nil {
			defer func() {
				for _, f := range e.errInterceptors {
					f(ctx, err)
				}
			}()
		}

		req, err := http.NewRequestWithContext(ctx, e.method, e.target.String(), nil)
		if err != nil {
			return nil, err
		}

		if err = e.enc(ctx, req, request); err != nil {
			return nil, err
		}
		if req.Body != nil {
			defer func() { _ = req.Body.Close() }()
		}

		for _, f := range e.reqInterceptors {
			ctx = f(ctx, req)
		}

		resp, err := e.client.Do(req.WithContext(ctx))
		if err != nil {
			return nil, err
		}

		for _, f := range e.resInterceptors {
			ctx = f(ctx, resp)
		}

		response, err = e.dec(ctx, resp)
		if err != nil {
			return nil, err
		}

		return response, nil
	}
}

func NewPOST[EI, DO any](path string,
	enc func(context.Context, *http.Request, EI) error,
	dec func(context.Context, *http.Response) (response DO, err error),
	opts ...ClientEndpointOpt,
) (ClientEndpoint, error) {
	return newClientEndpoint(http.MethodPost, path, enc, dec, opts...)
}

func NewGET[EI, DO any](path string,
	enc func(context.Context, *http.Request, EI) error,
	dec func(context.Context, *http.Response) (response DO, err error),
	opts ...ClientEndpointOpt,
) (ClientEndpoint, error) {
	return newClientEndpoint(http.MethodGet, path, enc, dec, opts...)
}

func NewPUT[EI, DO any](path string,
	enc func(context.Context, *http.Request, EI) error,
	dec func(context.Context, *http.Response) (response DO, err error),
	opts ...ClientEndpointOpt,
) (ClientEndpoint, error) {
	return newClientEndpoint(http.MethodPut, path, enc, dec, opts...)
}

func NewPATCH[EI, DO any](path string,
	enc func(context.Context, *http.Request, EI) error,
	dec func(context.Context, *http.Response) (response DO, err error),
	opts ...ClientEndpointOpt,
) (ClientEndpoint, error) {
	return newClientEndpoint(http.MethodPatch, path, enc, dec, opts...)
}

func NewDELETE[EI, DO any](path string,
	enc func(context.Context, *http.Request, EI) error,
	dec func(context.Context, *http.Response) (response DO, err error),
	opts ...ClientEndpointOpt,
) (ClientEndpoint, error) {
	return newClientEndpoint(http.MethodDelete, path, enc, dec, opts...)
}

func newClientEndpoint[EI, DO any](
	method string, path string,
	enc func(context.Context, *http.Request, EI) error,
	dec func(context.Context, *http.Response) (response DO, err error),
	opts ...ClientEndpointOpt,
) (ClientEndpoint, error) {
	if len(method) < 1 {
		return nil, apierrors.MissingField("method")
	}
	if len(path) < 1 {
		return nil, apierrors.MissingField("path")
	}
	if enc == nil {
		return nil, apierrors.MissingField("encoder")
	}
	if dec == nil {
		return nil, apierrors.MissingField("decoder")
	}

	endPath := path
	if !strings.HasPrefix(endPath, "/") {
		endPath = "/" + endPath
	}

	c := &clientEndpoint{
		method: method, path: endPath,
		enc: func(ctx context.Context, r *http.Request, req any) error {
			in, ok := req.(EI)
			if !ok {
				return fmt.Errorf("invalid request type: expected %T, got %T", *new(EI), req)
			}
			return enc(ctx, r, in)
		},
		dec: func(ctx context.Context, res *http.Response) (response any, err error) {
			return dec(ctx, res)
		},
	}
	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func POST[DO, EI any](ctx context.Context, c Client, path string, req EI) (DO, error) {
	return call[DO](ctx, c, http.MethodPost, path, req)
}

func GET[DO, EI any](ctx context.Context, c Client, path string, req EI) (DO, error) {
	return call[DO](ctx, c, http.MethodGet, path, req)
}

func PUT[DO, EI any](ctx context.Context, c Client, path string, req EI) (DO, error) {
	return call[DO](ctx, c, http.MethodPut, path, req)
}

func PATCH[DO, EI any](ctx context.Context, c Client, path string, req EI) (DO, error) {
	return call[DO](ctx, c, http.MethodPatch, path, req)
}

func DELETE[DO, EI any](ctx context.Context, c Client, path string, req EI) (DO, error) {
	return call[DO](ctx, c, http.MethodDelete, path, req)
}

func call[DO, EI any](ctx context.Context, c Client, method, path string, req EI) (DO, error) {
	res, err := c.Call(ctx, method, path, req)
	if err != nil {
		var res DO
		return res, err
	}
	if res == nil {
		var res DO
		return res, nil
	}
	return res.(DO), nil
}

type clientConfig struct {
	httpClient      *http.Client
	reqInterceptors []ClientRequestInterceptor
	resInterceptors []ClientResponseInterceptor
	errInterceptors []ClientErrorInterceptor
}

type clientOption func(c *clientConfig)

func defaultClientOpts() []clientOption {
	return []clientOption{
		WithHTTPClient(new(http.Client)),
		WithClientReqTimeout(defaultClientReqTimeout),
		WithClientReqInterceptors(
			ClientRequestWithContentType("application/json"),
			roundtripReqForMandatoryParams(),
		),
	}
}

func WithHTTPClient(cli *http.Client) clientOption {
	return func(c *clientConfig) {
		c.httpClient = cli
	}
}

func WithClientReqTimeout(timeout time.Duration) clientOption {
	return func(c *clientConfig) {
		c.httpClient.Timeout = timeout
	}
}

func WithClientReqInterceptors(reqInterceptors ...ClientRequestInterceptor) clientOption {
	return func(c *clientConfig) {
		c.reqInterceptors = append(c.reqInterceptors, reqInterceptors...)
	}
}

func WithClientResInterceptors(resInterceptors ...ClientResponseInterceptor) clientOption {
	return func(c *clientConfig) {
		c.resInterceptors = append(c.resInterceptors, resInterceptors...)
	}
}

type client struct {
	clientName        string
	baseURL           *url.URL
	httpClient        *http.Client
	endpointsByMethod map[string]map[string]AnyEndpoint
}

func NewClient(
	clientName string,
	baseURL *url.URL,
	endpoints []ClientEndpoint,
	opts ...clientOption,
) (*client, error) {
	if len(clientName) < 1 {
		return nil, apierrors.MissingField("clientName")
	}
	if baseURL == nil {
		return nil, apierrors.MissingField("baseURL")
	}
	if len(baseURL.Host) < 1 {
		return nil, apierrors.InvalidFormat("baseURL.host", baseURL.Host, "non-empty host")
	}

	cfg := &clientConfig{
		reqInterceptors: []ClientRequestInterceptor{},
		resInterceptors: []ClientResponseInterceptor{},
	}
	for _, opt := range append(defaultClientOpts(), opts...) {
		opt(cfg)
	}

	c := &client{
		clientName:        clientName,
		baseURL:           baseURL,
		httpClient:        cfg.httpClient,
		endpointsByMethod: make(map[string]map[string]AnyEndpoint),
	}

	if err := c.addEndpoints(c.baseURL, endpoints, cfg.reqInterceptors, cfg.resInterceptors, cfg.errInterceptors); err != nil {
		return nil, err
	}

	if len(c.endpointsByMethod) < 1 {
		return nil, apierrors.InvalidArgument("at least one endpoint is required")
	}

	return c, nil
}

func (c *client) Call(ctx context.Context, method, path string, req any) (any, error) {
	if path == "/" {
		path = ""
	}
	byMethod, ok := c.endpointsByMethod[path]
	if !ok {
		return nil, apierrors.NotFound("client.endpoint", c.clientName+" "+path)
	}
	endpoint, ok := byMethod[method]
	if !ok {
		return nil, apierrors.NotFound("client.endpoint", c.clientName+" "+method+" "+path)
	}
	return endpoint(ctx, req)
}

func (c *client) addEndpoints(
	baseURL *url.URL,
	endpoints []ClientEndpoint,
	reqInterceptors []ClientRequestInterceptor,
	resInterceptors []ClientResponseInterceptor,
	errInterceptors []ClientErrorInterceptor,
) error {
	for _, end := range endpoints {
		endURL, err := url.Parse(baseURL.String() + end.Path())
		if err != nil {
			return apierrors.InvalidFormat("endpoint.url", baseURL.String()+end.Path(), "parseable URL: "+err.Error())
		}

		end = updateEndpoint(end,
			WithClientEndpointTarget(endURL),
			WithClientEndpointHttpClient(c.httpClient),
			WithClientEndpointReqInterceptors(reqInterceptors...),
			WithClientEndpointResInterceptors(resInterceptors...),
			WithClientEndpointErrInterceptors(errInterceptors...),
		)

		if endByMethod, ok := c.endpointsByMethod[end.Path()]; !ok || endByMethod == nil {
			c.endpointsByMethod[end.Path()] = make(map[string]AnyEndpoint)
		}
		c.endpointsByMethod[end.Path()][end.Method()] = end.Endpoint()
	}
	return nil
}
