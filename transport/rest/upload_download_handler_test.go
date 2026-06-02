package rest_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kiterrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/transport/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubDoc satisfies resource.Resource so the upload handler can
// encode it as a jsonapi document.
type stubDoc struct {
	*dummyMember
}

func TestNewMultipartUploadHandlerHappyPath(t *testing.T) {
	var seenFileBytes []byte
	cmdFn := func(_ context.Context, file io.Reader, cmd uploadCmd) (*dummyMember, error) {
		b, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		seenFileBytes = b
		assert.Equal(t, "W1", cmd.WorkspaceID)
		assert.Equal(t, "report.pdf", cmd.FileName)
		return newDummyMember("D1", cmd.WorkspaceID, "uploader", "owner"), nil
	}
	decoder := func(f rest.MultipartFile, req *http.Request) (uploadCmd, error) {
		return uploadCmd{
			WorkspaceID: req.PathValue("workspaceId"),
			FileName:    f.FileName,
		}, nil
	}
	encoder := func(m *dummyMember) *dummyMember { return m }

	h := rest.NewMultipartUploadHandler(cmdFn, decoder, encoder)

	body, contentType := buildMultipart(t, "file", "report.pdf", []byte("PDF-CONTENTS"))
	mux := http.NewServeMux()
	mux.Handle("POST /v1/workspaces/{workspaceId}/documents", h)
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/W1/documents", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, []byte("PDF-CONTENTS"), seenFileBytes)
}

func TestNewMultipartUploadHandlerMissingFile(t *testing.T) {
	cmdFn := func(_ context.Context, _ io.Reader, _ uploadCmd) (*dummyMember, error) {
		t.Fatal("usecase invoked despite missing file")
		return nil, nil
	}
	decoder := func(_ rest.MultipartFile, _ *http.Request) (uploadCmd, error) {
		return uploadCmd{}, nil
	}
	encoder := func(m *dummyMember) *dummyMember { return m }

	h := rest.NewMultipartUploadHandler(cmdFn, decoder, encoder)

	// Multipart body with no file part.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("note", "hello")
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/x", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

// uploadCmd is the test's typed Command for the upload usecase.
type uploadCmd struct {
	WorkspaceID string
	FileName    string
}

func buildMultipart(t *testing.T, fieldName, fileName string, content []byte) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)
	_, _ = part.Write(content)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

// --- Streaming download -----------------------------------------------------

type downloadCmd struct {
	ID string
}

// seekableBytes wraps a byte slice as an io.ReadSeekCloser so the
// download handler can serve Range requests.
type seekableBytes struct{ *bytes.Reader }

func (seekableBytes) Close() error { return nil }

func TestNewStreamingDownloadHandlerFullBody(t *testing.T) {
	content := []byte("HELLO-WORLD-PAYLOAD")
	cmdFn := func(_ context.Context, cmd downloadCmd) (*rest.DownloadResponse, error) {
		assert.Equal(t, "D1", cmd.ID)
		return &rest.DownloadResponse{
			ContentType: "text/plain",
			Size:        int64(len(content)),
			Filename:    "hello.txt",
			Body:        seekableBytes{bytes.NewReader(content)},
		}, nil
	}
	decoder := func(req *http.Request) (downloadCmd, error) {
		return downloadCmd{ID: req.PathValue("id")}, nil
	}
	h := rest.NewStreamingDownloadHandler(cmdFn, decoder)

	mux := http.NewServeMux()
	mux.Handle("GET /documents/{id}", h)
	req := httptest.NewRequest(http.MethodGet, "/documents/D1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Equal(t, "bytes", rec.Header().Get("Accept-Ranges"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "hello.txt")
	assert.Equal(t, content, rec.Body.Bytes())
}

func TestNewStreamingDownloadHandlerRange(t *testing.T) {
	content := []byte("0123456789ABCDEF") // 16 bytes
	cmdFn := func(_ context.Context, _ downloadCmd) (*rest.DownloadResponse, error) {
		return &rest.DownloadResponse{
			ContentType: "application/octet-stream",
			Size:        int64(len(content)),
			Body:        seekableBytes{bytes.NewReader(content)},
		}, nil
	}
	decoder := func(_ *http.Request) (downloadCmd, error) { return downloadCmd{}, nil }
	h := rest.NewStreamingDownloadHandler(cmdFn, decoder)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Range", "bytes=2-5")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusPartialContent, rec.Code, rec.Body.String())
	assert.Equal(t, "bytes 2-5/16", rec.Header().Get("Content-Range"))
	assert.Equal(t, "4", rec.Header().Get("Content-Length"))
	assert.Equal(t, []byte("2345"), rec.Body.Bytes())
}

func TestNewStreamingDownloadHandlerInvalidRange(t *testing.T) {
	content := []byte("abc")
	cmdFn := func(_ context.Context, _ downloadCmd) (*rest.DownloadResponse, error) {
		return &rest.DownloadResponse{
			ContentType: "application/octet-stream",
			Size:        int64(len(content)),
			Body:        seekableBytes{bytes.NewReader(content)},
		}, nil
	}
	decoder := func(_ *http.Request) (downloadCmd, error) { return downloadCmd{}, nil }
	h := rest.NewStreamingDownloadHandler(cmdFn, decoder)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Range", "bytes=100-200") // beyond size
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestedRangeNotSatisfiable, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Range"), "*/3")
}

func TestNewStreamingDownloadHandlerNotFound(t *testing.T) {
	cmdFn := func(_ context.Context, _ downloadCmd) (*rest.DownloadResponse, error) {
		return nil, kiterrors.NotFound("doc", "nope")
	}
	decoder := func(_ *http.Request) (downloadCmd, error) { return downloadCmd{}, nil }
	h := rest.NewStreamingDownloadHandler(cmdFn, decoder)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// keep imports tidy
var _ = strings.HasPrefix
