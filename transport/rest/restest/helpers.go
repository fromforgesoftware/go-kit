package restest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/fromforgesoftware/go-kit/application/usecase"
	"github.com/fromforgesoftware/go-kit/golden"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/fromforgesoftware/go-kit/transport/rest"
	"github.com/stretchr/testify/assert"
)

type handlerTestConfig struct {
	handlerOpts        []rest.HandlerOpt
	reqParams          reqParams
	responseAssertions []func(t *testing.T, s *HandlerSuite, res *http.Response)
	wait               *sync.WaitGroup
}

func newConfig(opts ...HandlerTestOpt) *handlerTestConfig {
	config := &handlerTestConfig{
		handlerOpts: []rest.HandlerOpt{},
		reqParams: reqParams{
			urlVals: make(url.Values),
			headers: make(map[string]string),
		},
	}
	for _, opt := range opts {
		opt(config)
	}

	return config
}

type HandlerTestOpt func(test *handlerTestConfig)

func WithWaitGroup(wg *sync.WaitGroup) HandlerTestOpt {
	return func(test *handlerTestConfig) {
		test.wait = wg
	}
}

func responseValidators(validators ...func(t *testing.T, suite *HandlerSuite, res *http.Response)) HandlerTestOpt {
	return func(testConfig *handlerTestConfig) {
		//nolint: bodyclose // body is closed in the exec func
		testConfig.responseAssertions = append(testConfig.responseAssertions, validators...)
	}
}

func WithReqHeaders(headers map[string]string) HandlerTestOpt {
	return func(testConfig *handlerTestConfig) {
		maps.Copy(testConfig.reqParams.headers, headers)
	}
}

func WithReqValue(key string, values ...string) HandlerTestOpt {
	return func(testConfig *handlerTestConfig) {
		if arr, found := testConfig.reqParams.urlVals[key]; !found || arr == nil {
			testConfig.reqParams.urlVals[key] = []string{}
		}
		testConfig.reqParams.urlVals[key] = append(testConfig.reqParams.urlVals[key], values...)
	}
}

func WithReqEmptyBody() HandlerTestOpt {
	return WithReqBody(http.NoBody)
}

func WithReqBody(reader io.Reader) HandlerTestOpt {
	return func(testConfig *handlerTestConfig) {
		testConfig.reqParams.body = reader
	}
}

func WithReqBodyFromFile(t *testing.T, fileFolder, fileName string) HandlerTestOpt {
	t.Helper()

	//nolint:gosec //no file inclusion issue, this is a test
	f, err := os.Open(filepath.Join(fileFolder, fileName))
	assert.NoError(t, err)

	return WithReqBody(f)
}

func WithHandlerOpts(opts ...rest.HandlerOpt) HandlerTestOpt {
	return func(testConfig *handlerTestConfig) {
		testConfig.handlerOpts = append(testConfig.handlerOpts, opts...)
	}
}

func AssertResponseStatus(status int) HandlerTestOpt {
	return responseValidators(
		func(t *testing.T, s *HandlerSuite, res *http.Response) {
			t.Helper()

			assert.Equal(t, status, res.StatusCode)
		},
	)
}

func AssertCreateResponseOK() HandlerTestOpt {
	return AssertResponseStatus(http.StatusCreated)
}

func AssertUpdateResponseOK() HandlerTestOpt {
	return AssertResponseStatus(http.StatusOK)
}

func AssertListResponseOK() HandlerTestOpt {
	return AssertResponseStatus(http.StatusOK)
}

func AssertGetResponseOK() HandlerTestOpt {
	return AssertResponseStatus(http.StatusOK)
}

func AssertResMatchingFile(fileDir, fileName string, updateGoldenFile bool) HandlerTestOpt {
	return responseValidators(
		func(t *testing.T, s *HandlerSuite, res *http.Response) {
			t.Helper()

			assertResMatchingGoldenFileContent(
				t, res, getHandlerGoldenFilePath(fileDir, fileName),
				updateGoldenFile,
			)
		},
	)
}

