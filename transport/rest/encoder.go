package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
)

// bufPool reuses byte buffers across encode calls so the buffer-then-write
// pattern doesn't allocate per response.
var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// writeBuffered marshals via marshalFn into a pooled buffer first, then
// writes the response in one shot with a Content-Length set. This avoids
// the "WriteHeader(200) then encode fails midway, client sees a torn body"
// failure mode that plagued the previous direct-to-ResponseWriter pattern.
func writeBuffered(w http.ResponseWriter, contentType string, statusCode int, marshalFn func(io.Writer) error) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	if err := marshalFn(buf); err != nil {
		return err
	}

	h := w.Header()
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	h.Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(statusCode)
	_, err := w.Write(buf.Bytes())
	return err
}

var ErrResponseWrongType = errors.New("response is not of the expected type")

type (
	ErrorEncoder func(ctx context.Context, err error, w http.ResponseWriter)

	EncodeResponseFunc func(context.Context, http.ResponseWriter, interface{}) error
)

type encoderConfig struct {
	allowsEmptyRes bool
}

type encoderOpt func(c *encoderConfig)

func EncoderAllowsEmptyRes(emptyResAllowed bool) encoderOpt {
	return func(c *encoderConfig) {
		c.allowsEmptyRes = emptyResAllowed
	}
}

func defaultEncoderOpts() []encoderOpt {
	return []encoderOpt{EncoderAllowsEmptyRes(false)}
}

func NewHTTPEncoder[I any](
	f func(context.Context, http.ResponseWriter, I) error,
	opts ...encoderOpt,
) EncodeResponseFunc {
	c := new(encoderConfig)
	for _, opt := range append(defaultEncoderOpts(), opts...) {
		opt(c)
	}

	return func(ctx context.Context, w http.ResponseWriter, in any) error {
		if in == nil && c.allowsEmptyRes {
			var zero I
			return f(ctx, w, zero)
		}
		cast, ok := in.(I)
		if in != nil && !ok {
			return ErrResponseWrongType
		}
		return f(ctx, w, cast)
	}
}

// NewEmptyHTTPEncoder writes only the status code, no body. Safe
// without the writeBuffered() guard that the data-bearing encoders
// use — there's no marshalling step that could fail half-way and
// leave a torn response.
func NewEmptyHTTPEncoder(statusCode int) EncodeResponseFunc {
	return func(ctx context.Context, w http.ResponseWriter, in any) error {
		w.WriteHeader(statusCode)
		return nil
	}
}

func RestJSONEncoder[I, O any](
	itemMapper func(in I) O,
	successCode int,
) func(context.Context, http.ResponseWriter, any) error {
	return NewHTTPEncoder(
		func(ctx context.Context, w http.ResponseWriter, in I) error {
			return writeBuffered(w, "application/json; charset=utf-8", successCode, func(buf io.Writer) error {
				return json.NewEncoder(buf).Encode(itemMapper(in))
			})
		},
	)
}