type reqParams struct {
	urlVals url.Values
	headers map[string]string
	body    io.Reader
}

type HandlerTest struct {
	name               string
	handler            http.Handler
	req                *http.Request
	responseAssertions []func(t *testing.T, s *HandlerSuite, res *http.Response)
	wg                 *sync.WaitGroup
}

type HandlerSuite struct {
	tests []*HandlerTest
}

func (hts *HandlerSuite) Exec(t *testing.T) {
	t.Helper()

	for _, test := range hts.tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			rr := httptest.NewRecorder()
			test.handler.ServeHTTP(rr, test.req)
			res := rr.Result()
			defer func() { _ = res.Body.Close() }()

			if len(test.responseAssertions) < 1 {
				t.Fatal("test must have at least on assertion")
			}
			for _, assertion := range test.responseAssertions {
				assertion(t, hts, res)
			}
			if test.wg != nil {
				test.wg.Wait()
			}
		})
	}
}

func NewHandlerSuite(tests ...*HandlerTest) *HandlerSuite {
	return &HandlerSuite{tests}
}

func NewHandlerTest(
	name string, handler http.Handler, req *http.Request,
	opts ...HandlerTestOpt,
) *HandlerTest {
	config := newConfig(opts...)

	return &HandlerTest{
		name: name, handler: handler, req: req,
		responseAssertions: config.responseAssertions,
		wg:                 config.wait,
	}
}

func NewEndpointsHandler(version, basePath string, endpoints ...rest.Endpoint) http.Handler {
	m := http.NewServeMux()
	for _, e := range endpoints {
		p := path.Join(version, basePath, e.Path())
		e = rest.NewEndpoint(e.Method(), p, e)
		m.Handle(fmt.Sprintf("%s %s", e.Method(), strings.TrimSuffix(e.Path(), "/")), e)
	}

	return m
}

func NewReq(t *testing.T, ctx context.Context, method, target string, body io.Reader) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	assert.NoError(t, err)
	assert.NotNil(t, req)

	return req
}

func newGetReq(t *testing.T, ctx context.Context, reqURL string, opts ...HandlerTestOpt) *http.Request {
	t.Helper()

	config := newConfig(opts...)
	target := reqURL
	if len(config.reqParams.urlVals) > 0 {
		target += "?" + config.reqParams.urlVals.Encode()
	}
	req := NewReq(t, ctx, http.MethodGet, target, http.NoBody)
	hs := map[string]string{"Accept": "application/json"}
	maps.Copy(hs, config.reqParams.headers)
	for hName, hVal := range hs {
		req.Header.Set(hName, hVal)
	}

	return req
}

func NewListReq(t *testing.T, ctx context.Context, resType resource.Type, opts ...HandlerTestOpt) *http.Request {
	t.Helper()

	return newGetReq(t, ctx, "/"+resType.String(), opts...)
}

func NewGetReq(t *testing.T, ctx context.Context, resType resource.Type, resID string, opts ...HandlerTestOpt) *http.Request {
	t.Helper()

	var idPath string
	if resID != "" {
		idPath = "/" + resID
	}

	return newGetReq(t, ctx, "/"+resType.String()+idPath, opts...)
}

func NewCreateReq(
	t *testing.T, ctx context.Context, resType resource.Type,
	opts ...HandlerTestOpt,
) *http.Request {
	t.Helper()

	config := newConfig(opts...)
	target := "/" + resType.String()
	if len(config.reqParams.urlVals) > 0 {
		target += "?" + config.reqParams.urlVals.Encode()
	}
	req := NewReq(t, ctx, http.MethodPost, target, config.reqParams.body)
	hs := map[string]string{"Content-Type": "application/json"}
	maps.Copy(hs, config.reqParams.headers)
	for hName, hVal := range hs {
		req.Header.Set(hName, hVal)
	}

	return req
}

func NewUpdateReq(
	t *testing.T, ctx context.Context, resType resource.Type, id string,
	opts ...HandlerTestOpt,
) *http.Request {
	t.Helper()

	config := newConfig(opts...)
	req := NewReq(t, ctx, http.MethodPatch, fmt.Sprintf("/%s/%s", resType.String(), id), config.reqParams.body)
	hs := map[string]string{"Content-Type": "application/json"}
	maps.Copy(hs, config.reqParams.headers)
	for hName, hVal := range hs {
		req.Header.Set(hName, hVal)
	}

	return req
}

func NewDeleteReq(t *testing.T, ctx context.Context, resType resource.Type, resID string, opts ...HandlerTestOpt) *http.Request {
	t.Helper()

	config := newConfig(opts...)
	target := fmt.Sprintf("/%s/%s", resType.String(), resID)
	req := NewReq(t, ctx, http.MethodDelete, target, http.NoBody)
	for hName, hVal := range config.reqParams.headers {
		req.Header.Set(hName, hVal)
	}

	return req
}

func NewCreateHandlerTest[DTO, R resource.Resource](
	t *testing.T,
	ctx context.Context, name string, resType resource.Type,
	c usecase.Creator[R], decoder func(DTO) R, encoder func(R) DTO, opts ...HandlerTestOpt,
) *HandlerTest {
	t.Helper()

	config := newConfig(opts...)
	// trace := monitoringtest.NewMonitor(t).Tracer()
	h := rest.NewCreateHandler(c, decoder, encoder, config.handlerOpts...)

	return NewHandlerTest(
		name, h, NewCreateReq(
			t, ctx, resType, opts...,
		), opts...,
	)
}

func NewGetHandlerTest[DTO, R resource.Resource](
	t *testing.T,
	ctx context.Context, name string, resType resource.Type, resID string,
	c usecase.Getter[R], encoder func(R) DTO, parseOpts []query.ParseOpt, opts ...HandlerTestOpt,
) *HandlerTest {
	t.Helper()

	config := newConfig(opts...)
	// trace := monitoringtest.NewMonitor(t).Tracer()
	h := rest.NewGetHandler(c, encoder, parseOpts, config.handlerOpts...)

	m := http.NewServeMux()
	// m.NotFoundHandler = jsonapi.NewRouteNotFoundHandler(trace)
	m.Handle(fmt.Sprintf("%s /%s/{id}", http.MethodGet, resType.String()), h)

	return NewHandlerTest(
		name, m, NewGetReq(t, ctx, resType, resID, opts...),
		opts...,
	)
}

func NewListHandlerTest[DTO, R resource.Resource](
	t *testing.T,
	ctx context.Context, name string, resType resource.Type,
	c usecase.Lister[R], encoder func(R) DTO, opts ...HandlerTestOpt,
) *HandlerTest {
	t.Helper()

	config := newConfig(opts...)
	// trace := monitoringtest.NewMonitor(t).Tracer()
	h := rest.NewListHandler(c, encoder, config.handlerOpts...)

	return NewHandlerTest(
		name, h, NewListReq(t, ctx, resType, opts...),
		opts...,
	)
}

func getHandlerGoldenFilePath(dir, fName string) string {
	return filepath.Join(dir, fName)
}

func assertResMatchingGoldenFileContent(t *testing.T, res *http.Response, goldenFileName string, updateGoldenFile bool) {
	t.Helper()

	defer func() { _ = res.Body.Close() }()
	resDump, err := httputil.DumpResponse(res, true)
	assert.NoError(t, err)

	golden.AssertEqualFile(t, goldenFileName, bytes.NewReader(resDump), updateGoldenFile)
}

func AssertReqMatchingGoldenFileContent(t *testing.T, req *http.Request, goldenFileName string, updateGoldenFile bool) {
	t.Helper()

	defer func() { _ = req.Body.Close() }()
	resDump, err := httputil.DumpRequest(req, true)
	assert.NoError(t, err)

	golden.AssertEqualFile(t, goldenFileName, bytes.NewReader(resDump), updateGoldenFile)
}
